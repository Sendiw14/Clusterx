---
title: Quickstart
lead: Start the local stack, submit a first job and read its attempts back. Every command here is marked with what was actually run.
description: Local Docker Compose quickstart for CLUSTERX with the verification status of every step.
blurb: Clean checkout to a first job and a three-worker local stack, with what was verified and what was not.
order: 1
---

This quickstart starts a **local development stack**: PostgreSQL, the Go control plane and three example workers. It is not a production deployment, and it does not include the web console yet. You work with the REST API through `curl`.

> [!VERIFIED] On 2026-11-18 the integrator ran this flow on Colima: `./infra/compose/bootstrap.sh` exited 0 and left 3 live workers, and `POST /api/v1/jobs` with the `hash_sha256` and `fail_n_times` handlers succeeded.

> [!UNVERIFIED] A run from a fresh clone by someone other than the integrator, a run on a Linux host, and stopping a worker container to watch recovery have not been done. Where a step below depends on one of those, it says so.

## Prerequisites

- Docker with the Compose plugin (Compose V2). The verified run used Colima on macOS.
- A shell with `git`, `cp`, `curl` and `openssl`.
- Free loopback ports: 8080 (REST API), 9091 (metrics) and 5432 (PostgreSQL, changeable with `CLUSTERX_PG_PORT` in `.env`). The compose file publishes them on 127.0.0.1 only.
- You do not need Go installed: the images are built by Compose.

## 1. Get the code and set three secrets

```sh
git clone <repository-url> clusterx
cd clusterx
cp .env.example .env
# fill the three empty values in .env, for example with: openssl rand -hex 24
```

Open `.env` and set `POSTGRES_PASSWORD`, `CLUSTERX_APP_DB_PASSWORD` and `CLUSTERX_METRICS_TOKEN`. The compose file refuses to start if any of them is empty. Never commit a filled `.env`.

`<repository-url>` is a placeholder: use the address of the repository you cloned from.

## 2. Bootstrap the stack

```sh
./infra/compose/bootstrap.sh
```

The script, read from `infra/compose/bootstrap.sh`:

1. Starts PostgreSQL and the control plane and waits until they are healthy. Migrations run as a separate one-shot service first.
2. On the first run, creates **one operator token and three worker credentials** with the control plane's own CLI and writes them, once, to `infra/compose/.secrets.env` (gitignored, mode 0600). No default credentials exist.
3. Starts three worker containers, `worker-1` to `worker-3`, each with its own credential.

```text expected
Ready: http://127.0.0.1:8080/health/ready — operator token is in infra/compose/.secrets.env
```

> [!VERIFIED] Exit status 0 and three live workers, integrator, 2026-11-18. The line above is the script's final message.

> [!NOTE] The workers talk to the control plane in plaintext (`CLUSTERX_WORKER_INSECURE=true`) because they share a private compose network. That is acceptable for local development and is not a production setting.

## 3. Check that it is up

```sh
curl http://127.0.0.1:8080/health/live
curl http://127.0.0.1:8080/health/ready
```

```json expected
{"status":"ok"}
{"status":"ready"}
```

> [!VERIFIED] These two calls were executed in the M4 curl guide against the built binaries (port 18080 there). Port 8080 is the port the compose file publishes; the same calls on 8080 are the same endpoints but the exact compose run was not recorded.

## 4. Submit a first job

Open `infra/compose/.secrets.env`, copy the value of `CLUSTERX_OPERATOR_TOKEN`, and put it in your shell:

```sh
export CLUSTERX_TOKEN=<value of CLUSTERX_OPERATOR_TOKEN in infra/compose/.secrets.env>

curl -X POST http://127.0.0.1:8080/api/v1/jobs \
  -H "Authorization: Bearer $CLUSTERX_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: guide-1" \
  -d '{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"hello"},"priority":5}'
```

A job is acknowledged with HTTP 201 only after it is committed to PostgreSQL. The body contains the job with `"status":"queued"` and an `id`; a `Location` header points to it.

> [!VERIFIED] Status 201 with `status: queued` and a `Location` header is what the M4 curl guide recorded for this exact request. Job submission with `hash_sha256` and with `fail_n_times` succeeding on the compose stack was confirmed by the integrator.

Send the same request again with the same `Idempotency-Key`. You get HTTP 200, an `Idempotent-Replayed: true` header and the same job. Send it with the same key but different data and you get HTTP 409 with code `idempotency_conflict`. Keys are remembered for 24 hours. See the [API reference](/docs/api).

## 5. Read the job back

