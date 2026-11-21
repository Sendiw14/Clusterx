<h1 align="center">CLUSTERX</h1>

<p align="center"><b>Jobs that outlive their workers.</b><br>
A self-hosted, at-least-once distributed job engine: a Go control plane on PostgreSQL, independent pull-based workers,
an operator console and a documentation site.</p>

<p align="center">
  <a href="https://github.com/Sendiw14/Clusterx/actions/workflows/ci.yml"><img alt="ci" src="https://github.com/Sendiw14/Clusterx/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/Sendiw14/Clusterx/actions/workflows/security.yml"><img alt="security" src="https://github.com/Sendiw14/Clusterx/actions/workflows/security.yml/badge.svg"></a>
  <img alt="status: pre-1.0, not released" src="https://img.shields.io/badge/status-pre--1.0%20%C2%B7%20not%20released-orange">
  <a href="LICENSE"><img alt="license: Apache-2.0" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
</p>

<p align="center">
  <img src="docs/design/screenshots/m6/overview-1440.png" alt="CLUSTERX console overview on live data: queue and worker counts and the last-hour throughput chart" width="49%">
  <img src="docs/design/screenshots/m6/job-failed-1440.png" alt="Job detail for a failed job showing attempts, events timeline and retry lineage" width="49%">
</p>
<p align="center"><sub>Console on a local stack with mock OAuth (<code>docs/design/screenshots/m6/</code>, captured 2026-11-19 and 2026-11-20).</sub></p>

## What it is

You submit typed, bounded jobs over REST. Workers you run pull them over gRPC, execute a handler you wrote, and report
back. PostgreSQL is the single source of truth: every state change is one short transaction, a job is acknowledged only
after it commits, and an attempt that lost its lease can no longer change the outcome. The operator console shows
what the database says — queue age, workers, attempts, leases, events — and lets operators cancel, retry and drain.

**Status: pre-1.0, not released.** It is built against the contract in [`docs/product/MASTER_PROMPT.md`](docs/product/MASTER_PROMPT.md).
What is implemented, tested and still open is tracked in [`docs/product/traceability.md`](docs/product/traceability.md)
and [`docs/testing/release-gates.md`](docs/testing/release-gates.md).

## How it works

```mermaid
flowchart LR
  B[Browser] -- "HTTPS · session cookie" --> W["Next.js server<br/>(sessions, CSRF, proxy)"]
  W -- "signed service assertion" --> C["Go control plane<br/>REST · scheduler · gRPC"]
  A[API clients] -- "API token" --> C
  C <-->|SQL| P[("PostgreSQL<br/>only authority")]
  K["Workers<br/>trusted handlers"] -- "gRPC · pull" --> C
```

Workers talk to the control plane and never to PostgreSQL. Every call is authorized in Go, whatever its path in.

A job moves through seven states. Leases, fencing and the retry budget decide which transitions are allowed
([state machine](docs/architecture/state-machine.md)):

```mermaid
stateDiagram-v2
  [*] --> scheduled: run_after in the future
  [*] --> queued
  scheduled --> queued: due
  queued --> running: claimed (attempt + lease)
  running --> succeeded
  running --> retry_wait: retryable failure or lease expiry
  retry_wait --> queued: due
  running --> failed: permanent failure or budget exhausted
  queued --> canceled
  scheduled --> canceled
  retry_wait --> canceled
  running --> canceled: cancel acknowledged or lease expires
  succeeded --> [*]
  failed --> [*]
  canceled --> [*]
```

A manual retry of a failed job creates a new, linked job; the old record never changes.

## What it guarantees — and what it does not

- **At-least-once execution.** A job may run more than once (worker crash, lease expiry). Handlers must be
  idempotent for external side effects. There is **no** exactly-once guarantee.
- **PostgreSQL is the only authority.** Jobs, attempts, leases and events change in single short transactions;
  a submission is acknowledged (2xx) only after commit.
- **Fencing.** A late result from an expired or superseded attempt is rejected; it cannot overwrite the
  recorded outcome. Fencing protects the database — it cannot un-run a handler that kept going.
- **Cancellation is cooperative.** A hard deadline limits authorization, not a Go handler's physical execution.
- **Single coordinator.** No HA leader election; coordinator downtime pauses scheduling.
- **Priority scheduling** (0–9, 9 highest) among eligible jobs; no strict global FIFO or fairness promise.
- **Bounded data.** 64 KiB payloads, 128 KiB results, bounded logs; history expires per retention settings.
- **The public `/demo` is illustrative fixture data**, never live telemetry.
- **Workflows/DAGs are a separate 1.1 scope** and are not part of 1.0.

