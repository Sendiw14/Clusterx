# ADR-0004 — IDs, idempotency, payload bounds, retention vs immutability

- Status: **Proposed** (TASK-M1-BE-01).
- Requirements: FR-ENG-01, FR-ENG-12, FR-ENG-17, FR-ENG-18, FR-ENG-20, FR-ENG-23, FR-ENG-24; AC-07, AC-12, AC-17. Master §5, §6. Risk R-14, A-02.

## IDs
UUIDv7 everywhere via PostgreSQL 18 built-in `uuidv7()` (time-sortable; A-02 confirmed). IDs are
opaque to clients; ordering for pagination uses `(created_at, id)` explicitly, never UUID bit layout.

## Idempotent submission
- Header `Idempotency-Key` (1–128 chars `[A-Za-z0-9_.:-]`) optional on `POST /jobs`.
- Scope: (`principal`, `namespace`, `key`) where principal = `user:<uuid>` or `token:<uuid>`.
- Fingerprint: SHA-256 over RFC 8785 (JCS) canonical JSON of {namespace, handler_name, handler_version,
  payload, priority, run_after (UTC RFC 3339 or null), max_attempts, timeout_seconds}.
- Replay signal: HTTP 200 + header `Idempotent-Replayed: true` (first create 201) — CCR-M1.
- Retention: 24 h (`expires_at`). Concurrency: inserted in the **same transaction** as the job with
  `INSERT ... ON CONFLICT (principal, namespace, idem_key) DO UPDATE ... WHERE expires_at <= now()`;
  on 0 rows the transaction rolls back and the existing row is read:
  same fingerprint ⇒ 200 + original job (header `Idempotent-Replayed: true`);
  different ⇒ 409 `idempotency_conflict` (details: `job_id` of the original).
  A concurrent identical submission blocks on the unique index until the first commits, then replays.
- Expired keys may be reused; cleanup deletes expired rows hourly.
- Manual retry uses the same table with namespace `manual-retry:<parent_job_id>`; plus partial unique
  index (one child per failed parent).

## Bounds (initial; validated before DB write and by CHECK)
| Item | Limit | Over-limit behavior |
|---|---|---|
| HTTP request body | 80 KiB | 413 `payload_too_large` before parsing |
| Job payload (JSON text) | 64 KiB | 413 |
| Result (JSON text) | 128 KiB | gRPC INVALID_ARGUMENT `result_too_large`; SDK converts to permanent failure `result_too_large`; never truncated |
| error_message | 2 KiB | truncated with marker `…[truncated]` (UTF-8 safe) |
| attempt log_tail | 16 KiB | keep last 16 KiB, marker prefix |
| event / audit `data` | 4 KiB | server-generated; bounded by construction |
| run_after | ≤ now()+30 d | 422 |
| JSON nesting depth | 32 | 422 |
| Page size | 1–200 (default 50) | 422 |
| gRPC message | 256 KiB receive / send | RESOURCE_EXHAUSTED |
Payload must be valid UTF-8 JSON; NUL characters rejected. Payloads/results are **not** for secrets.

## Events
`job_events(job_id, version)` primary key; `version` = `jobs.version` after the change, so a client
snapshot `{job, version}` plus events `> version` reconcile exactly. Append-only trigger.

## Retention vs immutability
Terminal immutability forbids UPDATE, not DELETE. Retention is a separate lifecycle operation:
| Data | Retention (proposed) | Mechanism |
|---|---|---|
| attempt `log_tail` | 7 d after attempt finish | **exception** (CR-M4-01 NF-03, migration 0002): the finished-attempt trigger itself allows exactly `log_tail <non-null> → NULL` with `log_expired false → true` on an attempt finished ≥ 7 d ago, every other column unchanged; entry point `clusterx_expire_attempt_logs(n)` (role `clusterx_retention`). The only post-final mutation |
| terminal jobs + attempts + events + idempotency rows | 30 d after `terminal_at` | `DELETE FROM jobs` batch, FK CASCADE; child `retried_from_job_id` → NULL |
| non-terminal jobs | never auto-deleted | |
| audit_events | 90 d | batch delete |
| idempotency | 24 h | batch delete |
| sessions | expiry + 7 d | batch delete |
DB guards (TASK-M1-BE-04, RV01-06):
- Attempts and events of a job that still exists cannot be deleted. Only the FK cascade from `DELETE jobs`
  can remove them.
- Audit rows younger than 90 d cannot be deleted.
- Lineage on a terminal job can only be nulled by the FK action, never repointed.

Retention writes `job_tombstones(job_id, terminal_at, expired_at)` in the same transaction as the DELETE;
tombstoned IDs answer **410 `expired`** (CCR-M1), unknown IDs 404. Tombstones are kept 90 d, after which
the ID answers 404. Figures are not legal advice (§6). Backups: 14 d.
The log-retention function and the guards `jobs_retention_delete_only` (DELETE jobs only when terminal ≥ 30 d) and
BEFORE TRUNCATE triggers on history tables are in migration 0002 (CR-M4-01 NF-03/NF-04). Runtime role
`clusterx_runtime`: SELECT/INSERT/UPDATE (since 0004: SELECT only on job_tombstones, no UPDATE on job_events/audit_events), DELETE only on sessions, submission_idempotency, worker_capabilities;
no TRUNCATE/DDL. `clusterx_retention`: the deletes above. Login users are granted one role out of band.

## Migrations
Forward-only, numbered `NNNN_name.sql`, each applied in one transaction, expand → backfill → contract
across releases; no destructive change in the same release that stops using a column.

## Implementation notes (TASK-M5-BE-01)
- `clusterx-control-plane retention run` (one bounded pass: ≤ 20 × 1000 rows per category) and the optional in-process loop
  (`CLUSTERX_RETENTION_INTERVAL`, ≥ 1 m) connect with `CLUSTERX_RETENTION_DATABASE_URL`, a login role in `clusterx_retention`.
  The runtime role cannot run it (42501). Tombstones are inserted in the same statement as the job DELETE.
- Credential idempotency (`POST /tokens`, `POST /worker-credentials`): `submission_idempotency.job_id` references `jobs`, so the
  key and request fingerprint are stored in the create audit row (`details.idempotency_key`, `details.fingerprint`, 24 h window),
  serialized by `pg_advisory_xact_lock` on (actor, action, key). No schema change.
  **Contract interpretation (accepted in REVIEW-M5-RV-01 RV5-08, recorded by TASK-M5-BE-02):** the audit row is the idempotency
  record for credential creation. The key is scoped to (actor, action): another principal reusing it gets a new credential
  (test `TestM5_TokensAndWorkerCredentials`). This requires `audit_events` retention to stay **≥ 24 h** (it is 90 d); a
  shorter audit retention would silently shorten the replay window. Lookups use the partial index
  `audit_events_idempotency_idx (action, actor, (details->>'idempotency_key'), occurred_at) WHERE details ? 'idempotency_key'`
  (migration 0004).
- Worker denial audit budget (RV5-11): `Store.DenialAudit`, an in-process token bucket per worker credential id
  (1 row/s, burst 60, reset on restart; ADR-0001 single coordinator). Denials beyond it are answered but not audited.
- Migration 0004 (TASK-M5-BE-02): `job_events.txid xid8 DEFAULT pg_current_xact_id()` + partial index (stream feed
  visibility cursor, RV5-04; pre-0004 rows keep NULL); the credential idempotency index above; and least privilege for
  `clusterx_runtime` (RV5-10): no INSERT/UPDATE on `job_tombstones`, no UPDATE on `job_events`/`audit_events`.
- Migration 0003 is additive (indexes only) for metrics series and session retention.
