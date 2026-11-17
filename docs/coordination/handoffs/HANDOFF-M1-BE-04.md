# HANDOFF — TASK-M1-BE-04 (RV-01 fixes + FE-02 gaps, M1 exit)

- **Author, model/effort:** backend lane subagent, model `claude-opus-5-5`. Requested effort: medium. Effective effort and Ultracode: UNVERIFIED (MW-04).
- **Branch:** `m1/backend` (worktree `.worktrees/backend`). Not pushed, not merged.
- **Inputs:**
  - `main:docs/coordination/tasks/TASK-M1-BE-04.md`
  - `main:docs/coordination/reviews/REVIEW-M1-RV-01.md` (snapshot `ee1fa3d`)
  - `0e06a76:docs/design/consumer-requirements.md` §11.8 and §11.4
  - reviewer probes `<scratchpad>/rv01/cp/poc/review_test.go` (read only; adapted, not copied verbatim)
- **Migration choice:** I edited `apps/control-plane/migrations/0001_init.sql` **in place** rather than adding 0002, because nothing is deployed. This is stated in the file header.
- **Status of every reviewer item: "fixed, pending re-review" or dispositioned below. Nothing is marked closed.**

## Commits
| SHA | Content |
|---|---|
| `b91fb60` | DDL. Guard triggers: attempt insert must be the job's current lease with number = budget; admission fields immutable; budget advances by one only on claim. Also: lineage repoint blocked, history and young-audit delete guards, `job_attempts.result_sha256`, `api_token_authority` view, `sessions.last_activity_at` + `idle_timeout_seconds` |
| `a79b148` | PoC changes:<br>- Claim updates the job first, then inserts the attempt.<br>- Reports lock worker + credential `FOR SHARE`, then the job `FOR UPDATE`, then run the CAS with `clock_timestamp()`.<br>- Canonical reply `Reply{Outcome, AlreadyFinal}`; duplicate checks identity + result hash; discarded-success marker.<br>- New `poc/fence_test.go`. |
| `9adfc5b` | The deadline fence case also tries to keep the lease past the deadline (M4 demonstrability) |
| `19893d0` | state-machine.md (E5 canonical reply, lock order, clock, denial order, flat unschedulable fields) and worker.proto reply comments |
| `75a1eb8` | ADR-0002, ADR-0003, ADR-0005, ADR-0006 and trust-boundaries for RV01-02/03/04/08/11/12/13, SESS-01, BFF-01, BFF-02 |
| `757c127` | OpenAPI `1.0.0-draft.4`, api-contracts.md, permissions.md, DOC-01 text (incl. the BE-01 handoff naming note) |
| `296e856` | ADR-0004 retention guards |
| (this) | this handoff |

## Commands, exit codes, results (final HEAD `296e856` code)
| Check | Command | Exit | Result |
|---|---|---|---|
| Cluster | `LC_ALL=en_US.UTF-8 pg_ctl -D <scratchpad>/pg18data -o "-p 55432 -k '' -c listen_addresses=localhost" -w start` | 0 | PG 18.7, TCP localhost only |
| DDL (fresh DB) | `dropdb --if-exists ddlcheck; createdb ddlcheck; psql -h localhost -p 55432 -U clusterx -d ddlcheck -v ON_ERROR_STOP=1 --single-transaction -q -f apps/control-plane/migrations/0001_init.sql` | 0 | **PASS** |
| Vet | `go vet ./...` | 0 | **PASS** |
| Race ×3 | `CLUSTERX_PG_ADMIN_DSN='postgres://clusterx@localhost:55432/postgres?sslmode=disable' go test -race -count=3 -v ./poc/...` | 0 | **PASS**: 57 `--- PASS` (19 tests × 3), 0 FAIL, 0 SKIP; `ok …/poc 34.508s` |
| Proto | `protoc -I . -I /opt/anaconda3/include --descriptor_set_out=/dev/null clusterx/worker/v1/worker.proto` | 0 | **PASS** |
| OpenAPI parse | PyYAML: `version 1.0.0-draft.4 paths: 25 ops: 28 refs: 52 unresolved: [] api ops without 500: []`; stray-key scan (keys with spaces / nulls outside examples) → `0 []` | 0 | **PASS** |
| Lint | `npx --yes @redocly/cli@2.79.0 lint api/openapi.yaml` | 0 | `Woohoo! Your API description is valid. 🎉` / `You have 5 warnings.` |
| Cluster stop | `pg_ctl -D … -m fast stop`; `lsof -iTCP:55432 -sTCP:LISTEN` → 0 | 0 | stopped |

