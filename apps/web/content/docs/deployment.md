---
title: Deployment
lead: What the local Compose stack contains and how it is hardened, and the single-VM production example that was proven only on one local machine.
description: CLUSTERX local Docker Compose reference, database roles and secrets, backup, restore and alert drills, and a production example verified locally only.
blurb: Local Compose stack, database roles, secrets and configuration, plus a production example that has only been proven locally.
order: 6
---

> [!UNVERIFIED] **No production host exists, and production is not yet verified.** The production example, the backup and restore drill, the alert drills and the TLS setup were run on one developer machine (macOS with a Colima VM, arm64), with a self-signed certificate authority. A reference Linux amd64 host, a public domain and certificate, and real GitHub sign-in were **not run**. Nothing on this page is a measured capacity or a recovery guarantee.

## Local Compose reference (verified)

`infra/compose/compose.yaml` is marked in its own header as **development only, not a production example**. Start it with the steps in the [Quickstart](/docs/quickstart). The integrator ran it on Colima on 2026-11-18, and the independent audit reproduced it from a clean clone on macOS under `$HOME` and on Linux arm64 (an Ubuntu VM). amd64 has not been tried. With Colima, keep the checkout under `$HOME`: Colima mounts only `$HOME` into its VM.

| Service | Role | Published port |
| --- | --- | --- |
| `postgres` | PostgreSQL 18.7, image pinned by digest, data in the named volume `pgdata` | 127.0.0.1:5432 (change with `CLUSTERX_PG_PORT`) |
| `migrate` | One-shot: applies the database migrations as the schema owner, then exits | none |
| `control-plane` | REST on 8080, gRPC on 9090 (internal), metrics on 9091 | 127.0.0.1:8080 and 127.0.0.1:9091 (change with `CLUSTERX_HTTP_PORT`, `CLUSTERX_METRICS_PORT`) |
| `worker`, `worker-2`, `worker-3` | The reference worker running the three example handlers, one credential each, capacity 4 | none |
| `retention` | Optional, profile `retention`: one bounded retention pass as the `clusterx_retention_job` login, then exits. Run it from cron: `docker compose --env-file ../../.env --profile retention run --rm retention` | none |

Every published port is bound to loopback. The web console is **not** part of this stack: it needs a GitHub OAuth app. To try the console locally without GitHub, the repository's end-to-end harness starts the Go control plane, PostgreSQL, workers and a mock sign-in server. See the [Quickstart](/docs/quickstart).

Container hardening already present in the compose file: read-only root filesystems, all Linux capabilities dropped, `no-new-privileges`, and for the workers a process limit of 128 and a memory limit of 256 MB.

> [!NOTE] Workers connect with `CLUSTERX_WORKER_INSECURE=true` (plaintext) because they share a private Compose network. That setting is for local development only.

## Configuration and secrets

Copy `.env.example` to `.env` and fill the three empty values. `.env` must never be committed. `bootstrap.sh` turns these into **secrets files** under `infra/compose/.secrets/` (gitignored, directory mode 0700), which compose mounts into the containers; the passwords are not passed as environment variables. The control plane and worker also accept `*_FILE` variants for secret values (for example `CLUSTERX_DATABASE_URL_FILE` and `CLUSTERX_WORKER_CREDENTIAL_FILE`); setting both a value and its file is refused.

| Variable | Purpose |
| --- | --- |
| `POSTGRES_PASSWORD` | Password of the database owner account used for migrations |
| `CLUSTERX_APP_DB_PASSWORD` | Password of the least-privilege runtime login role the control plane uses |
| `CLUSTERX_METRICS_TOKEN` | Bearer token required to scrape `/metrics` on the internal listener |
| `CLUSTERX_PG_PORT` | Optional. Host port for PostgreSQL, default 5432 |
| `CLUSTERX_HTTP_PORT` | Optional. Host port for the REST API, default 8080 |
| `CLUSTERX_METRICS_PORT` | Optional. Host port for metrics, default 9091 |
| `CLUSTERX_IMAGE_PREFIX` | Optional. Image name prefix, default `clusterx`. Set it together with `COMPOSE_PROJECT_NAME` to run a second, separate stack |

`bootstrap.sh` also generates a **development-only** key pair for the web server's service assertions: the private key `.secrets/bff-dev-private.jwk.json` (mode 0600) for a local web server, and the public `bff-dev.jwks.json`, which the control plane loads so that console sessions work. Never use that pair outside development. The retention login's password is generated once and kept in `.secrets/`.

The operator token and the three worker credentials are **generated once** by `bootstrap.sh`, printed once by the control plane's CLI, and written to `infra/compose/.secrets.env` (gitignored, created with owner-only permissions). Only SHA-256 hashes are stored in the database. There are no default credentials.

The control plane reads more settings from `CLUSTERX_*` environment variables: the database URLs, heartbeat interval, liveness timeout, lease TTL, cancel grace, drain grace, scheduler tick and backoff base and cap. Each is validated at start-up against its allowed range and the process refuses to start on a value outside it. The defaults are listed on [Architecture](/architecture).

## The web server's configuration (built; not deployed anywhere)

The Next.js server fails closed: production is the default, and anything missing or unsafe makes it answer `upstream_auth` instead of limping along. All settings are `CLUSTERX_*` environment variables and none is sent to the browser.

