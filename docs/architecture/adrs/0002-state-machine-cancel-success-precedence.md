# ADR-0002 — State machine and cancel/success commit-order precedence

- Status: **Proposed** (TASK-M1-BE-01). Review: TASK-M1-RV-01 (critical race).
- Requirements: FR-ENG-02, FR-ENG-03, FR-ENG-12, FR-ENG-13, FR-ENG-16; AC-03, AC-05, AC-06. Master §5.
- Normative table: `docs/architecture/state-machine.md`.

## Decision
1. Seven 1.0 states; three terminal (`succeeded`, `failed`, `canceled`). Terminal rows are immutable
   while retained, enforced by trigger `jobs_terminal_immutable`. No dead-letter state: the console's
   dead-letter view is `status='failed'`.
2. **Every** transition is one short READ COMMITTED transaction: a CAS
   `UPDATE jobs ... WHERE <allowed status> AND <fence> RETURNING version` with an affected-row check,
   the attempt update, and exactly one `job_events` insert with `version = new jobs.version`.
3. **Commit order decides cancel vs success.** Both statements target the same `jobs` row:
   - Cancel request (running): `SET cancel_requested_at = now() WHERE status='running' AND cancel_requested_at IS NULL`.
   - Success: `SET status='succeeded' WHERE status='running' AND cancel_requested_at IS NULL AND <fence>`.
   The second writer blocks on the row lock; after the first commits, PostgreSQL re-evaluates the
   second statement's WHERE against the new row version, which no longer matches ⇒ 0 rows.
   - **Cancel committed first ⇒ success denied.** The success call instead finalizes the job as
     `canceled` (attempt `canceled`) in the same transaction — the worker has stopped, so this is the
     cancellation acknowledgment. Reply reason `cancel_requested`.
   - **Success committed first ⇒ cancel is a no-op conflict**: REST 409 `job_terminal`
     (`details.status='succeeded'`). Success is never rolled back.
   - Cancel on `canceled` is an idempotent 200; on `failed` is 409.
4. Cancel of `scheduled`/`queued`/`retry_wait` finalizes `canceled` directly (200). Cancel of
   `running` returns 202 and the job finalizes when the worker acknowledges (ReportFailure/Success
   on a cancel-requested job), or when the lease — which renewals cap at
   `cancel_requested_at + cancel_grace` (15 s) — expires (E8 ⇒ `canceled`).
5. Retries: `max_attempts` 1..5 incl. first run; Claim is the only place `attempts_used` increments,
   guarded by CHECK `attempts_used <= max_attempts` + predicate `attempts_used < max_attempts`.
   Timeouts/crashes consume attempts. No automatic retry after budget exhaustion.
6. **Manual retry** (failed only) inserts a new job with `retried_from_job_id`, fresh budget, its own
   idempotency scope; parent untouched. Partial unique index allows one child per parent; the
   `Idempotency-Key` (namespace `manual-retry:<parent>`) makes the call replay-safe; audited.
7. Late/duplicate worker reports: same-attempt duplicate of the committed outcome ⇒ idempotent OK
   (`already_final`); anything else ⇒ denied + bounded diagnostic audit row; no job/event change.

## Consequences
- A handler may have produced side effects even when its success is denied (at-least-once; §16).
- READ COMMITTED re-check semantics are load-bearing; tests `TestAC06*` must stay in CI and the
  isolation level must not be raised to REPEATABLE READ without retry handling (serialization errors).
