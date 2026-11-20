# CLUSTERX — Deployment

Status: M8 (TASK-M8-BE-01). Two shapes exist:

| Shape | Files | Use |
|---|---|---|
| Local starter (development) | `infra/compose/compose.yaml`, `bootstrap.sh` | laptop/Colima; plaintext gRPC inside the compose network; DB on `127.0.0.1` |
| Production example | `infra/production/` | one Linux host behind Caddy TLS; secrets as files; DB not published |

**No production host has been provisioned or tested** (OD-04: hosting, domain and budget need owner approval). The
production example was verified **locally** on macOS + Colima (arm64) with a self-signed CA; see "Local proof" below.
amd64 reference VM, public DNS/ACME, real GitHub OAuth app: NOT RUN.

## Production example topology

```
Internet ─▶ :443/:80 Caddy ──▶ web:3000 (Next.js BFF)        ─┐ edge network
                       └────▶ https://control-plane:8080 (API) ┘
web ──TLS (internal CA) + EdDSA assertion──▶ control-plane:8080
workers ──gRPC + TLS (internal CA, SNI control-plane) + cxw_ credential──▶ control-plane:9090   (workers network, internal)
control-plane ──▶ postgres:5432                                                              (data network, internal)
prometheus ──TLS + scrape token──▶ control-plane:9091 (internal listener)                    (optional profile)
```

| Port | Exposed | Notes |
|---|---|---|
| 443/tcp, 80/tcp | yes (Caddy) | 80 only redirects to HTTPS |
| 8080 (API), 9090 (gRPC), 9091 (metrics) | no | container networks only; to run workers on other hosts, publish 9090 (TLS is mandatory in production config) and firewall it to worker IPs |
| 5432 | **no** | `data` network is `internal: true` (no egress, no published port) |
| 9190 (Prometheus) | `127.0.0.1` only | reach over SSH tunnel |

Firewall (host): allow inbound 443 and 80 (and 22 for administration); deny everything else inbound. Outbound: the web
container needs HTTPS to GitHub for OAuth; ACME needs outbound HTTPS; nothing else does.

Startup ordering: `postgres` (healthy) → `migrate` (one-shot, must exit 0) → `control-plane` → `web` → `caddy`; workers
after the control plane and retry with backoff until it answers.

Container hardening: every CLUSTERX container runs nonroot (UID 65532), `read_only: true`, `cap_drop: [ALL]`,
`no-new-privileges`, memory/CPU/pids limits; `web` gets a 64 MiB tmpfs for `.next/cache`. Caddy runs as root with only
`NET_BIND_SERVICE`; PostgreSQL uses the official image's own user switching (not read-only).

Secrets flow: files under `infra/production/secrets/` (directory 0700, gitignored) → compose `secrets:` →
`/run/secrets/<name>` → `*_FILE` variables. `docker inspect` shows only paths. The web image reads `CLUSTERX_*_FILE`
through `infra/production/web-entry.mjs` (the web config itself reads plain variables; native `*_FILE` support is
requested from the web lane). `.env.production` (from `.env.production.example`) holds hostnames and ports only.

| Secret file | Consumer | Real deployment source |
|---|---|---|
| `postgres_password`, `app_db_password`, `retention_db_password`, `database_url`, `migrate_database_url` | postgres, migrate, control plane | generated, stored in your secret manager |
| `metrics_token` | control plane, Prometheus | generated ≥ 32 chars |
| `bff.jwks.json` / `bff-private.jwk.json` | control plane / web | Ed25519 key pair; rotate by `kid` |
| `web_session_key` | web | base64 of ≥ 32 random bytes |
| `github_client_id`, `github_client_secret` | web | GitHub OAuth app (owner approval) |
| `control-plane.crt/.key`, `internal-ca.crt` | control plane, web, workers, Caddy, Prometheus | internal CA (SAN `control-plane`) |
| `site.crt/.key` | Caddy (`CLUSTERX_TLS_MODE=files`) | omit with `CLUSTERX_TLS_MODE=acme` |
| `worker_credential_N` | workers | `clusterx-control-plane worker-credential create` (shown once) |

## First start (real host)

