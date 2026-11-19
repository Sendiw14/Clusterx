# ADR-0003 — Leases, heartbeat vs renewal, fencing, DB clock and timing bounds

- Status: **Proposed** (TASK-M1-BE-01). Review: TASK-M1-RV-01.
- Requirements: FR-ENG-07, FR-ENG-10, FR-ENG-14, FR-ENG-15, FR-ENG-16, FR-ENG-19; AC-03, AC-04, AC-09, AC-11. Master §4, §5.

## Decision

### Clock
The **DB clock `now()`** is the only clock for due time, lease expiry, deadline, credential expiry and
idempotency expiry. Go never compares its wall clock to stored timestamps for authorization; Go only
computes *durations* (backoff, lease TTL) that SQL adds to `now()`. Worker clocks are never trusted;
the worker receives `lease_expires_at` and a relative `lease_ttl` and must use the relative value.

### Two distinct signals
| | Heartbeat | Lease renewal |
|---|---|---|
| RPC | `Heartbeat` | `RenewLease` |
| Scope | worker process (`worker_id`, `incarnation_id`) | one attempt (`attempt_id`, `lease_generation`) |
| Writes | `workers.last_heartbeat_at` | `jobs.lease_expires_at` (no event, no version bump) |
| Purpose | liveness for UI, unschedulable reasons, drain | authority to keep executing / report |
| Extends leases? | **never** | yes, capped by hard deadline and cancel grace |

### Lease
- Claim sets `lease_generation = lease_generation + 1` (fencing token; equals attempt number in 1.0
  because each claim is one attempt), `lease_expires_at = now() + lease_ttl`,
  `hard_deadline_at = now() + timeout_seconds`.
- Renewal: `lease_expires_at = LEAST(now()+lease_ttl, hard_deadline_at, cancel_requested_at + cancel_grace)`.
- Current-lease uniqueness: partial unique index `job_attempts(job_id) WHERE state='active'` plus CHECK
  `running ⇔ current_attempt_id/lease_expires_at/hard_deadline_at NOT NULL`.

### Fence (applies to RenewLease, ReportSuccess, ReportFailure, PollCancellation)
attempt id + lease generation match `jobs` · job `running` · `lease_expires_at > clock_timestamp()` ·
`hard_deadline_at > clock_timestamp()` · attempt `active` and owned by (`worker_id`, `incarnation_id`) ·
`workers.current_incarnation_id = incarnation_id`, worker not deregistered · credential not revoked/expired.

**Locking and time (amended TASK-M1-BE-04, RV01-04; CR-M4-01 NF-01).** Global order for every transaction:
`worker_credentials` → `workers` → `jobs` → `job_attempts` (ascending id within a class); a share-locked
identity row is never written in the same transaction; 40P01/40001 are retried (≤ 3 runs) then `unavailable`.
Each worker mutation transaction first locks the `worker_credentials` then `workers` row `FOR SHARE`, then
the `jobs` row `FOR UPDATE`, then runs the CAS. The fence also requires `workers.credential_id` = the
credential presented on the call (M4). The
fence reads `clock_timestamp()`, i.e. after any lock wait.
- **Revocation bound (exact):** a revocation, re-registration or deregistration is a non-key UPDATE on the
  worker or credential row and conflicts with the share lock. Either it committed before the mutation took
  the share lock, and the fence sees it (denied), or it waits until the mutation commits. A mutation that
  commits *after* a revocation commit is therefore always denied. Revocation is linearized with reports;
  the only window is a mutation already holding the share lock, which is bounded by that transaction's
  `lock_timeout` + `statement_timeout`. M4 `internal/store` sets `SET LOCAL lock_timeout = '1s'` on worker-mutation
  transactions (`statement_timeout` 5 s, ADR-0001); the PoC does not.
- **Deadline bound (exact):** lease and deadline are compared with the clock after the job row lock is
  held. A report that waited for the lock is judged at the time it got the lock, not at transaction start.
  Locking first is required: PostgreSQL does not re-check a waiter's WHERE when the lock holder committed
  without modifying the row (observed in `TestRV0104_DeadlineJudgedAtRecheckNotTxStart`; mutants
  M-joblock and M-clock are killed by it).
- Cost: the share lock briefly blocks heartbeats (`UPDATE workers`) of the same worker during a report
  (single-row, milliseconds).
- `lease_expires_at <= hard_deadline_at` is a CHECK. The deadline clause in the fence is defense in depth
  (mutant M4 is equivalent unless that CHECK is also removed).
- Credential lookups are not cached for mutations.

