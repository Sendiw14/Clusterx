# CLUSTERX — Job state machine (normative, M1 draft)

Status: **specified** (TASK-M1-BE-01). Source: MASTER_PROMPT R2 §5; ADR-0002, ADR-0003.
Requirement IDs: FR-ENG-02, FR-ENG-03, FR-ENG-04, FR-ENG-10..17; AC-02..AC-07, AC-09.

This table is the single source for transition code and tests. M2 generates a Go table from it
(`apps/control-plane/internal/state`), and a table-driven test asserts every (event × state) cell.
Any change here is a contract change (§1.3).

## States

| State | Terminal | Meaning |
|---|---|---|
| `scheduled` | no | Admitted with `run_after > now()`; waits for due promotion |
| `queued` | no | Eligible to claim (`available_at <= now()`) |
| `running` | no | Exactly one `active` attempt holds the current lease (`current_attempt_id`, `lease_generation`, `lease_expires_at`, `hard_deadline_at`) |
| `retry_wait` | no | Previous attempt failed retryably / expired; waits for `available_at` (backoff) |
| `succeeded` | **yes** | Result committed; immutable while retained |
| `failed` | **yes** | Permanent failure or attempt budget exhausted; immutable. "Dead-letter" = filtered view of `failed` |
| `canceled` | **yes** | Canceled before or during execution; immutable |

`blocked`, `skipped` are reserved for 1.1 and rejected by the `status` CHECK in 1.0.

Orthogonal fields (not states): `cancel_requested_at`, `available_at`, `current_attempt_id`,
`lease_generation`, `lease_expires_at`, `hard_deadline_at`, `attempts_used`, `version`.
**Unschedulable is a derived read-time reason, never a state** (see bottom).

## Global rules

1. Every row in the table below is **one short PostgreSQL transaction** (READ COMMITTED) that changes
   `jobs`, `job_attempts` and appends exactly one `job_events` row with `version = jobs.version` (new).
   Denials write no job/attempt/event row; worker-side denials append one bounded `audit_events`
   row (`category='diagnostic'` or `'security'`).
2. Every allowed transition is a CAS: `UPDATE jobs ... WHERE id=$1 AND status IN (allowed) AND <fence> RETURNING version`.
   Zero rows ⇒ the transition is denied; the caller re-reads the row only to pick the error code.
3. Terminal rows are never updated (DB trigger `jobs_terminal_immutable`). Finished attempts are never updated.
4. All time comparisons use the DB clock `now()`.

### Fence predicate `F(attempt)` (ADR-0003)

A worker mutation for (`job_id`, `attempt_id`, `lease_generation`, `worker_id`, `incarnation_id`) is authorized iff, in the same statement:

- `jobs.status = 'running'` and `jobs.current_attempt_id = attempt_id` and `jobs.lease_generation = lease_generation`
- `jobs.lease_expires_at > now()` and `jobs.hard_deadline_at > now()`
- `job_attempts.state = 'active'`, `job_attempts.worker_id = worker_id`, `job_attempts.worker_incarnation_id = incarnation_id`
- `workers.current_incarnation_id = incarnation_id` and `workers.status <> 'deregistered'`
- the worker's `worker_credentials` row has `revoked_at IS NULL` and (`expires_at IS NULL OR expires_at > now()`)

The first failing clause (checked in the order above after a 0-row CAS) selects the denial code.

## Event × state table

Legend: **→ X** allowed transition to X · **= ** allowed no-op (idempotent, no event) · **✗ code** denied with code.
REST codes are `error.code` values (HTTP status in brackets); worker codes are the gRPC `ErrorInfo.reason`
(gRPC status in brackets). "—" means the event cannot be addressed to that state by construction.

