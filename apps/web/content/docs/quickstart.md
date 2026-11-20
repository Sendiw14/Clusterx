---
title: Quickstart
lead: Start the local stack, submit a first job and read its attempts back. Every command here is marked with what was actually run.
description: Local Docker Compose quickstart for CLUSTERX with the verification status of every step.
blurb: Clean checkout to a first job and a three-worker local stack, with what was verified and what was not.
order: 1
---

This quickstart starts a **local development stack**: PostgreSQL, the Go control plane and three example workers. It is not a production deployment, and the web console is not part of this compose stack: the console needs a GitHub OAuth app, so the clean-checkout path works with the REST API through `curl`. To see the console without GitHub, read [Try the console without GitHub](#try-the-console-without-github) at the end.

> [!VERIFIED] On 2026-11-18 the integrator ran this flow on Colima: `./infra/compose/bootstrap.sh` exited 0 and left 3 live workers, and `POST /api/v1/jobs` with the `hash_sha256` and `fail_n_times` handlers succeeded.

> [!VERIFIED] On 2026-11-20 an independent auditor repeated this flow from a clean `git clone` **inside a Linux arm64 Ubuntu VM**: `bootstrap.sh` exited 0 and a job ran to `succeeded`. The auditor also killed a worker container there and watched the attempts recover (step 8). On macOS the same clean clone, made outside `$HOME`, **failed** with a bind-mount error. A clean clone on macOS under `$HOME` was **not** run by the audit; the only macOS evidence is the integrator's run above, made from a working checkout.

> [!UNVERIFIED] Not run: a Linux amd64 host, and anything involving real GitHub sign-in. The commands below were run on arm64 only.

## Prerequisites

- Docker with the Compose plugin (Compose V2). Run on Colima on macOS (the integrator's working checkout) and, from a clean clone, in a Linux arm64 VM. With Colima, keep the checkout under `$HOME`: Colima mounts only `$HOME` into its VM, and a clean clone outside it (under `/private/tmp`) failed with a bind-mount error in the audit.
- A shell with `git`, `cp`, `curl` and `openssl`.
- Free loopback ports: 8080 (REST API), 9091 (metrics) and 5432 (PostgreSQL). Change them with `CLUSTERX_HTTP_PORT`, `CLUSTERX_METRICS_PORT` and `CLUSTERX_PG_PORT` in `.env`. The compose file publishes them on 127.0.0.1 only.
- You do not need Go installed: the images are built by Compose.

## 1. Get the code and set three secrets

```sh
git clone <repository-url> clusterx
cd clusterx
cp .env.example .env
# fill the three empty values in .env, for example with: openssl rand -hex 24
```

Open `.env` and set `POSTGRES_PASSWORD`, `CLUSTERX_APP_DB_PASSWORD` and `CLUSTERX_METRICS_TOKEN`. The compose file refuses to start if any of them is empty. Never commit a filled `.env`. The optional keys `CLUSTERX_HTTP_PORT`, `CLUSTERX_METRICS_PORT` and `CLUSTERX_IMAGE_PREFIX` (image name prefix; set `COMPOSE_PROJECT_NAME` too to run a second, separate stack) can stay at their defaults.

`<repository-url>` is a placeholder: use the address of the repository you cloned from.

## 2. Bootstrap the stack

```sh
./infra/compose/bootstrap.sh
```

The script, read from `infra/compose/bootstrap.sh`:

1. Writes the compose secrets as files under `infra/compose/.secrets/` (gitignored, directory mode 0700): the database passwords and URLs, the metrics token and a generated password for the retention login. Containers read them as files; the passwords are not in their environment.
2. On the first run, generates a **development-only** key pair for the web server's service assertions (`.secrets/bff-dev-private.jwk.json`, mode 0600, and the public `bff-dev.jwks.json` that the control plane loads). Do not use it outside development.
3. Starts PostgreSQL and the control plane and waits until they are healthy. Migrations run as a separate one-shot service first.
4. On the first run, creates **one operator token and three worker credentials** with the control plane's own CLI and writes them, once, to `infra/compose/.secrets.env` (gitignored, mode 0600). No default credentials exist.
5. Starts three worker containers, `worker-1` to `worker-3`, each with its own credential.

```text expected
Ready: http://127.0.0.1:8080/health/ready — operator token is in infra/compose/.secrets.env
```

> [!VERIFIED] Exit status 0 and three live workers, integrator, 2026-11-18. The line above is the script's final message.

> [!NOTE] The database roles are created only when the PostgreSQL volume is first created, so a volume from a stack started before the retention login existed has no such login. The optional retention service (see [Deployment](/docs/deployment)) fails to authenticate there until the volume is recreated. The main services keep working.

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

Submit a dozen long jobs (the `sleep_ms` handler accepts up to 60,000 milliseconds, for example `{"ms":20000}`), wait until the workers hold some of them, then kill one worker container hard and read the jobs again:

```sh
docker compose -f infra/compose/compose.yaml --env-file .env kill -s SIGKILL worker-2
```

The lease stops being renewed and expires after the lease TTL (30 s by default). The attempts the killed worker held are closed and the jobs are retried on another worker as a new attempt. If you restart the stopped worker and it somehow reports for the old attempt, the report is rejected. At-least-once means a handler may have run twice.

> [!VERIFIED] The independent audit ran this with 12 `sleep_ms` jobs of 20 s on three workers. Killing `worker-2` left 4 active attempts; they were closed 28 s after the kill, re-claimed on another worker, and all 12 jobs ended `succeeded` 50 s later. No job was left `running` without an active attempt. Linux arm64 VM, 2026-11-20. The exact flags above are the README's compose invocation; the auditor's flags were not recorded verbatim.

> [!NOTE] When a job's timeout is not longer than the lease TTL, a crash is recorded as `deadline_exceeded` rather than `lease_expired` (the deadline wins). `hash_sha256` has a 30 s timeout, the same as the default lease. Both lead to a retry. See [Reliability](/docs/reliability).

What was also observed in the M4 curl guide, using host binaries: a graceful `SIGTERM` to both workers during a 20-second `sleep_ms` job made them drain, cancel the handler after the grace period and deregister; the attempt was recorded as `failed` with `worker_shutdown`; after the control plane was restarted a new worker completed the job on attempt 2.

## Stop the stack

```sh
docker compose -f infra/compose/compose.yaml --env-file .env --profile workers down
```

Add `-v` to delete the database volume. The auditor ran the `-v` form and the volume and network were removed.

> [!UNVERIFIED] Running more than three workers is not covered by this guide. Throughput measured on a small VM for one, three and five workers is on the [Reliability](/docs/reliability) page, with its limits.

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `missing ../../.env` | The script found no `.env`. Run `cp .env.example .env` from the repository root and fill the three values. |
| Compose reports a variable is "set in .env" | One of `POSTGRES_PASSWORD`, `CLUSTERX_APP_DB_PASSWORD` or `CLUSTERX_METRICS_TOKEN` is empty. |
| `secret creation failed` | The CLI could not create a token or credential. The script removes the partial `.secrets.env`; fix the cause shown in its output and run it again. |
| HTTP 401 `unauthenticated` | The token is missing or wrong. Re-copy `CLUSTERX_OPERATOR_TOKEN` from `infra/compose/.secrets.env`. |
| HTTP 403 `forbidden` | The token's role or scope does not allow the operation. Submitting jobs needs an operator token. |
| HTTP 422 `validation_failed` | The handler or payload is invalid. `details.fields` lists each field as a JSON Pointer. Unknown fields are rejected on purpose. |
| HTTP 404 `not_found_route` | The path is not one the control plane routes (a typo, or a path outside `/api/v1`). Compare it with the [API reference](/docs/api). |
| `invalid mount config … bind source path does not exist` (Colima) | The checkout is outside `$HOME`. Move it under `$HOME`. |
| Port already in use | Another process holds 8080, 9091 or 5432. Change `CLUSTERX_HTTP_PORT`, `CLUSTERX_METRICS_PORT` or `CLUSTERX_PG_PORT` in `.env`. |

Always quote the `request_id` from an error body when reporting a problem.

## Try the console without GitHub

The console signs people in with GitHub, and that needs an OAuth app that you register and that the owner has not set up (nothing in this repository fakes a login in production). There is therefore **no console in the clean-checkout path above**. The one way to see the console locally is the end-to-end test harness, which starts a throwaway stack with a **mock OAuth server** standing in for GitHub:

- The real Go control plane and worker built from your checkout, two workers, and a private PostgreSQL 18 cluster on port 55440. Control plane on 18180 (gRPC 19190, metrics 19191), mock OAuth on 19300, web server on 3100. It does not touch the compose stack above.
- Needs Node 26 with pnpm 9.15.9, Go 1.27.4, PostgreSQL 18 binaries (set `CLUSTERX_E2E_PG_BIN` if they are not at `/opt/homebrew/opt/postgresql@18/bin`) and Playwright's browsers (`pnpm --filter web exec playwright install chromium`).

```sh
pnpm install
pnpm --filter web build
CLUSTERX_E2E_PG_DIR=/some/empty/dir pnpm --filter web e2e
```

The users are fixed test identities (an operator, a viewer, and others that exercise denial). The harness runs the specs that drive the console in a browser: jobs, workers, tokens, live updates, errors and permissions. Then it **stops everything it started**. It is not an interactive environment you can browse afterwards. Playwright's `--headed` and `--ui` options show the browser while a spec runs; they are standard Playwright flags and this guide did not exercise them.

> [!VERIFIED] The full suite passes against this harness in Chromium (211 passed, 54 skipped screenshot specs, 2026-11-20 audit run). See the changelog for the Firefox and WebKit runs.

> [!UNVERIFIED] Everything here uses mock sign-in on plain HTTP on loopback. A real GitHub sign-in, TLS and production-mode cookies in a browser have not been run. The public [demo](/demo) shows the console's screens on fixture data and needs no setup.

## Next

Read [Concepts](/docs/concepts) for the vocabulary, [Worker SDK](/docs/workers) to write a handler, and [Reliability](/docs/reliability) for what is and is not guaranteed.