The 5 lint warnings are the same expected set as draft.3:
- `info-license` (OD-02)
- `no-server-example.com` (localhost)
- `operation-2xx-response` on `/stream` (it returns 101)
- 2× `operation-4xx-response` on `/health/*`

The first lint of draft.4 exited 1 with 3 struct errors (unquoted comma/colon inside flow descriptions) and 4 warnings: 3 examples were missing the new required `as_of`, and `ValidationDetails` was unused. All are fixed.

Run logs from the race ×3 tally lines:
- **Boundary race** (stale report vs recovery, 30 iterations per run):

  | Run | denied-then-recovered | recovered | success |
  |---|---|---|---|
  | 1 | 5 | 14 | 11 |
  | 2 | 4 | 14 | 12 |
  | 3 | 2 | 14 | 14 |

- **Random cancel-vs-success race:** cancel-first / success-first = 98/2, 99/1 and 100/0.
- The random race is skewed because cancel now locks the row before the report does. Both orderings are therefore **forced** in `TestAC06_ForcedOrderings` (RV01-09) rather than relying on chance.

## Mutant table (each mutant applied alone, full suite `go test -count=1 ./poc/`, then restored with `git checkout`)
Script: `<scratchpad>/be04/mutants.py`. Output: `<scratchpad>/be04/mutants.txt`.

| Mutant | Change | Exit | Verdict | Killing test (first failure line) |
|---|---|---|---|---|
| M1 | drop credential revoked/expired clause from fence | 1 | KILLED | `TestAC09_ReportFenceDenials/M1_credential_revoked` and `/M1_credential_expired`: `report = {succeeded false}, <nil>; want denied credential_revoked` |
| M2 | drop `w.current_incarnation_id = $5` | 1 | KILLED | `…/M2_worker_re-registered_(old_process_reports)`: `want denied incarnation_mismatch` |
| M3 | drop `a.worker_incarnation_id = $5` | 1 | KILLED | `…/M3_attempt_held_by_another_incarnation`: `want denied incarnation_mismatch` |
| M4 | drop `j.hard_deadline_at > clock_timestamp()` | 0 | **SURVIVED: equivalent** | see disposition below |
| M4 + D-leaseCHECK | M4 and drop CHECK `lease_expires_at <= hard_deadline_at` | 1 | KILLED | `…/M4_hard_deadline_passed`: `report = {succeeded false}, <nil>; want denied deadline_exceeded`; also `TestSchemaGuards` |
| M5 | drop `w.status <> 'deregistered'` | 1 | KILLED | `…/M5_worker_deregistered`: `want denied worker_deregistered` |
| M11 | drop `case cancel:` in RecoverExpired | 1 | KILLED | `TestE8_CancelRequestedThenLeaseExpiry`: `recover = 0, ERROR: … violates check constraint "jobs_check9"` (the mutant tries `retry_wait` with a cancel request; the test fails, and the DB CHECK is the second layer) |
| M-lock | drop the worker/credential `FOR SHARE` | 1 | KILLED | `TestRV0104_RevocationSerializesWithInFlightReport`: `timed out waiting for 2 lock waiters` (the revocation no longer waits) |
| M-joblock | drop the job `FOR UPDATE` before the fence | 1 | KILLED | `TestRV0104_DeadlineJudgedAtRecheckNotTxStart`: `report after deadline = <nil>, want deadline_exceeded` |
| M-clock | `clock_timestamp()` → `now()` in fence | 1 | KILLED | same test, same message |
| M-dup-owner | drop the "not the caller's attempt" case | 1 | KILLED | `TestRV0107_…`: `other worker = {succeeded true}, <nil>; want stale_attempt` |
| M-dup-result | ignore the result hash on duplicates | 1 | KILLED | `TestRV0107_…`: `different result = {succeeded true}, <nil>; want result_mismatch` |
| M-reply | drop the idempotent canceled reply | 1 | KILLED | `TestAC06_CancelThenSuccess`: `retried success after cancel = { false}, denied: job_terminal; want OK/canceled/already_final` |
| M-marker | drop `worker_reported` event data | 1 | KILLED | `TestAC06_CancelThenSuccess` (canceled event data NULL) |
| D-lineage | old lineage probe (any rewrite allowed) | 1 | KILLED | `TestSchemaGuards`: `(d) lineage repointed on terminal job: accepted` |
| D-jobguard | drop `jobs_guard_update` trigger | 1 | KILLED | `TestSchemaGuards`: `(b) budget skipped: accepted` |
| D-attemptguard | drop `job_attempts_guard_insert` trigger | 1 | KILLED | `TestSchemaGuards`: `(c) active attempt on queued job: accepted` |
| D-historydelete | drop `job_events_no_delete` trigger | 1 | KILLED | `TestSchemaGuards`: `(e) events of retained job deleted: accepted` |
| D-auditdelete | drop `audit_events_no_young_delete` trigger | 1 | KILLED | `TestSchemaGuards`: `(g) young audit row deleted: accepted` |
| D-tokenview | view returns raw `t.scopes` | 1 | KILLED | `TestRV0102_…`: `viewer/jobs:write,jobs:read/ok, want viewer/jobs:read/ok` |
| D-leaseCHECK | drop the lease ≤ deadline CHECK | 1 | KILLED | `TestSchemaGuards`: `(h) lease beyond hard deadline: accepted` |