| Event (actor) | scheduled | queued | running | retry_wait | succeeded | failed | canceled |
|---|---|---|---|---|---|---|---|
| E1 Admit (operator/token) | creates `scheduled` if `run_after > now()` | creates `queued` otherwise | — | — | — | — | — |
| E2 Due promotion (scheduler) | → `queued` if `available_at <= now()`, else no-op | = | = | → `queued` if `available_at <= now()`, else no-op | = | = | = |
| E3 Claim (worker) | ✗ not eligible (not returned) | → `running` (+attempt, `lease_generation+1`, `attempts_used+1`) if F-worker\* ok and capability match and `attempts_used < max_attempts` | ✗ not eligible (skipped) | ✗ not eligible | ✗ | ✗ | ✗ |
| E4 RenewLease (worker) | ✗ `stale_attempt` [FAILED_PRECONDITION] | ✗ `stale_attempt` | extend lease if F ok: `lease_expires_at = LEAST(now()+lease, hard_deadline_at, cancel_requested_at+cancel_grace)`; no event, no version bump. ✗ per F order: `stale_attempt`, `lease_expired`, `deadline_exceeded`, `incarnation_mismatch`, `worker_deregistered`, `credential_revoked` | ✗ `stale_attempt` | ✗ `job_terminal` | ✗ `job_terminal` | ✗ `job_terminal` |
| E5 ReportSuccess (worker) | ✗ `stale_attempt` | ✗ `stale_attempt` | if F ok and `cancel_requested_at IS NULL` → `succeeded`; if F ok and cancel requested → `canceled` (success denied, reply `cancel_requested`, see ADR-0002); else ✗ F code | ✗ `stale_attempt` | duplicate from same attempt: = (reply `ALREADY_FINAL`, idempotent); other: ✗ `job_terminal` | ✗ `job_terminal` | ✗ `job_terminal` |
| E6 ReportFailure retryable (worker) | ✗ `stale_attempt` | ✗ `stale_attempt` | if F ok: cancel requested → `canceled`; else `attempts_used < max_attempts` → `retry_wait` (`available_at = now()+backoff`); else → `failed` (`attempts_exhausted`) | ✗ `stale_attempt` | ✗ `job_terminal` | duplicate same attempt: =; else ✗ `job_terminal` | ✗ `job_terminal` |
| E7 ReportFailure permanent (worker) | ✗ `stale_attempt` | ✗ `stale_attempt` | if F ok: cancel requested → `canceled`; else → `failed` | ✗ `stale_attempt` | ✗ `job_terminal` | duplicate: =; else ✗ `job_terminal` | ✗ `job_terminal` |
| E8 Lease expiry / deadline (scheduler) | — | — | if `lease_expires_at <= now()` or `hard_deadline_at <= now()`: attempt → `lease_expired`/`deadline_exceeded`; job → `canceled` if cancel requested, else `retry_wait` if `attempts_used < max_attempts`, else `failed` | — | — | — | — |
| E9 Cancel (operator) | → `canceled` [200] | → `canceled` [200] | sets `cancel_requested_at` and caps `lease_expires_at` at now()+cancel_grace [202]; if already set = [202] | → `canceled` [200] | ✗ `job_terminal` [409] | ✗ `job_terminal` [409] | = [200] (already canceled) |
| E10 PollCancellation (worker) | ✗ `stale_attempt` | ✗ `stale_attempt` | returns `cancel_requested` bool + `lease_expires_at` if F ok; else ✗ F code | ✗ `stale_attempt` | returns `job_terminal` | returns `job_terminal` | returns `cancel_requested=true, job_terminal` |
| E11 Manual retry (operator) | ✗ `retry_not_allowed` [409] | ✗ `retry_not_allowed` [409] | ✗ `retry_not_allowed` [409] | ✗ `retry_not_allowed` [409] | ✗ `retry_not_allowed` [409] | creates **new** job (`queued`, `retried_from_job_id`, fresh attempt budget) [201]; old row untouched; same Idempotency-Key = [200]; child already exists with different/no key ✗ `retry_already_exists` [409] | ✗ `retry_not_allowed` [409] |
| E12 Worker drain deadline passed (scheduler) | — | — | renewals for that worker are denied with `worker_draining`; lease then expires ⇒ E8 (not cancel) | — | — | — | — |

\* F-worker for Claim: worker row `status='active'`, `current_incarnation_id` matches, credential valid,
active attempts of that worker `< capacity`. Denials: `incarnation_mismatch` [FAILED_PRECONDITION],
`worker_draining` [FAILED_PRECONDITION], `worker_deregistered` [FAILED_PRECONDITION],
`credential_revoked` [UNAUTHENTICATED], `capacity_exhausted` (empty claim, OK), unknown handler in
`supported` list ⇒ `unknown_handler` [INVALID_ARGUMENT]. A claim that finds no eligible row returns an empty list (OK), not an error.

Cancel-vs-success order (ADR-0002): whichever transaction commits first wins; the other's CAS
re-evaluates against the committed row (PostgreSQL READ COMMITTED re-check of the locked row) and
matches 0 rows. Proven by `apps/control-plane/poc` `TestAC06*`.

## Admission denials (E1)

| Condition | code | HTTP |
|---|---|---|
| Missing/invalid credentials | `unauthenticated` | 401 |
| Viewer or token without `jobs:write` | `forbidden` | 403 |
| Body > 80 KiB or payload > 64 KiB | `payload_too_large` | 413 |
| Malformed JSON | `bad_request` | 400 |
| Unknown handler/version, schema mismatch, priority ∉ 0..9, max_attempts ∉ 1..5, timeout ∉ 1..86400 s, `run_after` > now()+30 d | `validation_failed` (details.fields[]) | 422 |
| Same idempotency scope+key, different fingerprint, unexpired | `idempotency_conflict` | 409 |
| Same scope+key+fingerprint, unexpired | (no error) returns original job | 200 (first creation 201) |
| Rate limit | `rate_limited` + `Retry-After` | 429 |
| DB unavailable / readiness failing | `unavailable` + `Retry-After` | 503 (never 2xx before commit) |

