---
title: Concepts
lead: The vocabulary exactly as the engine uses it: states, attempts, leases, fencing, priorities and what at-least-once means.
description: Job states, attempts versus jobs, leases, fencing, at-least-once delivery, due jobs, priorities, cancellation and idempotency in CLUSTERX.
blurb: Job states, attempts versus jobs, leases, fencing, at-least-once, due jobs, priorities, cancellation and idempotency.
order: 2
---

The normative source is `docs/architecture/state-machine.md`; this page restates it in plain terms. If the two ever disagree, the state machine document wins and this page has a bug.

## Job states

A job has exactly seven states in 1.0. Three are terminal and immutable while the job is retained.

| State | Terminal | Meaning |
| --- | --- | --- |
| `scheduled` | no | Admitted with a `run_after` in the future; waits to become due. |
| `queued` | no | Eligible to be claimed by a worker. |
| `running` | no | Exactly one active attempt holds the current lease. |
| `retry_wait` | no | The last attempt failed in a retryable way or its lease expired; waits for its backoff to pass. |
| `succeeded` | yes | A result was committed. |
| `failed` | yes | A permanent failure, or the attempt budget is used up. |
| `canceled` | yes | Canceled before or during execution. |

### Reasons and views, not states

- **Unschedulable** is a reason shown beside a queued job when no worker can take it. It is computed when you read the job: `no_capable_worker`, `capable_workers_offline` or `capable_workers_draining`. A job whose capable workers are all busy is not unschedulable.
- **Dead-letter** is a filtered view of jobs whose status is `failed`. It is not a state.
- `blocked` and `skipped` are reserved for a later release and are rejected in 1.0.

## Jobs and attempts

A **job** is the thing you submit. An **attempt** is one execution of it by one worker. Claiming a job creates an attempt, increments the job's `attempts_used`, and starts a lease, all in one transaction. A job can therefore have several attempts, each with its own worker, outcome and error.

`max_attempts` is between 1 and 5 (default 3) and **includes the first run**. A timeout, a worker crash and a lease expiry each consume an attempt. When the budget is used up the job becomes `failed` with the error code `attempts_exhausted`. There is no hidden extra attempt.

Retries wait with exponential backoff and full jitter: base 2 seconds, capped at 5 minutes by default.

## Leases and heartbeats

A **lease** is the time-limited right to keep running an attempt. A claim sets `lease_expires_at` and increments the job's `lease_generation`. The worker must **renew** it while the handler runs, or the lease expires. If the hard deadline and the lease have both passed, the attempt is recorded as `deadline_exceeded`, not `lease_expired` (the deadline wins). Because a claim caps the lease at the deadline, a worker crash on a job whose timeout is not longer than the lease TTL is recorded as `deadline_exceeded`.

Two signals are easy to confuse:

| | Heartbeat | Lease renewal |
| --- | --- | --- |
| Scope | The worker process | One attempt |
| Writes | `last_heartbeat_at` on the worker | `lease_expires_at` on the job |
| Purpose | Liveness and capacity display, drain | Authority to keep running and to report |
| Extends a lease? | Never | Yes, capped by the hard deadline |

All lease and deadline decisions use the **database clock**. A worker's clock is never trusted; it receives a relative TTL. The default lease TTL is 30 seconds. See [Architecture](/architecture) for the other defaults.

## Fencing

Fencing is how a late report from a worker that has lost its lease is stopped. A result, a failure report or a lease renewal is accepted only if, at the moment it is judged:

- the job is `running` and its current attempt and lease generation match the ones in the call;
- the lease and the hard deadline have not passed;
- the attempt is active and belongs to this worker and this worker incarnation;
- the worker has not been deregistered and its credential is not revoked or expired.

Anything else is rejected with a specific reason such as `stale_attempt`, `lease_expired`, `incarnation_mismatch` or `credential_revoked`, and a bounded diagnostic record is kept. The job's state is not changed by a rejected report.

> [!WARNING] Fencing protects what the **database records**. It cannot unrun a handler. A worker that lost its lease may still be running and may already have caused side effects elsewhere.

## At-least-once delivery

Every job is executed **at least once**, never exactly once. A handler can run more than once: after a crash, after a lease expiry while the handler was still alive, or after a retryable failure. Two attempts of the same job can overlap briefly in the outside world.

Make external side effects idempotent. The job's `id` is stable across attempts, so it makes a good idempotency key for writes you perform in a handler.

## Due jobs and priority

A job submitted with a future `run_after` (at most 30 days ahead) starts `scheduled` and is promoted to `queued` when it is due. Priority is an integer from 0 to 9 and **9 is the highest**. Eligible jobs are claimed in the order priority (highest first), time they became available, creation time and id.

Claiming locks rows with `SKIP LOCKED` so concurrent workers never take the same job. The consequence is that the order is **not strict FIFO**, and sustained high-priority load can keep low-priority jobs waiting. That is monitored, not prevented.

## Cancellation

Cancellation is **cooperative**.

- A `scheduled`, `queued` or `retry_wait` job is canceled immediately.
- A `running` job gets a cancel request. The lease is capped at the cancel grace period (15 seconds by default) from the request, and the job becomes `canceled` when the worker acknowledges or the lease expires.
- A job that already reached `succeeded` or `failed` cannot be canceled: the request is a conflict.

If a cancel request and a success report race, **whichever commits first wins**. If the cancel committed first, the worker's success is not recorded: the job ends `canceled`, and the event notes that the worker had reported success. A success that committed first is never rolled back.

A deadline or a cancel request does not stop a handler that ignores its context. It stops the engine from accepting that handler's result.

> [!NOTE] The cancel and manual-retry operations are in the API contract but are not served in the current build. See the [API reference](/docs/api).

## Idempotent submission

Send an `Idempotency-Key` header (1 to 128 characters from `A-Za-z0-9_.:-`) with `POST /api/v1/jobs`. The key is scoped to your principal and kept for 24 hours.

- Same key, same request: you get the original job back with HTTP 200 and `Idempotent-Replayed: true`.
- Same key, different request: HTTP 409 `idempotency_conflict`, with the original job's id in `details`.

"The same request" is decided by a fingerprint of the handler, version, payload, priority, `run_after`, attempts and timeout, computed over canonical JSON so key order does not matter.

## Manual retry

A terminal `failed` job can be retried by creating a **new linked job** with a fresh attempt budget. The failed job is never changed. Only one child can exist per failed parent.

## Why a job failed: `failure_class`

When a job ends in `failed` or `canceled`, `failure_class` says why, and `error_code` carries the handler or system code.

| Ended because | `failure_class` |
| --- | --- |
| Retryable failures used up the budget | `retryable_exhausted` |
| The handler reported a permanent failure | `permanent` |
| The hard deadline passed with no budget left (also a crash when the timeout is not longer than the lease TTL) | `timeout` |
| The lease expired with no budget left, deadline not yet passed | `lease_expired` |
| Canceled | `canceled_by_operator` |

## Events

Each state change appends one event with a per-job `version`. The event types are `admitted`, `promoted`, `claimed`, `succeeded`, `failed_retryable`, `failed_permanent`, `attempts_exhausted`, `lease_expired`, `deadline_exceeded`, `cancel_requested`, `canceled` and `retry_created`. Because the job also carries its `version`, a client can take a snapshot and fetch only the events after it to reconcile.
