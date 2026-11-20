# HANDOFF — TASK-M8-BE-01 (operations: monitoring/alerts, benchmarks, backup/restore, production example, security docs)

- **Author:** backend lane subagent, model `claude-opus-5-5`, requested effort medium. Effective effort and Ultracode:
  UNVERIFIED from inside the subagent (MW-04). The run was interrupted once by a usage limit and resumed; no state lost.
- **Branch:** `m8/backend` (worktree `.worktrees/backend`), base main `efcb7f7` (the ticket says `4c68744`; `efcb7f7` is
  that commit plus the ticket). Not pushed, not merged.
- **Requirements / acceptance:** NFR-OBS-01, NFR-OPS-01/02, NFR-PERF-03, NFR-SCALE-01, FR-ENG-23/24, FR-SEC-08..10;
  AC-17, AC-18, AC-19, AC-25, AC-26/AC-27 (local only, OD-04). RV01-15 re-measured.
- **Contract changes:** none. OpenAPI, proto and migrations 0001–0004 unchanged. New Prometheus series on the internal
  `/metrics` listener only (not part of OpenAPI).
- **Review owner:** independent reviewer (RV). Integrator review: `.github/workflows/security.yml`, `.gitignore`,
  `.dockerignore`, `.gitleaksignore` (repo-root files outside the ticket's listed paths), the `infra/compose/compose.yaml`
  additions.
- **Environment:** macOS (Apple M4, 10 cores, 16 GiB); Colima VM aarch64 4 vCPU / 6 GiB / 40 GiB, Docker 29.5.2;
  Go 1.27.4; native PostgreSQL 18.7 on `localhost:55450` for Go tests. The registry / module-proxy network was unreliable
  all session (Docker Hub pulls stalled; npm and Go proxy connection resets and DNS failures).

## Commits

| SHA | Slice | Content |
|---|---|---|
| `1fd8682` | monitoring (code) | `store.OpsSnapshot` + `api.OpsCollector`: `clusterx_db_up`, `clusterx_jobs{status}`, `clusterx_queue_oldest_age_seconds`, `clusterx_jobs_unschedulable`, `clusterx_workers{state}`, `clusterx_workers_recently_lost`, `clusterx_attempts_finished_5m{state}`, `clusterx_db_size_bytes`; tests; opt-in `TestExplainAtScale`; EXPLAIN evidence |
| `c10d8af` | benchmarks + drills | `scripts/bench/*`, `scripts/ops/{backup,restore,drill-backup-restore}.sh`, `TestUpgradeDrill0002To0004`, raw evidence |
| `ad52ce3` | monitoring + alerts | `infra/monitoring/{prometheus.yml,alerts.yml,alerts_test.yml,drill/}`, `monitoring` compose profile, `scripts/ops/drill-alerts.sh`, incident runbook, `stop_grace_period: 40s` for workers, alert evidence |
| `0f6865d` | dependency | `golang.org/x/net` v0.60.0 → v0.62.0 in all 5 modules (GO-2026-6860, GO-2026-6866, not reachable) |
| `9fb0e3e` | security + docs | threat model, ASVS 5.0 verification, `security.yml`, `scripts/security/*`, `.gitleaksignore`, `.dockerignore`, benchmarks/backup/upgrade docs, dependency matrix |
| `bf9066d` | production example | `infra/production/*`, deployment doc, production evidence |
| (this) | handoff | this file |

## Commands and exit codes (all executed)

| # | Command | Exit | Evidence |
|---|---|---|---|
| 1 | `apps/control-plane: go vet ./... && go test -race -count=1 ./...` (after `1fd8682`) | 0 | `evidence/m8/cp-race-1.txt` |
| 2 | `GOWORK=off go vet ./... && go test -race -count=1 ./...` in all 5 Go modules (after the x/net bump) | 0 ×5 | `evidence/m8/go-vet-race-all-modules.txt` |
| 3 | `CLUSTERX_EXPLAIN_SCALE=1 go test -run TestExplainAtScale ./internal/store` | 0 | `evidence/m8/explain/` |
| 4 | `go test -race -run TestUpgradeDrill0002To0004 ./internal/store` | 0 | `evidence/m8/backup-restore/upgrade-drill-0002-to-0004.txt` |
| 5 | `scripts/bench/up.sh`; `run.sh {1,3,5}` ×3; `run.sh 3 … kill`; `down.sh` | 0 each | `evidence/m8/bench/` |
| 6 | `scripts/ops/drill-backup-restore.sh` | 0 | `evidence/m8/backup-restore/drill.txt` |
| 7 | `promtool test rules alerts_test.yml`; `promtool check rules alerts.yml` | SUCCESS | `evidence/m8/alerts/promtool-test-rules.txt` |
| 8 | `scripts/ops/drill-alerts.sh` | 0 | `evidence/m8/alerts/` |
| 9 | `scripts/security/scan.sh … govulncheck` / `pnpm` / `gitleaks` | 0 / 0 / 0 (after triage) | `evidence/m8/security/` |
| 10 | production: fail-closed `docker run` checks | 1 ×4 (expected) | `evidence/m8/production/fail-closed.txt` |
| 11 | `CLUSTERX_SKIP_BUILD=1 infra/production/setup-local-proof.sh`; `verify-local-proof.sh` | 0 | `evidence/m8/production/e2e.txt`, `internal-tls.txt`, `backup.txt` |

Earlier failed attempts (disclosed): the first benchmark loop ran under zsh and passed no arguments (no runs, discarded);
the first alert drill aborted (worker not yet started) and the second found a worker draining from an earlier stop — the
script now waits/restarts; the first web image builds failed on npm `ECONNRESET` (BuildKit/buildx missing → legacy builder;
retry succeeded). A first set of benchmark runs (before cgroup counters were added) was discarded in favour of the
committed set.

## Measured numbers

**AC-18 (1,000 jobs, 3 workers, exact):** 3/3 runs: 1000 accepted, 900 succeeded, 100 failed, 1100 attempts = Σ
attempts_used, 100 retries, 0 attempt-count mismatches, 0 event-version mismatches, 0 active attempts, 0 non-terminal.
**Worker SIGKILL mid-run:** 4 active attempts on the killed worker → 3 `lease_expired` + 1 `deadline_exceeded`, all
retried, 1104 attempts, still 900/100, max end-to-end 34.1 s (lease TTL 30 s).

**AC-19 (3 repetitions each):**

| Workers | Admission p50/p95/p99 ms (range over runs) | End-to-end p50 / p95 / p99 ms | Throughput jobs/s |
|---|---|---|---|
| 1 | 4.1–5.6 / 5.8–10.0 / 11.5–14.8 | ≈3.3 s / 6.0–6.2 s / 6.9–7.3 s | 114.7–129.9 |
| 3 | 4.0–7.4 / 6.3–12.0 / 9.2–14.9 | 0.9–1.06 s / 2.9–3.5 s / 3.7–4.0 s | 199.2–233.9 |
| 5 | 5.9–7.4 / 7.3–9.9 / 9.3–15.7 | 0.54–0.55 s / 2.2–2.7 s / 3.4–3.7 s | 242.3–280.5 |

DB contention: 0 deadlocks, 0 rollbacks, 0 lock waits > 100 ms in all runs; control plane ≤ 16 MiB, PostgreSQL ≤ 193 MiB,
worker ≤ 9 MiB. Scaling is not linear (≈1.7× then 1.2×); the 2 s retry backoff bounds the run; the pgx pool (≤ 4 conns)
is the probable next bottleneck. Full tables and SLO proposal (not frozen): `docs/testing/benchmarks.md`.

**RV01-15 (EXPLAIN ANALYZE BUFFERS, 120k queued / 150k terminal / 270k events / 150k idempotency):** claim candidates
0.40 ms via `jobs_claimable_idx` (8 rows per capability), list 0.59 ms, filtered list 0.20 ms, idempotency lookup
0.008 ms, feed poll 0.005 ms, `/metrics` ops snapshot 60 ms (linear in rows: status counts). 19 concurrent claimants:
p50 18.3 ms, p95 59.2 ms, p99 63.8 ms per claim transaction (auto_explain logging on). **RV01-15: closed by measurement.**

## Drills

- **Backup/restore (AC-17):** populated (20 succeeded, 5 failed, 3 canceled, 5 scheduled, 12 running with 12 active
  attempts, 11 queued) → `pg_dump -Fc` 0.263 s → 10 post-backup jobs → disaster → restore into a new DB 0.338 s →
  control plane ready 0.216 s later. **RPO window 2.201 s** (the 10 jobs lost, as designed), **RTO service 14.116 s**
  (≈13.5 s of it is `docker compose stop`), **RTO work drained 71.773 s**. Invariants on the restored DB: all 0; 12
  `lease_expired` attempts recovered, 0 stuck running.
- **Migration upgrade:** populated schema 0002 (all 7 statuses, active expired lease, events, idempotency) → 0003+0004
  applied in 2 ms → invariants hold, pre-0004 events `txid NULL`, current engine recovers the lease and completes work.
- **Alerts:** all 9 rules reached `firing` by induction (worker SIGKILL, permanent failures, queue age, no capable worker,
  tmpfs volume filled to 95 %, `stop postgres` — admission answered 503 —, `stop control-plane`). Timeline:
  `evidence/m8/alerts/README.md`. Caveat: `ClusterxWorkerOffline` was already firing at baseline from earlier runs; the
  induced kill raised its value 2 → 3.
- **Production example (local, self-signed CA):** TLS 1.3 both hosts; job executed by 3 workers over gRPC+TLS; BFF
  container → control plane over TLS 200; DB unreachable from web; only Caddy publishes ports; nonroot/RO/cap_drop;
  no secret values in env; Prometheus scrapes over TLS; fail-closed exit 1 ×4.

## Supply-chain scans

govulncheck v1.9.0: after the x/net bump **no vulnerabilities** in all 5 modules (before: 2 unreachable in x/net).
`pnpm audit`: **0** (560 deps). gitleaks v8.30.1: 3 history + 1 tree findings, all test-fixture false positives (RFC 6455
sample WebSocket key, a test metrics token) → `.gitleaksignore`, rerun 0. **osv-scanner and syft: NOT RUN locally** (their
`go install` failed repeatedly on module-proxy network errors); both run in `security.yml`.

## NOT RUN

- osv-scanner and syft SBOM locally (network); `security.yml` itself has not run in CI (not pushed).
- amd64 / Linux reference host for anything (OD-04); public DNS, ACME, real GitHub OAuth login; firewall; remote workers.
- Restore and migration timing at production volume (≥ 100k jobs); `CREATE INDEX` lock time of 0003 on large tables.
- Benchmarks longer than one 1,000-job burst; amd64; with the default mutation rate limit on.
- Alertmanager / notification delivery (none shipped).
- Web e2e (Playwright) against the production image.

## Findings / integration needs

1. **Worker stop timeout (fixed here):** compose's default 10 s stop grace SIGKILLed workers mid-drain (worker shutdown
   timeout 30 s), leaving a `draining` row; the next start registered draining, deregistered and exited 0 once (sticky
   drain, RV5-02). `stop_grace_period: 40s` added to workers in both compose files; bench evidence predates it.
