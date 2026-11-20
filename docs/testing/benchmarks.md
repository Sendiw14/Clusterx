# CLUSTERX — Benchmarks (AC-18, AC-19) and query plans at scale (RV01-15)

Status: **measured locally, M8 (TASK-M8-BE-01)**. Every number below comes from a run whose raw output is committed under
`docs/testing/evidence/m8/`. These are **development-host numbers on a Docker VM**, not reference-host capacity: no
Linux amd64 reference VM exists yet (OD-04). Do not quote them as product performance.

## Environment (disclosed limits)

| Item | Value |
|---|---|
| Host | Apple M4, 10 cores, 16 GiB RAM, macOS (Darwin 27.0) |
| Docker | Colima VM `default`: **aarch64, 4 vCPU, 6 GiB RAM, 40 GiB disk**, kernel 6.8.0-117, Docker 29.5.2 |
| Stack | `infra/compose/compose.yaml` + `scripts/bench/compose.bench.yaml` (project `clusterx-m8`), images built from this branch |
| PostgreSQL | 18.7 (official image), default settings + `pg_stat_statements`, `log_lock_waits=on`, `deadlock_timeout=100ms`; data on the VM's virtual disk |
| Control plane | one container, default timing (lease TTL 30 s, heartbeat 10 s, backoff base 2 s), pgx pool default size (observed ≤ 4 connections + the sampler = `max_backends=5`) |
| Workers | `examples/basic-jobs` handlers, capacity 4 each, 256 MiB / 128 pids limit |
| Driver | `scripts/bench/driver` on the macOS host, 16 concurrent HTTP submitters to `127.0.0.1:18091` (Colima port forward adds host↔VM latency to admission numbers) |
| Deviation | per-principal mutation rate limit **disabled** (`CLUSTERX_RATE_MUTATIONS_PER_SECOND=0`): the default 20/s would measure the limiter, not admission |

Other load on the machine: the integrator's `clusterx` compose project (idle) was running in the same VM; no image pull or
test suite ran during the measured runs.

## Workload (fixed, deterministic)

`scripts/bench/driver/main.go` `Job(i)`, per block of 10 jobs, 1,000 jobs, `max_attempts=3`, one Idempotency-Key per job:

| i mod 10 | Handler | Expected outcome | Attempts |
|---|---|---|---|
| 0–5 | `hash_sha256` `{data}` | succeeded | 1 |
| 6–7 | `sleep_ms` `{ms:100}` | succeeded | 1 |
| 8 | `fail_n_times` `{n:1}` | retryable failure, then succeeded (2 s backoff) | 2 |
| 9 | `fail_n_times` `{n:1, permanent:true}` | failed (permanent) | 1 |

Expected exact totals: 1,000 jobs = 900 succeeded + 100 failed; 1,100 attempts; 100 automatic retries.

Commands (one stack, all runs): `scripts/bench/up.sh`; then `scripts/bench/run.sh <1|3|5> <dir>` three times each, and
`scripts/bench/run.sh 3 <dir> kill` once; `scripts/bench/stats.sh <dir>`; `scripts/bench/down.sh`.
Each run truncates the job tables (benchmark stack only), resets `pg_stat_*`, starts N workers, submits, waits until every
job is terminal, then runs `scripts/bench/reconcile.sql` against the database.

## AC-18 — exact reconciliation (1,000 jobs, 3 workers)

All ten runs reconcile exactly (`reconcile.txt` in each run directory). 3-worker runs:

| Run | Accepted (201) | Jobs | Succeeded | Failed | Attempts | Σ attempts_used | Retries | attempt-count mismatch | event-version mismatch | Active attempts left | Non-terminal |
|---|---|---|---|---|---|---|---|---|---|---|---|
| w3-r1 | 1000 | 1000 | 900 | 100 | 1100 | 1100 | 100 | 0 | 0 | 0 | 0 |
| w3-r2 | 1000 | 1000 | 900 | 100 | 1100 | 1100 | 100 | 0 | 0 | 0 | 0 |
| w3-r3 | 1000 | 1000 | 900 | 100 | 1100 | 1100 | 100 | 0 | 0 | 0 | 0 |
| w3-kill | 1000 | 1000 | 900 | 100 | **1104** | 1104 | 104 | 0 | 0 | 0 | 0 |

"event-version mismatch" = jobs whose `version` differs from `max(job_events.version)` or from the number of events.

