---
title: Deployment
lead: What the local Compose stack contains and how it is hardened, and what a single-VM production setup will need. Production is not yet verified.
description: CLUSTERX local Docker Compose reference, database roles and secrets, and the production single-VM plan clearly marked not yet verified.
blurb: Local Compose stack, database roles, secrets and configuration. The production section is marked not yet verified.
order: 6
---

> [!UNVERIFIED] **Only the local Compose stack has been run.** Everything under "Production on a single VM" is a plan for milestone M8 and has not been exercised. No TLS setup, backup, restore, upgrade or recovery-time figure on this page has been tested.

## Local Compose reference (verified)

`infra/compose/compose.yaml` is marked in its own header as **development only, not a production example**. Start it with the steps in the [Quickstart](/docs/quickstart). The integrator ran it on Colima on 2026-11-18.

| Service | Role | Published port |
| --- | --- | --- |
| `postgres` | PostgreSQL 18.7, image pinned by digest, data in the named volume `pgdata` | 127.0.0.1:5432 (change with `CLUSTERX_PG_PORT`) |
| `migrate` | One-shot: applies the database migrations as the schema owner, then exits | none |
| `control-plane` | REST on 8080, gRPC on 9090 (internal), metrics on 9091 | 127.0.0.1:8080 and 127.0.0.1:9091 (change with `CLUSTERX_HTTP_PORT`, `CLUSTERX_METRICS_PORT`) |
| `worker`, `worker-2`, `worker-3` | The reference worker running the three example handlers, one credential each, capacity 4 | none |
| `retention` | Optional, profile `retention`: one bounded retention pass as the `clusterx_retention_job` login, then exits. Run it from cron: `docker compose --env-file ../../.env --profile retention run --rm retention` | none |

Every published port is bound to loopback. The web console is **not** part of this stack.

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

The server is started with `node server.ts` (it hosts the live-update WebSocket relay, which a plain `next start` cannot). OAuth endpoint overrides exist only outside production.

> [!UNVERIFIED] Production mode (TLS, `__Host-` secure cookies, real GitHub) has not been run.

## Database roles

The design separates two database identities:

- A **schema owner** that runs migrations (`CLUSTERX_MIGRATE_DATABASE_URL`).
- A **runtime role** (`CLUSTERX_DATABASE_URL`) that is a member of `clusterx_runtime`: it can read, insert and update, can delete only from a few expiring tables, and cannot truncate or change the schema. Delete and truncate guards on the history tables are enforced by database triggers.

In production mode the control plane **refuses to start** if its runtime role is a superuser or can create objects. In the M4 curl guide, starting it with a superuser runtime URL exited with status 1 and the message `runtime database role is superuser or may CREATE…`.

Migrations are forward-only, numbered files applied one transaction each, under an advisory lock. Readiness reports not ready until the applied set matches what the binary expects.

> [!VERIFIED] Migrations applying (`applied=2`), the superuser refusal and the least-privilege role were executed in the M4 curl guide. Deleting and truncating through the runtime role is covered by backend tests.

## Production on a single VM (M8, not yet verified)

> [!UNVERIFIED] Nothing in this section has been run. It records the intended shape so the gaps are visible.

**Shape.** One Linux VM runs the control plane, PostgreSQL, the web application and the workers behind a TLS reverse proxy. Local demonstration and production configuration are kept separate, and the public demo uses no production data.

| Topic | Intent | Status |
| --- | --- | --- |
| TLS | Terminate TLS at a reverse proxy; workers use TLS to a non-loopback control plane (`CLUSTERX_WORKER_TLS_CA_FILE`) | Not yet verified |
| Secrets | Generated per environment, never in the repository, rotated by creating a new credential and revoking the old one | Not yet verified |
| Networking | Only the proxy is public; PostgreSQL, gRPC and metrics stay private | Not yet verified |
| Backups | Logical or physical backups kept for 14 days (proposed) | Not yet verified |
| Restore | A tested restore into a fresh database | Not yet verified |
| Upgrades | Expand, backfill, contract across releases; no destructive change in the release that stops using a column | Specified; never run |
| Coordinator restarts | Downtime: there is one control plane, so claims and reports pause | Restart exercised locally in the M4 guide |
| RPO and RTO | **Not set.** They are to be frozen after measurements | Not yet verified |
| Monitoring | Prometheus metrics endpoint exists; dashboards and alerts do not | Not yet verified |

Hosting, a domain, a managed database and any external identity or monitoring service need explicit owner approval before use. None has been chosen or provisioned.

## Incidents

There is no incident runbook yet. When something goes wrong, collect the `request_id` from the error body, the job `id` and its `events`, and the control plane and worker logs. The logs are written so that no credential appears in them: the M4 guide searched its logs for secret and token markers and found none.
