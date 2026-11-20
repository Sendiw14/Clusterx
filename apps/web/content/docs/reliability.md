---
title: Reliability
lead: What CLUSTERX guarantees, what it does not, how recovery is timed, and how much of each claim has actually been tested.
description: Guarantees and non-guarantees, retry budgets, recovery timelines, side-effect caveats, failure modes and test evidence for CLUSTERX.
blurb: Guarantees and non-guarantees, retry budgets, recovery timing, side-effect caveats and the test evidence behind each claim.
order: 5
---

This page separates three things that are easy to blur: what the design **promises**, what a test **has shown**, and what is **not yet verified**. Where the tests have not run, it says so.

## Guarantees

- **Acknowledged means committed.** `POST /api/v1/jobs` answers 2xx only after the transaction that holds the job, its `admitted` event and any idempotency record has committed. If the database is unavailable the answer is 503, never a premature success. If the connection fails during the commit itself, the outcome is unknown, so retry with the same `Idempotency-Key`.
- **At-least-once execution.** A job whose attempt budget is not exhausted keeps being offered to workers until it reaches a terminal state.
- **No stale write.** A result, failure or renewal from a stale attempt, lease generation, worker incarnation or revoked credential is rejected and never changes the job. Terminal rows are immutable while they are retained.
- **No double lease.** Concurrent claimers never take the same job; one active attempt exists per running job.
- **A bounded attempt budget.** `max_attempts` (1 to 5, default 3, includes the first run) is never exceeded.
- **One order per job.** Each job's events carry a `version` that is the authoritative order of what happened to it.

## Non-guarantees

- **No exactly-once execution.** A handler can run twice. Fencing protects the database record, not the outside world.
- **No unrunning a handler.** A worker that lost its lease may still be running and may already have acted.
- **No strict FIFO and no fairness.** Claiming locks rows with `SKIP LOCKED` and orders by priority, then time. Sustained high-priority load can starve low priority.
- **No high availability.** There is one control plane. While it restarts, nothing is claimed or reported, and leases may expire.
- **No killing of a handler.** A deadline or cancellation stops the engine from accepting a result; it does not stop a goroutine that ignores its context.
- **No unlimited retention.** Results (128 KiB at most) and attempt log tails (16 KiB at most) are size-limited. Log tails and terminal jobs are removed by a **retention pass**: log tails 7 days after an attempt finishes, terminal jobs and their history 30 days after they finish. These are proposed values, covered by tests (`TestNF03`, `TestNF04`). **Whether the pass runs depends on how you start the system.** The control plane's retention interval is off unless you set it. The production example sets it to one pass an hour, and a local proof of that example aged a job to 31 days, ran a pass and saw the job deleted (its API then answers 410). The local Compose stack runs nothing by default: it has an optional one-shot `retention` service that you run from cron. Without a scheduled pass, terminal jobs and history grow without bound. The 7 and 30 day periods are proposals.
- **No exactly-once at the API either.** Idempotency keys are remembered for 24 hours only.
- **No promised performance numbers.** Measured numbers from one development setup are at the end of this page; they are not capacity claims.

## Retry budgets

An attempt is consumed when it is **claimed**, so a crash, a timeout and a lease expiry each use one. The budget includes the first run.

| Situation | Outcome |
| --- | --- |
| Retryable failure, budget left | `retry_wait`, then `queued` after backoff |
| Retryable failure, budget used up | `failed`, error code `attempts_exhausted`, class `retryable_exhausted` |
| Permanent failure | `failed`, class `permanent`, no further attempts |
| Lease expired or deadline passed, budget left | `retry_wait` |
| Lease expired, budget used up | `failed`, class `lease_expired` (but see the deadline rule below) |
| Deadline passed, budget used up | `failed`, class `timeout`. If the lease and the deadline have both passed, the deadline wins |

Backoff is `min(cap, base x 2^(n-1))` with full jitter, base 2 s and cap 5 minutes by default.

## Recovery timeline

The control plane's scheduler runs about once a second. By design no job stays `running` longer than **the lease TTL after its last renewal (or its hard deadline, whichever is sooner) plus one scheduler tick**. With the defaults (30 s lease), a worker that vanishes right after renewing is recovered after at most about 31 seconds from that renewal, then the job waits for its backoff before another claim.

```text expected
t+0 s    worker A claims the job; the lease runs to t+30 s
t+12 s   worker A renews; the lease now runs to t+42 s
t+14 s   worker A crashes or is cut off; no more renewals
t+42 s   the lease expires on the database clock
t+43 s   the next scheduler tick closes attempt 1 as lease_expired; the job goes to retry_wait
later    after its backoff the job is queued and worker B claims attempt 2
```

> [!NOTE] This timeline is derived from the specification and the default timings. It was also observed with real worker kills: in the M8 benchmark a SIGKILLed worker held 4 attempts and the slowest affected job finished 34.1 s after submission, and an independent audit saw 4 attempts closed 28 s after a SIGKILL of a container worker, all re-run to success. Both ran on arm64 Docker VMs. Treat 30 s plus a tick as the design bound, not a guarantee for other hardware.