**Worker killed mid-run (w3-kill):** `docker kill --signal KILL` of worker-2 after 300 terminal jobs, while it held
**4 active attempts** (`kill.txt`). Those 4 attempts ended `lease_expired` ×3 and `deadline_exceeded` ×1 (a
`hash_sha256` job whose 30 s timeout equals the 30 s lease TTL, so lease and hard deadline expire together and recovery
classifies it as a deadline), each job was retried automatically and succeeded: 4 extra attempts, still 900/100 outcomes,
no job stuck `running`. The slowest job took **34.1 s** end to end (≈ 30 s lease TTL + rerun) — that is the recovery cost of a
crashed worker under default timing.

## AC-19 — 1 / 3 / 5 workers (same workload, same stack, 3 repetitions)

Latency definitions: **admission** = client-side POST round trip (201 only); **end-to-end** = `terminal_at - created_at`
(DB clock); **queue wait** = first attempt `started_at - created_at`. Percentiles: driver nearest-rank; SQL
`percentile_cont`. Throughput = 1,000 / (last `terminal_at` − first `created_at`).

| Workers | Run | Admission p50 / p95 / p99 ms | End-to-end p50 / p95 / p99 ms | Queue wait p50 / p95 ms | Throughput jobs/s |
|---|---|---|---|---|---|
| 1 | r1 | 4.11 / 5.83 / 14.78 | 3253 / 5996 / 6941 | 2924 / 5443 | 122.6 |
| 1 | r2 | 5.18 / 9.98 / 12.14 | 3277 / 6248 / 7347 | 2884 / 5694 | 114.7 |
| 1 | r3 | 5.55 / 9.41 / 11.54 | 3296 / 6005 / 7337 | 2957 / 5365 | 129.9 |
| 3 | r1 | 4.03 / 7.43 / 9.20 | 1030 / 3540 / 3671 | 930 / 1597 | 206.7 |
| 3 | r2 | 5.05 / 6.32 / 10.76 | 908 / 2975 / 3960 | 803 / 1500 | 233.9 |
| 3 | r3 | 7.39 / 12.01 / 14.87 | 1059 / 2857 / 3976 | 985 / 1602 | 199.2 |
| 5 | r1 | 6.16 / 9.87 / 15.74 | 552 / 2614 / 3711 | 497 / 809 | 242.3 |
| 5 | r2 | 5.87 / 7.27 / 9.32 | 540 / 2222 / 3369 | 484 / 792 | 280.5 |
| 5 | r3 | 7.41 / 9.72 / 11.62 | 547 / 2690 / 3540 | 487 / 794 | 255.2 |

Resources (cgroup v2 counters read inside the VM, `stats.txt`; CPU-seconds over the run, memory peak since container start):

| Run | Wall s | control-plane CPU s | postgres CPU s | per-worker CPU s | control-plane mem peak | postgres mem peak | worker mem peak |
|---|---|---|---|---|---|---|---|
| w1-r1 | 9 | 0.98 | 2.32 | 0.28 | 13.4 MiB | 155.9 MiB | 8.7 MiB |
| w3-r1 | 6 | 0.77 | 1.67 | 0.07–0.08 | 14.0 MiB | 155.9 MiB | 8.0–8.5 MiB |
| w5-r1 | 5 | 0.81 | 1.76 | 0.04–0.05 | 14.2 MiB | 155.9 MiB | 7.0–7.7 MiB |
| w3-kill | 35 | 1.28 | 4.27 | 0.13–0.14 | 15.9 MiB | 192.5 MiB | 8.3–8.5 MiB |

(The "wall" column is whole seconds from driver start to the last terminal job, so average-cores figures in `stats.txt`
are coarse.)

DB contention (all runs): `deadlocks=0`, `conflicts=0`, `xact_rollback=0`; lock waits > 100 ms logged by PostgreSQL: **0**
in all ten runs; 1-s `pg_locks` samples with an ungranted lock: 1 sample in w1-r2 and 1 in w5-r3 (max 1 waiter), 0 elsewhere.
Top statements by total time (`pg_stat_statements`, w5-r1) are the claim CAS `UPDATE jobs`, the success `UPDATE jobs`
and the admission `INSERT INTO jobs`, each ≈ 0.10–0.12 ms mean, ≤ 0.8 ms max.

### Reading the numbers (limits, not claims)