### Recovery
Scheduler selects `running` rows with `lease_expires_at <= now() OR hard_deadline_at <= now()` (its own
short transaction's `now()`; a report may see expiry a few ms before recovery does and is then denied,
recovery finalizes on the next tick, covered by `TestAC03_StaleReportVsRecoveryConcurrent`)
(`FOR UPDATE SKIP LOCKED`), marks the attempt `lease_expired`/`deadline_exceeded`, then
`canceled` (if cancel requested) / `retry_wait` (budget left) / `failed` (`attempts_exhausted`).
No job remains `running` longer than `min(lease_ttl after last renewal, hard deadline) + one scheduler tick`.

### Drain / deregister / re-register
Drain: `status='draining'`, `drain_deadline_at = now()+drain_grace`; no new claims; renewals allowed
until the drain deadline, then denied (`worker_draining`) so leases expire and normal recovery
(retry/fail, **not** cancel) applies. Re-register with a new incarnation fences all attempts of the
old incarnation immediately (renew/report denied `incarnation_mismatch`); they are recovered at lease expiry.

**Drain is sticky to the worker id (TASK-M5-BE-02, RV5-02, integrator decision).** Re-registering a
`draining` worker id (operator or self drain) keeps `status='draining'` and its drain deadline: the new
incarnation gets no claims (`Claim` → `worker_draining`) until it deregisters. A `deregistered` worker
id is never resurrected: registering with it goes through the same selection as a registration without
`worker_id` (RV5R-01): a matching incarnation is reused, a **live** non-deregistered worker of the same
credential ⇒ `incarnation_mismatch`, a stale one is taken over, otherwise a **new** worker row (new
`worker_id`, active) is created and the SDK persists it via `OnRegistered`. So one credential never has
more than one live worker row. Integrator ratification (TASK-M5-BE-03): the server-assigned new row is
accepted (supersedes "delete the id file"), with two operator-visible consequences:
1. **Drain under a supervisor acts like a rolling restart.** A drained worker deregisters and exits; a
   supervisor (`restart: always`, Kubernetes) restarts it and it joins as a new active row. To keep a
   host out, **revoke its worker credential** (drain alone does not). Console drain UI and docs must say so.
2. **Every graceful restart creates a new `workers` row** (SIGTERM ⇒ self-drain ⇒ deregister ⇒ restart).
   The persisted worker id only saves the liveness wait after a crash (CR-M5-01 #4); deregistered rows
   accumulate and are visible as history. **One persisted worker id per process:** if two live processes share one id
(shared volume, `CLUSTERX_WORKER_ID` in a scaled service), the newest registration supersedes and the
older incarnation is fenced (`incarnation_mismatch`, its attempts recovered at lease expiry); every
takeover of a still-live incarnation appends a `security` audit row `worker.incarnation_takeover`
(previous and new incarnation ids), charged to the per-credential audit budget of ADR-0004 (RV5R-02). Tests: `TestRV5_OperatorDrainSurvivesReRegister`,
`TestRV5_ConcurrentIncarnationsNewestWins`, `TestRV5R_DeregisteredIdBypassesSingleLiveBinding`,
`TestRV5R_TakeoverAuditBudgeted`.

### Timing values (starting configuration, bounds validated at startup — fail closed)
| Setting | Default | Allowed | Constraint |
|---|---|---|---|
| heartbeat_interval | 10 s | 2–60 s | |
| liveness_timeout | 30 s | ≥ 3 × heartbeat | |
| lease_ttl | 30 s | 5–300 s | ≥ 3 × renew RPC deadline (5 s) |
| renew_at | lease_ttl / 2 | ≤ lease_ttl / 2 | SDK renews early |
| scheduler_tick | 1 s ± 20 % | 0.2–10 s | |
| attempt timeout (per job) | 600 s | 1–86 400 s | ≥ lease_ttl not required (deadline caps lease) |
| cancel_grace | 15 s | 1–120 s | ≤ lease_ttl recommended |
| drain_grace | 60 s | 0–3600 s | |
| backoff base / cap | 2 s / 300 s | base 0.1–60 s, cap ≤ 3600 s | full jitter [0.5,1.0) |
These are proposals, not verified defaults (§5); freeze after pilot (OD-06). PoC tests use
sub-second leases.

## Consequences / limits (disclosed)
- Fencing protects authoritative DB writes only; a fenced handler may still be running and causing
  external side effects. Hard deadline limits **authorization**, it does not kill goroutines.
- Two attempts of the same job can overlap externally between lease expiry and the old worker
  noticing; handlers must be idempotent.
