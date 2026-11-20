# HANDOFF — TASK-M9-BE-02 (REVIEW-M9-RV-02 ops/backend fixes)

- **Author:** backend lane subagent, model `claude-opus-5-5`, requested effort medium. Effective effort and Ultracode:
  UNVERIFIED from inside the subagent (MW-04).
- **Branch:** `m9fix2/backend` (worktree `.worktrees/backend`), base main `aafa25a`. Not pushed, not merged.
- **Source findings:** REVIEW-M9-RV-02 RV9-02, 04, 05, 06, 07, 08, 09, 11. RV9-01 was already on main (`aafa25a`).
  RV9-03 (public copy) and RV9-10 (CI) are not in this ticket.
  Acceptance touched: AC-27 / MASTER §6 data lifecycle (retention), §12 (production example hardening), AC-17/18 (alerting).
- **Contract changes:** none to SQL schema, OpenAPI or proto. Behaviour changes for integrator awareness:
  1. `CLUSTERX_RATE_MUTATIONS_PER_SECOND=0` now fails config load when `CLUSTERX_ENV=production` (dev unchanged; the
     bench overlay runs in development, so it is unaffected).
  2. `/metrics` serves one scrape at a time; a concurrent scrape gets **503** (promhttp `MaxRequestsInFlight`), and a
     scrape is cut at 10 s.
  3. Production compose: `CLUSTERX_GRPC_ADDR=cp-grpc:9090`, `CLUSTERX_METRICS_ADDR=cp-metrics:9091` (hostnames,
     resolved at listen time to the container's address on `workers` / `monitoring`). Prometheus target is now
     `cp-metrics:9091` (TLS server name still `control-plane`). New secret file `retention_database_url`.
- **Review owner:** independent reviewer (RV). No integrator-review (SQL) files touched.

## Commits

| SHA | Content |
|---|---|
| `d0f6b90` | RV9-05 metrics in-flight bound + race test; RV9-08 prod fail-closed + config tests; RV9-11 `ClusterxNodeExporterDown` + promtool tests + runbook section; RV9-09 drill/backup hygiene |
| `fa88026` | RV9-02 retention (CP loop 1h + `retention` profile one-shot + setup DSN); RV9-04 Caddy 405; RV9-06 network-scoped listeners, Prometheus off `edge`; RV9-07 nonroot Caddy; verify-script probes; ops/security docs |
| (this) | handoff |

## Changed files

- `apps/control-plane/internal/api/server.go`, `apps/control-plane/internal/api/api_test.go`
- `apps/control-plane/internal/config/config.go`, `apps/control-plane/internal/config/config_test.go`
- `infra/monitoring/alerts.yml`, `infra/monitoring/alerts_test.yml`
- `infra/production/Caddyfile`, `compose.yaml`, `prometheus.yml`, `setup-local-proof.sh`, `verify-local-proof.sh`
- `scripts/ops/backup.sh`, `scripts/ops/drill-backup-restore.sh`
- `docs/operations/deployment.md`, `docs/operations/upgrades.md`, `docs/operations/incident-runbook.md`
- `docs/security/threat-model.md` (T-P2: Caddy no longer root)

## Per item

| ID | Status | What changed | Evidence |
|---|---|---|---|
| RV9-02 | **Fixed** | Control plane gets `CLUSTERX_RETENTION_DATABASE_URL_FILE` + `CLUSTERX_RETENTION_INTERVAL=1h` (existing ADR-0004 loop, own pool, `clusterx_retention_job`). New `retention` profile one-shot (`retention run`, `data` network only). `setup-local-proof.sh` writes the retention DSN | Local proof: job aged 31 d (admin fixture, guard trigger off for that statement), one pass: `jobs_deleted=1 complete=true`, `jobs_row 1→0`, `tombstone 0→1`, API `GET /jobs/{id}` → **410** |
| RV9-04 | **Fixed** | `(common)`: `@badmethod not method GET HEAD POST PUT PATCH DELETE OPTIONS` → `handle { Allow…; respond 405 }`. `caddy adapt` shows it ahead of `reverse_proxy` (web) and ahead of `handle @public` (API) | TRACE and PROPFIND → **405** on both hosts; OPTIONS on API passes through (Go JSON 405) |
| RV9-05 | **Fixed** | `promhttp.HandlerOpts{MaxRequestsInFlight: 1, Timeout: 10s}` | `TestMetricsBoundsConcurrentScrapes` (race): 2nd concurrent scrape 503, 1st 200, next one 200 after release |
| RV9-06 | **Fixed** | gRPC/metrics bound to network aliases; Prometheus on `monitoring` + `prometheus_ui` (port-publishing only), not `edge` | nc probes: edge→8080 open, edge→9090 **closed**, edge→9091 **closed**; workers→9090 open, workers→9091 closed; monitoring→cp-metrics:9091 open, monitoring→9090 closed. Prometheus targets `https://cp-metrics:9091/metrics up`, `node up`, 10 rules loaded. Workers `tls:true` to `control-plane:9090`, job succeeded |
| RV9-07 | **Fixed** | Caddy `user: 65532:65532`, `cap_add` removed, `sysctls: net.ipv4.ip_unprivileged_port_start: 0`. Image `/data/caddy`, `/config/caddy` are 1777, so fresh volumes work. Upgrade note with a tested chown command for volumes from a root Caddy | `docker inspect`: `caddy-1 user=65532:65532 cap_drop=[ALL] cap_add=[]`, serves 443/80; no error-level Caddy log lines; chown command exit 0 |
| RV9-08 | **Fixed** (fail closed) | `prod && RateMutations == 0` → config error | config tests: `prod mutation limit off` → error, `prod … 5` ok, `dev … 0` ok |
| RV9-09 | **Fixed** | Drill parks the live DSN at `infra/compose/.secrets/database_url.drill-orig` (gitignored, umask 077), never in the evidence dir; `trap cleanup EXIT` restores it and recreates the control plane on any exit. `backup.sh` traps removal of `.partial` and `.toc` | `bash -n`; `git check-ignore` matches `.secrets/`. **Drill itself NOT re-run** |
| RV9-11 | **Fixed** | `ClusterxNodeExporterDown: up{job="node"} == 0 or absent(up{job="node"})`, for 2m, warning; runbook section | promtool `check rules`: SUCCESS 10 rules; `test rules`: SUCCESS (up==0 case and absent case) |

## Commands and exits

| Command | Exit |
|---|---|
| `go vet ./... && go test -race -count=1 ./...` in control-plane, worker, proto, sdk-go, examples/basic-jobs (PG 18.7 Homebrew on 127.0.0.1:55432, TCP only, `CLUSTERX_PG_ADMIN_DSN`, PG18 bin first in PATH) | 0 ×5 (first attempt had chaos test fail because PATH lacked PG18 → PG14 `initdb`; rerun with correct PATH green) |
| `promtool check rules alerts.yml` / `test rules alerts_test.yml` (pinned `prometheus:v3.16.0@sha256:9332…`) | 0 / 0 |
| `CLUSTERX_ENV_FILE=.env.m9ops ./setup-local-proof.sh` (project `clusterx-m9ops`, prefix `clusterx-m9ops`, ports 127.0.0.1:18643/18680, Prometheus 19490) | 0 |
| `CLUSTERX_ENV_FILE=.env.m9ops ./verify-local-proof.sh` (run twice; second after the chown test) | 0 / 0 |
| `docker compose … --profile monitoring up -d prometheus node-exporter` + `/api/v1/targets` | targets up |
| `docker compose … --profile monitoring --profile retention down -v`; `docker rmi clusterx-m9ops/*` | 0 |

Cleanup: no `clusterx-m9ops` container, network, volume or image remains; `.env.m9ops` and the generated `secrets/` in
the worktree were deleted; PG cluster on 55432 stopped. Project `clusterx` was only listed (`running(5)` before and after).

## Limitations / not done

- The hourly in-process loop was not observed firing (1 h ticker); the proof runs the same `RunRetention` via the
  one-shot. The loop has no immediate pass at start.
- Backup/restore drill and alert drill not re-run; RV9-09 is checked by syntax and ignore rules only.
- HTTP→HTTPS redirect still targets 443 (pre-existing, documented in deployment.md).
- Network scoping relies on Docker DNS resolving the alias at listen time; a control plane outside compose must set
  explicit addresses. gRPC is still reachable from any `workers` peer (by design).
- Evidence was captured in the session scratchpad, not committed (evidence dirs are outside this ticket's allowed
  paths); the per-item table above quotes it.
- RV9-03 (public copy, web lane) and RV9-10 (CI, needs a real run) remain open.

## Integration needs

- Web lane: the public "deleted 30 days after they finish" claim is now true for the production example only while
  retention runs (it does by default); RV9-03 copy pass can cite `deployment.md` Retention paragraph.
- Anyone running the production example from an older checkout must re-run `setup-local-proof.sh` (creates
  `retention_database_url`) and, if Caddy volumes exist, run the chown in `upgrades.md`.