> [!NOTE] **Deadline before lease.** If the hard deadline and the lease have both passed, the attempt is recorded as `deadline_exceeded`. When a handler's timeout is not longer than the lease TTL (`hash_sha256` has 30 s, the same as the default lease), the lease is set to expire at the deadline, so a worker crash is recorded as `deadline_exceeded` and, with no budget left, as class `timeout`, not `lease_expired`. The retry outcome is the same; only the label differs. Read `timeout` on such short-timeout jobs as "crash or overrun". The `ClusterxLeaseExpiriesRepeated` alert counts both outcomes. This is the rule in the state machine (event E8).

## Side-effect caveats

Because delivery is at-least-once, any effect your handler has outside the database can happen twice:

- Between lease expiry and the old worker noticing, two attempts can overlap.
- A handler that succeeded but whose success report was fenced will run again.
- Use the job `id` as an idempotency key for writes, emails, payments and webhooks. The id is the same on every attempt.
- Do not assume that a canceled job stopped doing work immediately.

## Failure modes

| Failure | What happens | Verified |
| --- | --- | --- |
| A worker crashes | Lease expires, job is retried if budget remains | In-process test, a `kill -9` test, a SIGKILLed worker container in the M8 benchmark (all attempts recovered) and an independent audit drill (28 s after the kill, 12 of 12 jobs succeeded). Local, arm64 |
| A worker is partitioned | Its reports are rejected after expiry; its handler may keep running | Fence denials tested for every reason |
| A worker is revoked | New claims, renewals and reports are refused | Tested, including revocation racing an in-flight report |
| The control plane restarts | Unavailable while down; nothing uncommitted was acknowledged; recovery on start | A job survived a SIGTERM and restart in the M4 curl guide, and a delayed job survived `docker compose restart control-plane` in the audit drill |
| PostgreSQL is unavailable | Readiness fails, writes answer 503, claims pause | A database-outage chaos test, and the M8 alert drill stopped PostgreSQL and saw admissions answer 503. Local only |
| A handler ignores cancellation | The result is not accepted; the goroutine keeps running | Documented limit |
| A result is too large | Job fails permanently with `result_too_large` | Specified; tested at the SDK level |

## What has been tested

The backend lane ran these against a real PostgreSQL 18 (HANDOFF-M4-BE-01). They are correctness tests on one machine, not capacity tests.

| Claim | Evidence | Status |
| --- | --- | --- |
| Acknowledge only after commit | Failure injected before commit leaves zero rows; replay after a simulated restart works | Tested |
| One claimant wins a job | 24 concurrent claimants over 5 rounds; 24 gRPC workers over 200 jobs, each leased once | Tested |
| Budget is exact | A job with three attempts never gets a fourth | Tested |
| Cancel versus success | Both orders and 30 concurrent races | Tested |
| Idempotency | 10 concurrent replays, conflicts, other principals, expiry | Tested |
| Late and stale reports | Every denial reason in the fence | Tested |
| 1,000 jobs, 3 workers | In-process run | Tested |
| Real process kill (`kill -9`) | A process-level test, and a SIGKILL of a worker container during a benchmark and an audit drill | Tested (local) |
| Three worker processes under Compose, 1,000 jobs | The M8 benchmark: 10 runs, exact reconciliation (900 succeeded, 100 failed, attempts equal the sum of attempts used) | Run (local, arm64 VM) |
| Database outage mid-run | A chaos test: admissions answer 503 while the database is down and never acknowledge without a commit | Tested (local) |
| Backup and restore | A drill on the compose stack: restore into a new database, running attempts recovered as `lease_expired` | Drilled (local) |
| Reference Linux amd64 host | | **Not run** |
| The same suites in GitHub Actions | The workflows are defined but have never run | **Not run** |

The 1,000-job result, as recorded: `submitted=1000 terminal=1000 succeeded=1000 attempts=1250 retries=250 active_attempts=0`, with the three workers taking 411, 417 and 422 attempts, on a laptop with a 10 ms backoff base. The handlers there fail on purpose to produce retries. **This is a correctness result and not a throughput claim.**

## Measured numbers

Numbers exist from one development setup and are not capacity claims. Hardware: Apple M4 (10 cores, 16 GiB), Colima VM aarch64 with 4 vCPU and 6 GiB, PostgreSQL 18.7, one control plane, workers with capacity 4. Workload: 1,000 jobs (a mix of hashing, 100 ms sleeps and planned failures), three repetitions each. The raw output and the full tables are in `docs/testing/benchmarks.md` in the repository.

| Workers | Throughput (jobs/s, range of 3 runs) | End-to-end p95 | Admission p99 |
| --- | --- | --- | --- |
| 1 | 114.7 to 129.9 | 6.0 to 6.2 s | 11.5 to 14.8 ms |
| 3 | 199.2 to 233.9 | 2.9 to 3.5 s | 9.2 to 14.9 ms |
| 5 | 242.3 to 280.5 | 2.2 to 2.7 s | 9.3 to 15.7 ms |

All 1,000 jobs are admitted within about half a second, so end-to-end time is mostly queue wait, and the retry backoff of 2 s bounds the short runs. Scaling is not linear and the data does not support extrapolation. The proposed service objectives in that report (for example admission p99 under 100 ms) are **proposals and are not frozen**; no recovery point or recovery time objective has been set. A reference amd64 host has not been measured.
