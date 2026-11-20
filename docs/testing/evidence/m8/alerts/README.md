# Alert drill — 2026-11-19 (UTC), `scripts/ops/drill-alerts.sh`

Stack: benchmark stack `clusterx-m8` (Colima VM, see `docs/testing/benchmarks.md`) + `--profile monitoring`
(Prometheus v3.16.0, node-exporter v1.12.1) + `infra/monitoring/drill/compose.disk-drill.yaml` (node-exporter watches a
64 MiB tmpfs volume at `/pgdata`). Scrape and rule evaluation every 15 s. Rules: `infra/monitoring/alerts.yml` as committed
(promtool: `promtool-test-rules.txt`, `SUCCESS`, 9 rules).

Files: `timeline.txt` (actions + first time the script saw each alert firing), `alerts-api.jsonl` (`/api/v1/alerts`
every 15 s: labels, state, activeAt, value), `rules-api.json` (final rule states).

| Alert | Induced by | Inducing action (UTC) | pending | firing |
|---|---|---|---|---|
| ClusterxWorkerOffline | SIGKILL worker-1 holding 4 attempts | 19:17:38 | — | already firing at baseline (value 2: two workers left stale by earlier runs — the w3-kill benchmark and a worker SIGKILLed by compose's 10 s stop timeout during the backup drill); value **2 → 3 at 19:18:30** |
| ClusterxLeaseExpiriesRepeated | same kill: 4 leases expired | 19:17:38 | 19:18:30 | **19:19:31** (value 4) |
| ClusterxFailureRateHigh | 40 `fail_n_times` permanent failures | 19:17:3x | 19:18:00 | **19:20:01** (ratio 1.0) |
| ClusterxNoLiveWorkers | no live worker, 30 jobs queued | 19:17:38 | 19:18:30 | **19:20:31** |
| ClusterxJobsUnschedulable | same (no capable live worker) | 19:17:38 | 19:18:30 | **19:23:31** (value 30) |
| ClusterxQueueAgeHigh | oldest queued job > 300 s | 19:17:3x | 19:23:01 (313 s) | **19:25:02** (433 s) |
| ClusterxDiskSpaceLow | `dd` 61 MiB into the 64 MiB volume | 19:25:02 | 19:25:32 | **19:27:32** (4.7 % free) |
| ClusterxDatabaseUnavailable | `docker compose stop postgres` | 19:27:34 | 19:27:47 | **19:28:17**; admission during the outage answered **HTTP 503** |
| ClusterxControlPlaneDown | `docker compose stop control-plane` | 19:28:20 | 19:28:47 | **19:29:47** |

All 9 alerts reached `firing`. While PostgreSQL was down the DB-derived alerts resolved (their gauges are not exported
when the snapshot fails), as documented in the runbook. No Alertmanager/notification delivery was tested (none shipped).
