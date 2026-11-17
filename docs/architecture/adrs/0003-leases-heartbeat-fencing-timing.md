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
attempt id + lease generation match `jobs` · job `running` · `lease_expires_at > now()` ·
`hard_deadline_at > now()` · attempt `active` and owned by (`worker_id`, `incarnation_id`) ·
`workers.current_incarnation_id = incarnation_id`, worker not deregistered · credential not revoked/expired.
Evaluated **inside the CAS statement**, not by a prior SELECT. Revocation committed before the
statement starts is always observed (READ COMMITTED statement snapshot); **revocation bound = 0
for new mutations**; an in-flight statement that began before the revoke commit may still succeed.
Credential lookups are not cached for mutations.

### Recovery
Scheduler selects `running` rows with `lease_expires_at <= now() OR hard_deadline_at <= now()`
(`FOR UPDATE SKIP LOCKED`), marks the attempt `lease_expired`/`deadline_exceeded`, then
`canceled` (if cancel requested) / `retry_wait` (budget left) / `failed` (`attempts_exhausted`).
No job remains `running` longer than `min(lease_ttl after last renewal, hard deadline) + one scheduler tick`.

### Drain / deregister / re-register
Drain: `status='draining'`, `drain_deadline_at = now()+drain_grace`; no new claims; renewals allowed
until the drain deadline, then denied (`worker_draining`) so leases expire and normal recovery
(retry/fail, **not** cancel) applies. Re-register with a new incarnation fences all attempts of the
old incarnation immediately (renew/report denied `incarnation_mismatch`); they are recovered at lease expiry.

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