## Worker-side denial codes (gRPC `google.rpc.ErrorInfo.reason`, domain `clusterx.dev`)

| reason | gRPC status | Meaning / worker action |
|---|---|---|
| `stale_attempt` | FAILED_PRECONDITION | attempt/generation is not current; stop, discard result |
| `lease_expired` | FAILED_PRECONDITION | lease ran out; stop, result discarded |
| `deadline_exceeded` | FAILED_PRECONDITION | hard deadline passed; stop |
| `cancel_requested` | FAILED_PRECONDITION (on success) / OK flag (on renew/poll) | stop cooperatively; job finalized `canceled` |
| `job_terminal` | FAILED_PRECONDITION | job already final; discard |
| `incarnation_mismatch` | FAILED_PRECONDITION | process superseded; re-register |
| `worker_draining` | FAILED_PRECONDITION | no new claims; renewals denied after drain deadline |
| `worker_deregistered` | FAILED_PRECONDITION | re-register |
| `credential_revoked` | UNAUTHENTICATED | credential revoked/expired; exit |
| `unknown_handler` | INVALID_ARGUMENT | not in server registry |
| `result_too_large` | INVALID_ARGUMENT | result > 128 KiB; report a failure instead |
| `protocol_unsupported` | FAILED_PRECONDITION | upgrade worker |
| `rate_limited` | RESOURCE_EXHAUSTED | back off |
| `unavailable` | UNAVAILABLE | DB down; claims paused; never assume lease still valid |

Every worker-side denial on a mutation (E4–E7) appends one `audit_events` row (`diagnostic` for
stale/late, `security` for revoked/mismatch), rate-limited per worker (ADR-0004).

## Retry / attempt budget

- `max_attempts` ∈ 1..5 (default 3) **includes** the first run. `attempts_used` is incremented only by Claim.
- DB CHECK `attempts_used <= max_attempts` and Claim predicate `attempts_used < max_attempts` ⇒ no hidden extra attempt.
- Timeout, crash and lease expiry consume the attempt (it was claimed). With budget exhausted ⇒ `failed`, `error_code='attempts_exhausted'`.
- Backoff: `delay = min(cap, base·2^(n-1))` with full jitter `rand[0.5,1.0)·delay`; base 2 s, cap 5 min; computed in Go with injected clock/rand, applied as `now() + delay` in SQL.

## `failure_class` (CCR-M1 OQ-08)

Set in the same transaction as the terminal transition. DB CHECKs: non-null **exactly** on `failed`/`canceled`.

| Terminal transition | `failure_class` |
|---|---|
| E6 retryable failure with budget exhausted | `retryable_exhausted` |
| E7 permanent failure | `permanent` |
| E8 hard deadline reached, budget exhausted | `timeout` |
| E8 lease expired (crash/partition/drain), budget exhausted | `lease_expired` |
| any → `canceled` (E5/E6/E7 ack, E8 with cancel requested, E9) | `canceled_by_operator` |
| → `succeeded` | `null` |

`error_code` carries the handler/system code (`attempts_exhausted` for E8 exhaustion); `failure_class` is the
UI-facing category. The PoC asserts `failed/lease_expired/3` after three expired attempts (`TestBudgetExhaustedNoHiddenAttempt`).

## Conflict and expiry responses

- Every 409 (`job_terminal`, `retry_not_allowed`, `retry_already_exists`, `worker_deregistered`) carries
  `details: {status, version}` of the current row. Every mutation 2xx returns the post-state resource.
- Retention deletion writes `job_tombstones(job_id, terminal_at, expired_at)` in the same transaction as the
  DELETE. Any job-addressed operation on a tombstoned ID answers **410 `expired`**; unknown IDs answer 404.
- Workers carry `version` (bumped on status/incarnation change, not on heartbeat) for hint reconciliation.

## Events (`job_events.event_type`)

`admitted, promoted, claimed, succeeded, failed_retryable, failed_permanent, attempts_exhausted,
lease_expired, deadline_exceeded, cancel_requested, canceled, retry_created` (`retry_created` is
appended to the **child** job; the terminal parent is not modified).

## Unschedulable (derived, not a state)

Computed on read for `queued` jobs; returned as `unschedulable: {reason, since}` or `null`:

| reason | rule |
|---|---|
| `no_capable_worker` | no non-deregistered worker advertises (`handler_name`, `handler_version`) |
| `capable_workers_offline` | capable workers exist but none heartbeat within liveness timeout |
| `capable_workers_draining` | all live capable workers are `draining` |

`since` = `max(available_at, last time a capable live worker was seen)` (best effort). Saturation
(all capable workers at capacity) is **not** unschedulable.