| Variable | Purpose |
| --- | --- |
| `CLUSTERX_ENV` | `production` (default when unset), `development` or `test` |
| `CLUSTERX_APP_ORIGIN` | The console's exact origin. Must be `https` in production |
| `CLUSTERX_API_URL` | Control plane base URL. In production it must be `https`, except for a loopback address |
| `CLUSTERX_BFF_KEY_FILE` | Path to the Ed25519 private key (JWK with a key id) that signs service assertions. Read once at start |
| `CLUSTERX_WEB_SESSION_KEY` | Base64, at least 32 bytes. Seals cookies and derives the CSRF key. Treat it like the signing key: it can forge a cookie for any live session |
| `CLUSTERX_WEB_SESSION_KEY_PREVIOUS` | Optional. The old session key, accepted for reading during a rotation window and never used to seal |
| `CLUSTERX_GITHUB_CLIENT_ID`, `CLUSTERX_GITHUB_CLIENT_SECRET` | The GitHub OAuth application |

The server is started with `node server.ts` (it hosts the live-update WebSocket relay, which a plain `next start` cannot). OAuth endpoint overrides exist only outside production. The server reads these as plain variables; the production example's wrapper script `infra/production/web-entry.mjs` turns `CLUSTERX_*_FILE` secret files into them.

> [!UNVERIFIED] Production mode with real GitHub has not been run. The production example ran behind a local self-signed TLS proxy, with `/app` redirecting to `/login`; a real sign-in was not part of it.

## Database roles

The design separates two database identities:

- A **schema owner** that runs migrations (`CLUSTERX_MIGRATE_DATABASE_URL`).
- A **runtime role** (`CLUSTERX_DATABASE_URL`) that is a member of `clusterx_runtime`: it can read, insert and update, can delete only from a few expiring tables, and cannot truncate or change the schema. Delete and truncate guards on the history tables are enforced by database triggers.

In production mode the control plane **refuses to start** if its runtime role is a superuser or can create objects. In the M4 curl guide, starting it with a superuser runtime URL exited with status 1 and the message `runtime database role is superuser or may CREATE…`.

Migrations are forward-only, numbered files applied one transaction each, under an advisory lock. Readiness reports not ready until the applied set matches what the binary expects.

> [!VERIFIED] Migrations applying (`applied=2`), the superuser refusal and the least-privilege role were executed in the M4 curl guide. Deleting and truncating through the runtime role is covered by backend tests.

## Production example on a single VM (verified locally only)

`infra/production/` is a Compose example for one Linux host: Caddy terminates TLS and is the only service that publishes ports; the web server, control plane, three workers and PostgreSQL sit on internal networks. The full description and the evidence are in `docs/operations/deployment.md` and `docs/testing/evidence/m8/production/` in the repository.

> [!VERIFIED] On 2026-11-20 the example was started on a developer machine (macOS, Colima, arm64) with generated secrets and a self-signed certificate authority, then checked: TLS 1.3 on both host names, a job run by three workers over gRPC with TLS, the web server reaching the control plane over TLS, PostgreSQL not reachable from the web container, containers running as a non-root user with a read-only root filesystem and no Linux capabilities, no secret values in the environment, and the configuration refusing to start when secrets or TLS are missing (exit status 1).

> [!UNVERIFIED] Not run: a real host, an amd64 machine, public DNS and ACME certificates, a firewall, workers on a separate host, a rolling restart, real GitHub sign-in. The web image is 1.43 GB because it carries the whole build; a pruned image is not built.

| Topic | State | What exists |
| --- | --- | --- |
| TLS | Verified locally, self-signed CA | Caddy for browsers and API clients; TLS from the web server and from workers to the control plane (`CLUSTERX_WORKER_TLS_CA_FILE`) |
| Secrets | Verified locally | Files under `infra/production/secrets/` mounted as `/run/secrets/*`; generated per environment, never in the repository; rotate by creating a new credential and revoking the old one |
| Networking | Verified locally | Only Caddy publishes ports; PostgreSQL, gRPC and metrics stay on internal networks |
| Retention | Verified locally | The control plane runs one bounded retention pass an hour (`CLUSTERX_RETENTION_INTERVAL=1h`) as a separate database login. A local proof aged a job to 31 days, ran a pass and the job was deleted (API answer 410). Without that setting (the local Compose stack by default) nothing deletes anything and history grows without bound; its optional `retention` service is for cron. The 7 and 30 day periods are proposals |
| Backups | Drilled locally | `scripts/ops/backup.sh` writes a `pg_dump` archive with a checksum; on the drill stack it took 0.26 s. Keeping archives for 14 days is a proposal, not a tested policy |
| Restore | Drilled locally | `scripts/ops/restore.sh` restores into a **new** database. On the drill stack the restored control plane was ready 0.22 s after the restore; the 12 attempts that were running at backup time became `lease_expired` and were retried. Everything committed after the backup is lost |
| Upgrades | Migration drill run | Migrations 0002 to 0004 were applied to a populated database in a test. A rolling upgrade of a live deployment was not exercised. Rollback is a restore |
| Coordinator restarts | Downtime by design | One control plane, so claims and reports pause. A restart was exercised on the local stacks |
| Monitoring | Alerts drilled locally | Prometheus configuration and ten alert rules with an incident runbook. Nine were induced locally in M8; the node-exporter-down rule has a rule test only. **No Alertmanager is shipped**: nothing notifies anyone until you add routing |
| RPO and RTO | **Not frozen** | Measured only on the drill stack: a 2.2 s window of lost jobs, 14.1 s until the service answered again and 71.8 s until the re-run work drained. These are not objectives |

Hosting, a domain, a managed database and any external identity or monitoring service need explicit owner approval before use. None has been chosen or provisioned.

## Incidents

`docs/operations/incident-runbook.md` has one section per alert rule, each with the impact, what to check and how to fix it. The alerts were induced locally; none has fired on a real deployment. For any problem, collect the `request_id` from the error body, the job `id` and its `events`, and the control plane and worker logs. The logs are written so that no credential appears in them: the M4 guide searched its logs for secret and token markers and found none.
