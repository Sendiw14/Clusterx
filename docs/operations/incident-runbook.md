# CLUSTERX — Incident runbook

Status: M8. One section per alert in `infra/monitoring/alerts.yml` (the alert's `runbook_url` points here). Each alert was
induced locally; evidence and timings: `docs/testing/evidence/m8/alerts/` and the table at the end.

General first steps for any page:

```bash
docker compose --env-file <env> ps                       # what is running / restarting
docker compose --env-file <env> logs --since 15m control-plane | tail -200
curl -s https://<api-host>/health/ready                  # 200 = DB reachable and schema current
```

Prometheus (private, loopback): `http://127.0.0.1:<CLUSTERX_PROMETHEUS_PORT>/alerts`. No Alertmanager is shipped; wire
notification routing to your on-call system before relying on these alerts (OD-04: no external service approved).
The `clusterx_*` gauges are read from the database on every scrape: when the database is down they disappear and only
`clusterx_db_up 0` remains, so DB-derived alerts resolve while `ClusterxDatabaseUnavailable` fires.

## ClusterxControlPlaneDown

`up{job="clusterx-control-plane"} == 0` for 1 m — Prometheus cannot scrape the internal metrics listener.

- Impact: the single coordinator is down or unreachable: no admissions, no claims, workers retry with backoff (they never
  assume a lease is still authoritative, ADR-0003). Acknowledged jobs are durable in PostgreSQL.
- Check: `ps` (exited? restarting?), `logs control-plane` (config errors fail closed with exit 1 and list every problem;
  a schema/readiness error means run `migrate`), host memory (`mem_limit`), the metrics token file.
- Fix: correct the configuration/secret and `up -d control-plane`. After restart, expired leases are recovered by the
  scheduler; no manual job surgery.

## ClusterxDatabaseUnavailable

`clusterx_db_up == 0` for 30 s — the control plane is up but cannot read PostgreSQL.

- Impact: `/health/ready` 503; admissions answer 503 `unavailable` (never a 2xx without commit, AC-08); claims and reports
  fail and workers back off; leases may expire meanwhile and are recovered after the DB returns.
- Check: `ps postgres`, `logs postgres` (disk full? `PANIC`/`could not write`), `ClusterxDiskSpaceLow`, connection
  limits (`max_connections`), credentials (`database_url` secret / role password).
- Fix: restore the database service; if storage is lost, follow `backup-restore.md` (restore into a new database).

## ClusterxQueueAgeHigh

`clusterx_queue_oldest_age_seconds > 300` for 2 m — an eligible queued job has waited more than 5 minutes.

- Impact: work is not being picked up fast enough (capacity, or no capable worker).
- Check: `GET /api/v1/metrics/overview` (live/stale workers, unschedulable count); `GET /api/v1/workers`; whether
  `ClusterxJobsUnschedulable` or `ClusterxNoLiveWorkers` also fires; worker logs for claim errors (`unknown_handler`,
  `credential_revoked`, `worker_draining`).
- Fix: start/scale workers (`docker compose up -d worker-N`), un-drain, deploy a worker that supports the handler version.

## ClusterxJobsUnschedulable

`clusterx_jobs_unschedulable > 0` for 5 m — queued jobs whose handler/version no live, active worker advertises.

- Check: `GET /api/v1/jobs?status=queued&unschedulable_reason=<reason>` (or the console's unschedulable filter),
  `GET /api/v1/workers` capabilities.
- Fix: deploy a worker with the handler version, or cancel the jobs. Jobs stay queued and accounted; nothing is lost.

## ClusterxWorkerOffline

`clusterx_workers_recently_lost > 0` for 1 m (warning) — a worker stopped heartbeating within the last hour without
deregistering (crash, OOM kill, network partition, host loss). Also `ClusterxNoLiveWorkers` (critical): jobs queued and
no live worker for 2 m.

- Impact: its active attempts expire after the lease TTL (30 s default) and are retried per policy (AC-04); a handler that
  was mid-way may run again (at-least-once).
- Check: `docker compose ps` / `logs worker-N` (exit code 137 = SIGKILL/OOM), `GET /api/v1/workers?liveness=stale`.
- Fix: restart the worker (`up -d worker-N`); it registers a new incarnation. The stale row stops counting after 1 h.
  If the credential may be compromised, revoke it (`POST /api/v1/worker-credentials/{id}/revoke`) and issue a new one.

## ClusterxLeaseExpiriesRepeated

`sum(clusterx_attempts_finished_5m{state=~"lease_expired|deadline_exceeded"}) >= 3` for 1 m.

Both outcomes count as crash-type: the lease is capped at the hard deadline, so for a job with
`timeout_seconds <= lease_ttl` (e.g. `hash_sha256`, 30 s = default TTL) a crashed worker is recorded `deadline_exceeded`
and the exhausted job `failure_class=timeout`, never `lease_expired` (ADR-0003 Recovery, state-machine E8). The sum also
includes handlers that genuinely overran their timeout while heartbeating.

- Causes: workers dying (see above), workers unable to renew (control-plane overload, network loss, DB lock timeouts
  answering `unavailable`), handlers blocking the renew loop, clock/timing misconfiguration (lease TTL too short for
  heartbeat interval).
- Check: worker logs for `RenewLease` errors, control-plane logs for `unavailable`, `ClusterxWorkerOffline`.
- Tell crash from overrun: `clusterx_attempts_finished_5m` by `state`; for `deadline_exceeded` attempts compare the job's
  `timeout_seconds` with the lease TTL. Timeout ≤ TTL plus `ClusterxWorkerOffline` or worker restarts ⇒ crash; timeout
  well above TTL with live workers ⇒ handler overrun (raise `timeout_seconds` or fix the handler).
- Fix the cause; repeated expiries burn the attempt budget (`max_attempts`) and end jobs `failed` (`lease_expired`, or
  `timeout` for deadline outcomes).

## ClusterxFailureRateHigh

More than 25 % of attempts that ended in the last 5 minutes were `failed`, `lease_expired` or `deadline_exceeded`, with at
least 20 attempts, for 2 m.

- Check: `GET /api/v1/jobs?status=failed` grouped by `failure_class` / handler; attempt `error_code` values; a recent
  deploy of a handler or of a dependency it calls.
- Fix: roll back the handler/worker image; cancel or stop submitting the failing job type. Manual retry
  (`POST /jobs/{id}/retry`) only after the cause is fixed.

## ClusterxDiskSpaceLow

`node_filesystem_avail_bytes / node_filesystem_size_bytes < 0.10` on the PostgreSQL volume (`/pgdata` in node-exporter)
for 2 m.

- Impact: PostgreSQL stops accepting writes when the disk is full (then `ClusterxDatabaseUnavailable`).
- Check: `docker system df -v`, `du` of the volume, `SELECT pg_size_pretty(pg_database_size('clusterx'))`
  (`clusterx_db_size_bytes`), WAL growth (long transactions, failed archiving), retention running
  (`docker compose --profile retention run --rm retention`).
- Fix: run retention, remove old backups/images, grow the volume. Never delete files inside the PostgreSQL data directory.

## ClusterxNodeExporterDown

`up{job="node"} == 0`, or no `up{job="node"}` series at all, for 2 m (RV9-11).

- Impact: no filesystem metrics, so `ClusterxDiskSpaceLow` cannot fire. The engine itself is unaffected.
- Check: `docker compose --profile monitoring ps node-exporter`, its logs, and the `node` target on Prometheus' `/targets`.
- Fix: restart node-exporter (`docker compose --profile monitoring up -d node-exporter`). Meanwhile, check disk by hand
  (`df` on the volume host path, `clusterx_db_size_bytes`).

## Induced locally (M8 drill)

See `docs/testing/evidence/m8/alerts/README.md` for the timeline and Prometheus API captures.
