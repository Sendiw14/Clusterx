# CLUSTERX — Backup and restore (AC-17)

Status: **drilled locally** (M8, TASK-M8-BE-01) on the isolated compose stack. Evidence:
`docs/testing/evidence/m8/backup-restore/` (`drill.txt`, `backup-toc.txt`). The backup archive itself is not committed
(it contains job payloads/results and credential hashes).

## What is backed up

One logical `pg_dump` custom-format archive of the `clusterx` database: schema, `schema_migrations`, jobs, attempts, events,
idempotency keys, users, sessions, token and worker-credential **hashes** (never secrets), audit. Not included: cluster roles
and their passwords (recreated by `infra/compose/initdb/01-roles.sh` or your provisioning), compose secret files, TLS keys,
Prometheus data. Back those up separately through your secret store.

## Commands

```bash
# Backup (runs pg_dump inside the postgres container as the schema owner clusterx_admin; no DB port needed)
scripts/ops/backup.sh /secure/backups                     # -> clusterx-clusterx-<UTC>.dump (0600) + .sha256 + .toc
CLUSTERX_BACKUP_KEEP_DAYS=14 scripts/ops/backup.sh /secure/backups   # also prunes this script's archives > 14 days (§6)

# Production example stack
CLUSTERX_COMPOSE_DIR=infra/production CLUSTERX_ENV_FILE=infra/production/.env.production scripts/ops/backup.sh /secure/backups

# Restore into a NEW database (refuses an existing name; verifies the checksum; single transaction; stops on first error)
scripts/ops/restore.sh /secure/backups/clusterx-clusterx-<UTC>.dump clusterx_restored_<date>
```

Then point the control plane at the restored database and restart it (the database name is part of
`secrets/database_url` / `.secrets/database_url`), start the workers, and keep the old database until the restored one is
verified. Scheduling: run `backup.sh` from cron or a systemd timer on the host; there is no in-product scheduler.

## Recovery behaviour after a restore

The restored database is the state at backup time. On start the control plane:

- serves jobs exactly as they were (counts per status unchanged at start, drill below);
- treats every attempt that was `running` at backup time as a lost lease: after the lease TTL (30 s default) the attempt
  becomes `lease_expired` and the job is retried or ends per its policy (ADR-0003) — never stuck `running`;
- accepts the same worker credentials and API tokens (hashes are in the archive); workers register new incarnations.

**Data loss = everything committed after the backup started** (jobs, events, cancels, credentials). Clients that
retry a submission with the same `Idempotency-Key` get a **new** job, because the key was lost too. Handlers that ran
between backup and disaster may run again (at-least-once, §16).

## Drill (executed) — `scripts/ops/drill-backup-restore.sh`

Stack: `scripts/bench/up.sh` (project `clusterx-m8`, 3 workers, Colima VM — see `docs/testing/benchmarks.md`).

1. Populated: 20 succeeded, 5 failed, 3 canceled, 5 scheduled (+1 h), 12 running (**12 active attempts**, `sleep_ms` 20 s),
   11 queued.
2. Backup: `pg_dump -Fc` 83,491 bytes, 131 TOC entries, **0.263 s**.
3. 10 more jobs submitted after the backup (the RPO window), then "disaster": control plane and workers stopped.
4. Restore into `clusterx_restored_185308`: **0.338 s** (archive verify + createdb + pg_restore + ANALYZE).
5. Control plane re-pointed and restarted: ready (`/health/ready` 200) **0.216 s** after the restore finished.
6. Immediately after start the restored DB had the same per-status counts as at backup time (12 running).
7. After the 30 s lease TTL: 12 attempts `lease_expired`, all retried and finished; **all non-scheduled jobs terminal
   57.7 s after the restart** (12 × 20 s jobs re-run on 3 workers × 4 slots).
8. Invariants on the restored DB (`reconcile.sql`): attempt-count mismatch 0, event-version mismatch 0, active attempts 0,
   stuck running 0; 5 scheduled jobs still scheduled; post-backup jobs present: **0** (expected: lost).
9. Switched back to the original database.

| Measure | Value | Meaning |
|---|---|---|
| RPO window in the drill | 2.201 s | backup start → disaster; the 10 jobs written in it were lost, as designed |
| RTO, service | **14.116 s** | disaster → control plane ready on the restored DB. Of this, ≈ 13.5 s is `docker compose stop` (graceful shutdown of the control plane and 3 workers); restore 0.34 s, restart 0.22 s |
| RTO, work drained | 71.773 s | disaster → every non-scheduled job terminal (dominated by lease TTL 30 s + 20 s job re-runs) |

With a backup schedule of every *T*, the **achievable RPO is T plus the dump duration**, not the 2.2 s above. The
database here is tiny (56 jobs, 83 KB); restore time grows with data size and is **NOT measured at production volume**.

## Proposed objectives (not frozen; owner decision OD-06)

- RPO ≤ 24 h with daily `pg_dump` (≤ 1 h with hourly). Lower RPO needs WAL archiving / PITR, which is **not implemented**.
- RTO ≤ 30 min for a single-host restore (measured 14 s at 83 KB; to be re-measured at reference volume).
- Retention: 14 daily archives (`CLUSTERX_BACKUP_KEEP_DAYS=14`), stored encrypted off-host.

## Not done / limits

- No WAL archiving / point-in-time recovery; no replica.
- Restore drill at realistic volume (≥ 100k jobs) and on the reference Linux host: NOT RUN.
- Off-host copy and encryption of archives are the operator's responsibility (no tooling here).
- `pg_dump` takes a consistent snapshot without blocking writers; a very long dump holds an old snapshot (vacuum delay).