**M4 disposition (not fixed — dispositioned):**
- The DB CHECK `lease_expires_at <= hard_deadline_at`, together with claim and renewal capping the lease at the deadline, means no reachable state has a valid lease past the deadline. Dropping only the fence's deadline clause therefore cannot change behavior; it is an equivalent mutant, like the reviewer's M6b/M13.
- I kept the clause as defense in depth and showed it is the only remaining guard once the CHECK is gone: the M4 + D-leaseCHECK row is killed by the fence test.
- The CHECK itself is pinned by `TestSchemaGuards (h)`. ADR-0003 and state-machine.md record this.
- **Reviewer to decide if this disposition is acceptable.**

## Reviewer items
| ID | Status | What changed / evidence |
|---|---|---|
| RV01-01 | fixed, pending re-review | `TestAC09_ReportFenceDenials` has one case per clause and checks: denial code, unchanged job/attempt/event snapshot, one audit row. Also added: `TestAC09_ClaimDenials` (revoked, expired, superseded, draining, deregistered) and `TestE8_CancelRequestedThenLeaseExpiry`. Mutants M1/M2/M3/M5/M11 killed; M4 equivalent (above). |
| RV01-02 | fixed, pending re-review | Effective token authority = scopes ∩ owner's current role; a disabled owner gets 401 `user_disabled`. Implemented in view `api_token_authority` (DDL) and specified in ADR-0005 #3, permissions.md, trust-boundaries B3 and the OpenAPI `ApiTokenCreate.role` text. `TestRV0102_TokenAuthorityFollowsOwner` covers demote, disable, re-enable and revoke; mutant D-tokenview killed. Request handling is M5 (spec only). |
| RV01-03 | fixed, pending re-review | Canonical reply: OK + `OUTCOME_CANCELED`. A same-attempt retry gets OK + `OUTCOME_CANCELED` + `already_final`. Aligned across ADR-0002 §3/§7, state-machine E5 and the denial table, worker.proto and the PoC. The old test asserting `job_terminal` now asserts the canonical reply; mutant M-reply killed. |
| RV01-04 | fixed (preferred option), pending re-review | Worker/credential rows `FOR SHARE`, then job `FOR UPDATE`, then a CAS using `clock_timestamp()`. ADR-0003 states the exact bound. New finding while fixing: locking the job row first is **required**, because PostgreSQL does not re-check a waiter's WHERE when the locker committed without modifying the row. The first version (clock only) failed the deadline test. Tests: `TestRV0104_*`; mutants M-lock, M-joblock and M-clock killed. `lock_timeout = 1s` is specified for M2 but not set in the PoC. |
| RV01-05 | fixed, pending re-review | Attempt-insert guard (must be the current lease of a running job, number = `attempts_used` ≤ `max_attempts`, generation matches). Job-update guard (admission fields immutable; budget advances only +1 on queued→running). Claim reordered to update the job first (deferred FK). `TestSchemaGuards (a)(b)(c)`; mutants D-attemptguard and D-jobguard killed. |
| RV01-06 | fixed, pending re-review | Lineage may only be nulled, never repointed. Attempts and events of a retained job cannot be deleted; only the FK cascade from `DELETE jobs` can. Audit rows under 90 d cannot be deleted. Grants are documented in the migration header. `TestSchemaGuards (d)(e)(f)(g)` plus a legitimate retention delete with tombstone and lineage nulling; mutants D-lineage, D-historydelete and D-auditdelete killed. |
| RV01-07 | fixed, pending re-review | `result_sha256` on attempts. A duplicate gets OK only from the same attempt/worker/incarnation with the same hash; a different hash gets `result_mismatch` (security audit); another worker gets `stale_attempt`. `TestRV0107_…`; mutants M-dup-owner and M-dup-result killed. |
| RV01-08 | fixed, pending re-review | The `canceled` event `data` is `{"worker_reported":"success"}` (no content), and the attempt keeps `result_sha256`. Asserted in `TestAC06_CancelThenSuccess`; mutant M-marker killed. |
| RV01-09 | fixed, pending re-review | `TestAC06_ForcedOrderings` forces cancel-first and success-first with both waiters queued on the row lock (`pg_stat_activity` wait count) and asserts each. `TestAC03_StaleReportVsRecoveryConcurrent` is adapted from the reviewer probe. It found a valid third outcome: the report is denied by the clock just before recovery's `now()` reaches expiry, and the next tick recovers the job. Asserted. |
| RV01-10 | **dispositioned: deferred to M2** (as the reviewer recommended) | AC-07, manual retry, RenewLease cap, ReportFailure classes and drain have no executable evidence yet. M2 builds the table-driven event × state suite. |
| RV01-11 | fixed (spec), pending re-review | ADR-0006 §1 requires `sessions.user_id = sub`; a mismatch is `assertion_invalid`. §5 now states plainly that a BFF key holder can mint sessions for any allowlisted user (full impersonation), with mitigations; the false "bounds" claim is removed. OpenAPI Unauthenticated text updated. No code (M5/M6). |
| RV01-12 | fixed (spec, accepted residual), pending re-review | ADR-0006 §1 and trust-boundaries B2: restart replay window ≤ 65 s for mutations; reads and WS upgrades are never single-use. |
| RV01-13 | fixed (spec), pending re-review | OpenAPI `/stream` text, ADR-0006 §4 and permissions.md: token streams are re-validated every ≤ 30 s and close with 4401/4403. |
| RV01-14 | fixed (contract), pending re-review | `JobEvent.data` is now the closed `JobEventData` schema (`additionalProperties: false`, no payload/result/message/log fields); permissions.md notes events need no masking. |
| RV01-15 | **dispositioned: deferred to M4** (ticket instruction) | Claim query top-N before AC-19. |

