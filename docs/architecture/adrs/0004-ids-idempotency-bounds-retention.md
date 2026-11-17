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
- Retention: 24 h (`expires_at`). Concurrency: inserted in the **same transaction** as the job with
  `INSERT ... ON CONFLICT (principal, namespace, idem_key) DO UPDATE ... WHERE expires_at <= now()`;
  on 0 rows the transaction rolls back and the existing row is read:
  same fingerprint ⇒ 200 + original job (header `Idempotency-Replayed: true`);
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
| attempt `log_tail` | 7 d after attempt finish | **exception**: retention job sets `log_tail = NULL` via a dedicated SECURITY DEFINER function that bypasses the finished-attempt trigger; documented as the only post-final mutation |
| terminal jobs + attempts + events + idempotency rows | 30 d after `terminal_at` | `DELETE FROM jobs` batch, FK CASCADE; child `retried_from_job_id` → NULL |
| non-terminal jobs | never auto-deleted | |
| audit_events | 90 d | batch delete |
| idempotency | 24 h | batch delete |
| sessions | expiry + 7 d | batch delete |
Deleted jobs answer 404 `job_not_found`; the UI shows "data expired" when a referenced ID 404s and
its known `terminal_at` is older than retention. Figures are not legal advice (§6). Backups: 14 d.
The log-retention exception function is **not** in migration 0001 (M5, FR-ENG-23).

## Migrations
Forward-only, numbered `NNNN_name.sql`, each applied in one transaction, expand → backfill → contract
across releases; no destructive change in the same release that stops using a column.