## Quickstart (engine)

Verified from a clean clone on Linux arm64 (Ubuntu VM, independent audit). On macOS arm64 with Colima (Docker 29) it was run from a
checkout under `$HOME`, not from an independently audited clean clone. amd64 is **not yet verified**.
With Colima, keep the checkout under `$HOME` — Colima mounts only `$HOME` into its VM.

```bash
cp .env.example .env   # fill the three empty values, e.g. with: openssl rand -hex 24
./infra/compose/bootstrap.sh
```

This starts PostgreSQL 18.7, runs migrations, starts the control plane on `127.0.0.1:8080` and three workers,
and writes an operator API token to `infra/compose/.secrets.env` (gitignored). Submit a job:

```bash
set -a; . infra/compose/.secrets.env; set +a
curl -s -X POST http://127.0.0.1:8080/api/v1/jobs \
  -H "Authorization: Bearer $CLUSTERX_OPERATOR_TOKEN" -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: first-job' \
  -d '{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"hello"}}'
```

Stop with `docker compose -f infra/compose/compose.yaml --env-file .env --profile workers down`
(add `-v` to delete the database volume).

The **operator console** needs a GitHub OAuth app and the BFF keys (see
[`docs/operations/deployment.md`](docs/operations/deployment.md)); it is end-to-end tested against a local mock
OAuth server, **not yet against real GitHub**. To try the console locally without GitHub, use the E2E harness
(`CLUSTERX_E2E_PG_DIR=<dir> pnpm --filter web e2e`), which starts Go, PostgreSQL, workers and a mock OAuth server.

## What has and has not been verified

| Verified (local dev host / Colima VM; CI on GitHub) | Not verified |
|---|---|
| Go race tests across all modules, including real `kill -9` and database-outage tests | Real GitHub OAuth sign-in, first-operator setup |
| Live end-to-end tests in Chromium, Firefox and WebKit against the real stack (mock OAuth) | An amd64 reference host with a real domain and TLS |
| Backup/restore and migration drills, alert drills, local production example with TLS | Manual screen-reader, keyboard-only and zoom checks |
| Vulnerability, secret and SBOM scans; GitHub `ci` and `security` workflows green on `ad645cd` | Field performance data; frozen SLO / RPO / RTO targets |

One earlier `web` end-to-end run on GitHub failed once and was not reproduced; its cause is unknown.
Benchmarks ([`docs/testing/benchmarks.md`](docs/testing/benchmarks.md)) come from a 4-vCPU Colima VM and are not a capacity claim.

## Repository layout

```text
apps/control-plane   Go coordinator: REST /api/v1, worker gRPC, scheduler, stream, migrations
apps/worker          reference worker binary
apps/web             Next.js public site, docs, operator console, BFF, isolated demo
packages/sdk-go      Go worker SDK          packages/proto   gRPC contract + generated Go
packages/sdk-ts      generated TS client    packages/ui      design tokens + components
api/openapi.yaml     REST contract          examples/        example handlers
infra/compose        local starter          infra/production local-verified production example
infra/monitoring     Prometheus config + alert rules
docs/                product, architecture/ADRs, design, security, testing, operations
```

## Development

| Area | Commands |
|---|---|
| Go (all modules, needs PostgreSQL 18 for DB tests) | `go vet ./...` and `CLUSTERX_PG_ADMIN_DSN=… go test -race ./...` per module (`go.work` at root) |
| Contract | `pnpm contract:lint`, `pnpm sdk:check` |
| Web | `pnpm --filter web lint typecheck test build`; live E2E: `CLUSTERX_E2E_PG_DIR=<dir> pnpm --filter web e2e` |

Toolchain: Go 1.27.4, PostgreSQL 18.7, Node 26, pnpm 9.15.9 — exact versions in
[`docs/architecture/dependency-matrix.md`](docs/architecture/dependency-matrix.md). See [`CONTRIBUTING.md`](CONTRIBUTING.md).

## Documentation

- Architecture and decisions: [`docs/architecture/`](docs/architecture/) (state machine, ADR-0001…0006)
- Security: [`docs/security/`](docs/security/) (threat model, permissions, ASVS mapping) · reporting: [`SECURITY.md`](SECURITY.md)
- Operations: [`docs/operations/`](docs/operations/) · tests: [`docs/testing/test-matrix.md`](docs/testing/test-matrix.md)

## License

Apache License 2.0 — see [`LICENSE`](LICENSE).