- **The run is short.** All 1,000 jobs are admitted in 0.26–0.47 s, so end-to-end latency is mostly queue wait while the
  workers drain a full queue. 1 → 3 workers raised mean throughput ≈ 1.7× (122 → 213 jobs/s), 3 → 5 ≈ 1.2× (→ 259 jobs/s). Scaling is **not linear** and the
  data does not support extrapolation.
- **The 5-worker span is bounded by the retry backoff**: the 100 `fail_n_times n=1` jobs wait ≥ 2 s before their second
  attempt, so a 1,000-job run cannot finish in much under ~2.5–3.5 s whatever the worker count. Steady-state throughput at
  5 workers is therefore under-measured; a longer workload is needed before any capacity number is frozen.
- **Probable next bottleneck**: the control plane's pgx pool (≤ 4 connections observed) serialises claims, reports and
  admissions; PostgreSQL CPU (≈ 0.3 of 4 vCPU) and lock waits are not limiting at this size.
- Admission p99 includes the Colima port forward and the macOS→VM hop.
- arm64 VM only; amd64 reference host NOT RUN.

## Proposed SLOs (OD-06) — proposal only, NOT frozen

Measured on this development VM; to be re-measured on the reference host and frozen by the owner before RC (§11).

| Objective | Proposed target | Basis (this report) |
|---|---|---|
| Admission latency (201, steady load ≤ 1,000 jobs/burst) | p99 < 100 ms | measured p99 ≤ 15.7 ms (≥ 6× headroom for the reference host and network) |
| Queue wait with capacity available | p95 < 2 s | measured p95 0.8 s (5 workers) – 1.6 s (3 workers) for a 1,000-job burst |
| Recovery of a crashed worker's attempts | job retried within lease TTL + 10 s (40 s at defaults) | measured 34.1 s worst case |
| Exact accounting | 0 mismatches, 0 stuck running, 0 lost acknowledged jobs | 10/10 runs |
| `/metrics` scrape cost | < 250 ms at 270k job rows | measured 60 ms with auto_explain on |
| Claim candidate query | < 5 ms at ≥ 100k queued | measured 0.40 ms at 120k queued |

## Query plans at scale (RV01-15 re-measure)

`TestExplainAtScale` (`apps/control-plane/internal/store/explain_scale_test.go`, opt-in `CLUSTERX_EXPLAIN_SCALE=1`) seeds
**120,000 queued** jobs (4 handlers, workers support 2), **150,000 terminal** jobs, 270,000 events and 150,000 idempotency
rows, then enables `auto_explain` (`log_analyze`, `log_buffers`, `log_min_duration=0`) for new sessions and runs the
**production code paths**. Native PostgreSQL 18.7 on the macOS host (not the VM). Raw plans:
`evidence/m8/explain/plans-auto-explain.txt`; test output: `evidence/m8/explain/test-output.txt`.

| Path | Plan (key node) | Execution |
|---|---|---|
| Claim candidates (per-capability top-N, `FOR UPDATE SKIP LOCKED`, LIMIT 8) | Nested loop over `worker_capabilities_pkey` → `Index Scan using jobs_claimable_idx` + `LockRows`, 8 rows per capability, top-level sort of 16 rows | **0.40 ms**, 71 buffers |
| Claim CAS `UPDATE jobs … WHERE id` | `jobs_pkey` | 0.16–3.4 ms (first includes cold pages) |
| `GET /jobs` default page (50) | `Index Scan using jobs_created_idx`, 51 rows | 0.59 ms |
| `GET /jobs?status=queued&handler_name=sleep_ms` | `jobs_created_idx` with filter | 0.20 ms |
| Idempotency lookup (replay, different body ⇒ 409) | `Index Scan using submission_idempotency_pkey` | 0.008 ms |
| Stream feed poll (snapshot cursor) | `Index Scan using job_events_txid_idx` (`txid >= xmin`) | 0.005 ms |
| `/metrics` ops snapshot (one scrape) | status counts: 31.5 ms; unschedulable/oldest-queued CTE: 25.0 ms | 60 ms total |
| 19 concurrent claimants × 10 claims × 8 (auto_explain on) | — | p50 18.3 ms, p95 59.2 ms, p99 63.8 ms per claim transaction |

**RV01-15 disposition: closed by measurement.** The claim no longer scans and sorts the whole queue (M1: bitmap scan of
every queued row + sort, 6.6 ms at 20k); it reads 8 index entries per supported capability. The `/metrics` snapshot is
the only path that is linear in table size (counts by status); at 270k rows it costs 60 ms per 15 s scrape.