Replace `<job-id>` with the `id` from the previous response.

```sh
curl -H "Authorization: Bearer $CLUSTERX_TOKEN" http://127.0.0.1:8080/api/v1/jobs/<job-id>
curl -H "Authorization: Bearer $CLUSTERX_TOKEN" http://127.0.0.1:8080/api/v1/jobs/<job-id>/attempts
curl -H "Authorization: Bearer $CLUSTERX_TOKEN" "http://127.0.0.1:8080/api/v1/jobs/<job-id>/events?after_version=0"
```

Within a moment a `hash_sha256` job shows `status` `succeeded` and `attempts_used` 1, and the result holds the SHA-256 of your data.

> [!VERIFIED] In the M4 curl guide the job reached `succeeded` with `attempts_used` 1, and the attempts and events calls returned the shapes described here.

## 6. See a retry

The `fail_n_times` handler fails on purpose, so you can watch the retry path.

```sh
curl -X POST http://127.0.0.1:8080/api/v1/jobs \
  -H "Authorization: Bearer $CLUSTERX_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"handler_name":"fail_n_times","handler_version":1,"payload":{"n":1}}'
```

Read its attempts. The first attempt is `failed` with error code `planned_failure` and is marked retryable; after a short backoff a second attempt succeeds. The events show `admitted`, `claimed`, `failed_retryable`, `promoted`, `claimed`, `succeeded`.

> [!VERIFIED] The attempts and events described here were recorded in the M4 curl guide for a `fail_n_times` job with `n` set to 1. The default attempt budget is 3, including the first run.

## 7. Look at the workers

```sh
curl -H "Authorization: Bearer $CLUSTERX_TOKEN" "http://127.0.0.1:8080/api/v1/workers?liveness=live"
```

You should see three live workers, named `worker-1`, `worker-2` and `worker-3`. A viewer-role token would see their hostnames masked.

> [!VERIFIED] Three live workers after bootstrap, integrator, 2026-11-18. The `liveness=live` filter was exercised in the M4 curl guide against two workers.

## 8. Recovery drill

> [!UNVERIFIED] The following drill has not been run against the compose stack. It describes what the state machine specifies, not something observed.

Submit a long job (the `sleep_ms` handler accepts up to 60,000 milliseconds), stop the worker container that took it, and read the job again. By the specification the lease stops being renewed, expires after the lease TTL (30 s by default), the attempt is recorded as `lease_expired`, the job returns to the queue after backoff and another worker claims it as attempt 2. If you restart the stopped worker later and it somehow reports for attempt 1, the report is rejected. At-least-once means the handler may have run twice.

What *was* observed, in the M4 curl guide, using host binaries rather than containers: a graceful `SIGTERM` to both workers during a 20-second `sleep_ms` job made them drain, cancel the handler after the grace period and deregister; the attempt was recorded as `failed` with `worker_shutdown`; after the control plane was restarted a new worker completed the job on attempt 2.

## Scale workers and stop the stack

> [!UNVERIFIED] Adding workers beyond three, measuring throughput, and the commands to stop and clean the stack have not been verified, so none are given here. No throughput numbers exist.

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `missing ../../.env` | The script found no `.env`. Run `cp .env.example .env` from the repository root and fill the three values. |
| Compose reports a variable is "set in .env" | One of `POSTGRES_PASSWORD`, `CLUSTERX_APP_DB_PASSWORD` or `CLUSTERX_METRICS_TOKEN` is empty. |
| `secret creation failed` | The CLI could not create a token or credential. The script removes the partial `.secrets.env`; fix the cause shown in its output and run it again. |
| HTTP 401 `unauthenticated` | The token is missing or wrong. Re-copy `CLUSTERX_OPERATOR_TOKEN` from `infra/compose/.secrets.env`. |
| HTTP 403 `forbidden` | The token's role or scope does not allow the operation. Submitting jobs needs an operator token. |
| HTTP 422 `validation_failed` | The handler or payload is invalid. `details.fields` lists each field as a JSON Pointer. Unknown fields are rejected on purpose. |
| HTTP 404 `not_found_route` | The route is not served in this build. See the status column in the [API reference](/docs/api). |
| Port already in use | Another process holds 8080, 9091 or 5432. Change `CLUSTERX_PG_PORT` for PostgreSQL; the other ports are fixed in the compose file. |

Always quote the `request_id` from an error body when reporting a problem.

## Next

Read [Concepts](/docs/concepts) for the vocabulary, [Worker SDK](/docs/workers) to write a handler, and [Reliability](/docs/reliability) for what is and is not guaranteed.