```bash
cd infra/production
cp .env.production.example .env.production       # set hostnames; CLUSTERX_TLS_MODE=acme for a public domain
# provision ./secrets/* from your secret store (table above), then:
docker compose --env-file .env.production build
docker compose --env-file .env.production up -d postgres migrate control-plane web caddy
docker compose --env-file .env.production run --rm --no-deps control-plane token create --role operator --name first-operator --owner-github-id <id> --owner-login <login>
docker compose --env-file .env.production run --rm --no-deps control-plane worker-credential create --name worker-1   # x N -> secrets/worker_credential_N
docker compose --env-file .env.production up -d worker worker-2 worker-3
```

`setup-local-proof.sh` does all of this with generated values and a self-signed CA — **for the local proof only**.

## Sizing

Measured only on the development VM (`docs/testing/benchmarks.md`): during a 1,000-job burst the control plane used
≤ 16 MiB RSS and ≈ 0.2 cores, PostgreSQL ≤ 193 MiB and ≈ 0.3 cores, each worker ≤ 9 MiB. The production example's limits
(control plane 512 MiB / 2 CPU, PostgreSQL 1 GiB / 2 CPU, web 512 MiB, workers 256 MiB) are headroom choices, not
measurements. **Minimum/recommended host size is not yet established** — proposal for the reference host: 2 vCPU,
4 GiB RAM, 40 GiB SSD (minimum); 4 vCPU, 8 GiB (recommended), to be confirmed by a benchmark there.

## Local proof (executed 2026-11-20, macOS + Colima arm64)

`.env.production`: `CLUSTERX_WEB_HOST=clusterx.localhost`, `CLUSTERX_API_HOST=api.clusterx.localhost`,
`CLUSTERX_TLS_MODE=files`, bind `127.0.0.1`, HTTPS `18443`, redirect `18088`, project `clusterx-m8-prod`. Commands:
`CLUSTERX_SKIP_BUILD=1 ./setup-local-proof.sh` (images built beforehand), `./verify-local-proof.sh`, then
`docker compose --profile monitoring down -v`, images and `secrets/` removed. Evidence: `docs/testing/evidence/m8/production/`.

| Check | Result |
|---|---|
| Config fails closed (`fail-closed.txt`) | control plane `serve` in production with nothing / unreadable `*_FILE` / default DB password + 5-char metrics token + plaintext listeners: **exit 1**, every problem listed, no value echoed; worker with no credential + `INSECURE=true`: **exit 1** |
| TLS handshake, both hosts | TLSv1.3, chain `site` ← `CLUSTERX local proof CA`, verify 0; client without the CA refused (curl exit 60) |
| API via Caddy | `/health/ready` 200 over HTTP/2; `/metrics` on the API host 404 (not proxied); `/api/v1/jobs` without token 401 |
| Job end to end | `hash_sha256` job submitted with the operator token → `succeeded`, 1 attempt; 3 workers registered with `tls:true` to `control-plane:9090` |
| Web via Caddy | `/` 200, `/demo` 200, `/app` 307 → `/login`; HSTS, CSP, `X-Frame-Options: DENY` present; HTTP 308 → HTTPS |
| BFF → API TLS (`internal-tls.txt`) | `fetch https://control-plane:8080/health/ready` from the web container: 200 (internal CA via `NODE_EXTRA_CA_CERTS`); plain HTTP to the TLS listener: 400 |
| Isolation | web container cannot resolve `postgres` (ENOTFOUND); `data` and `workers` networks `internal=true`; only Caddy publishes ports (127.0.0.1:18443/18088) |
| Hardening (`docker inspect`) | control plane, web, workers: UID 65532, read-only, `cap_drop=[ALL]`, no-new-privileges, mem/pids limits; Caddy: root + `CAP_NET_BIND_SERVICE` only, read-only; PostgreSQL: official image user switching, not read-only |
| Secrets in environment | none (only `/run/secrets/*` paths) |
| Prometheus profile | scrapes `https://control-plane:9091/metrics` (internal CA, bearer file): `up`; `clusterx_workers{state="live"}` = 3 |
| Backup against this stack | `scripts/ops/backup.sh` with `CLUSTERX_COMPOSE_DIR=infra/production`: 66,858 bytes, 129 TOC entries, 0.44 s |

Not exercised: GitHub OAuth login (placeholder client; owner approval), ACME/public certificates, amd64 host, firewall
rules, workers on a separate host, rolling restart. Observed limits: the HTTP→HTTPS redirect targets port 443 (correct
on a real host, not with the remapped 18088/18443 local ports); the web image is 1.43 GB (full workspace incl. dev
dependencies; a pruned/standalone build is a web-lane follow-up).
