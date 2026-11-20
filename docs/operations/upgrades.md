# CLUSTERX — Upgrades, migrations and rollback

Status: M8. Executed: the 0002 → 0004 migration drill (`TestUpgradeDrill0002To0004`) and the backup/restore drill
(`backup-restore.md`). A rolling upgrade of a live multi-version deployment has **not** been exercised.

## Rules

- Migrations are **forward-only, versioned** SQL files embedded in the control-plane binary
  (`apps/control-plane/migrations/NNNN_*.sql`). Shipped files are byte-identical forever; `migrate` refuses drift
  (checksum) and applies each file in its own transaction under an advisory lock (concurrent runners apply once).
- **Expand/contract**: a release only adds (columns nullable or defaulted, new indexes, new tables, narrower grants with
  code that no longer needs them). Removal happens in a later release once no running version uses the object.
- The runtime role (`clusterx_app` ∈ `clusterx_runtime`) cannot run DDL; migrations run as the schema owner through the
  one-shot `migrate` service (`CLUSTERX_MIGRATE_ON_START=false` in production).
- **Rollback = restore**, not down-migrations. Schema rollback is unsafe once new code has written new columns.

## Upgrade sequence (single host)

1. Read the release notes: migrations included, protocol version, OpenAPI version, any config change.
2. **Back up**: `scripts/ops/backup.sh /secure/backups` and confirm the `.sha256` and `.toc` files.
3. Build or pull the new images, pinned by digest (`CLUSTERX_VERSION=<tag>`).
4. Drain workers (`POST /api/v1/workers/{id}/drain` or SIGTERM; ADR-0003) and wait for active attempts to finish or the
   drain grace to pass.
5. Stop the control plane (single coordinator: admissions answer connection errors during this window — §16 downtime).
6. Run migrations: `docker compose --env-file <env> run --rm migrate` (exit 0 required; it logs `applied N`).
7. Start the new control plane; check `/health/ready` = 200 (readiness also checks that the schema version matches the
   binary's migrations).
8. Start the workers on the new image; check `clusterx_workers{state="live"}` and submit one smoke job.
9. Keep the pre-upgrade archive for at least the retention window.

Compatibility: workers and control plane negotiate the gRPC protocol version at Register; an unknown handler or version
is rejected per job, not per worker (AC-10). Mixed worker versions are allowed if they implement the same handler
versions. The web BFF and the control plane share OpenAPI `1.0.0-draft.7`; deploy them together.

## Rollback

If step 6 or 7 fails:

- A failed migration file is rolled back by its own transaction (`TestFailedMigrationLeavesNoTrace`); start the **old**
  binary again — the schema is unchanged.
- If the new version already ran and wrote data: stop it, restore the pre-upgrade archive into a new database
  (`scripts/ops/restore.sh`), point the **old** binary at it. Work committed after the backup is lost (see RPO in
  `backup-restore.md`).

## Drill: populated schema 0002 → 0003/0004 (executed)

`apps/control-plane/internal/store/upgrade_drill_test.go` (runs in CI with the other DB tests):

1. Fresh database, migrations 0001–0002 only (M4 schema).
2. Seed as the M4 engine wrote it: one job in each of the 7 statuses, a finished attempt for retry_wait/succeeded/failed,
   an **active attempt whose lease already expired** on the running job, 15 contiguous events, one idempotency key.
   Attempts are inserted through the RV01-05 guard (born active as the current lease, then finished).
3. Invariants before: attempts_used = attempts per job, `jobs.version` = event count = max event version, running ⇔
   one active attempt.
4. Apply the remaining migrations: **2 applied (0003, 0004) in 2 ms** on this tiny dataset; same invariants hold; status
   counts unchanged; the 15 pre-0004 events have `txid NULL` (never fed to the stream, ADR-0004).
5. Current engine on the upgraded schema: `RecoverExpired` expires the old lease (attempt `lease_expired`), the job is
   promoted, a new worker claims it and the queued job, both succeed; invariants hold; the recovered job has 2 attempts.

Output: `docs/testing/evidence/m8/backup-restore/upgrade-drill-0002-to-0004.txt` (`--- PASS`).

Limits: 0003 builds indexes with plain `CREATE INDEX` inside the migration transaction (SHARE lock on the table for the
build). At large table sizes this blocks writes for the build time: **measure before upgrading a large database**; the
build time at 100k+ rows was NOT measured.