## FE-02 gaps (draft.4)
| Gap | Done |
|---|---|
| SESS-01 | `sessions.last_activity_at` + `idle_timeout_seconds` (DDL); Go enforces idle (ADR-0006 §3); 401 `session_expired` carries `details.expiry` idle\|absolute; `Me.session.last_activity_at` and `idle_expires_at` |
| BFF-01 | Closed `/login?error=` and `?reason=` sets in ADR-0006 §3 (adopts the FE proposal plus the `user_disabled` refinement) |
| BFF-02 | Proxy envelope rule in ADR-0006 §3 and the OpenAPI info description |
| ERR-01/02 | `ValidationDetails`: `rule` enum, JSON-Pointer `field` (`?name` for query, `header:Name`), optional `limit` |
| SORT-01 | `listJobs`: `sort`, `available_after/before`, `failure_class`, `handler_version`. `listWorkers`: `sort` |
| CRED-01 | `Idempotency-Key` on both credential creates. A replay returns 200 without the secret plus `Idempotent-Replayed`. 409 on conflict. |
| WRK-01 | `Worker.credential_revoked_at` |
| TIME-01 | Required `as_of` on `Job` and `WorkerDetail` |
| OVR-01 | `MetricsOverview.terminal_counts_window {days, since}` |
| CFG-01 | `Config.environment`, `build{version, commit}`, `contract_version`, `limits.timeseries_window_max_days` |
| DOC-01 | Flat unschedulable fields in state-machine.md; hint shape in api-contracts.md; BE-01 handoff naming note; JobNotFound text vs 410; Config example draft.4; `Internal` 500 response on every `/api/v1` op |
| Deferred per ticket | RT-01..03, AUDIT-01/02 → M6; OBS-01/02, RET-01 → M8 (not implemented) |

## NOT RUN / limits
- No Go service exists yet. Token authority, sid/sub binding, idle enforcement, stream re-validation and the proxy envelope are specification only (M5/M6), except where a SQL test exists (`api_token_authority`).
- The PoC does not set `lock_timeout`; it is specified for M2.
- TS client generation was not run. No CI run; all evidence is from a dev laptop (PG 18.7, go1.27.4, darwin/arm64).
- The idle bump "at most once per 60 s" can extend a session by up to 60 s beyond the exact idle window (documented).

## Open questions
- **Reviewer:**
  - Is the M4 equivalent-mutant disposition acceptable?
  - Is the decision that hint-only streams do not count as session activity right?
- **Owner (unchanged):**
  - OQ-03: may viewers read `/audit`?
  - Are session lifetimes of idle 2 h and absolute 12 h acceptable?
- **Integrator:** freeze draft.4 at M1 exit after re-review.