2. **Recovery classification:** when `timeout_seconds` equals the lease TTL (hash_sha256: 30 s), a crashed worker's
   attempt is recorded `deadline_exceeded`, not `lease_expired` (observed in w3-kill). Reviewer: confirm intended.
3. **Rate limiting behind Caddy:** all direct-API clients share Caddy's peer address for the auth-failure bucket
   (threat T-A2); trusted-proxy client IP handling needs a design decision (backend).
4. **Web lane:** native `*_FILE` secrets in `apps/web/src/server/config.ts` (wrapper `infra/production/web-entry.mjs`
   used meanwhile); nonce CSP; a pruned/standalone image (current 1.43 GB).
5. Images: Prometheus from quay.io (its digest differs from Docker Hub's), node-exporter quay.io (same digest), Caddy and
   Node from ECR Public (same digests as Docker Hub) — `dependency-matrix.md`.
6. `docs/security/permissions.md` stays the access matrix; `threat-model.md`/`verification.md` reference it.

## Cleanup

`clusterx-m8` and `clusterx-m8-prod` compose projects: `down -v`, images `clusterx-m8/*`, `clusterx-m8p/*` removed,
generated `.env`, `.env.production`, `secrets/`, `.secrets*` deleted. The integrator's `clusterx` project was only read
(`docker compose ls`: `running(5)` before and after). Backup archives stay in the session scratchpad (not committed).
