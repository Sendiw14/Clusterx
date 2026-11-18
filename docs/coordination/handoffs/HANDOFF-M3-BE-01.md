# HANDOFF — TASK-M3-BE-01 (Go foundation: control plane, worker, SDK skeleton, migrations, health, codegen)

- **Author, actual model/effort, Ultracode:** backend lane subagent, model `claude-opus-5-5`, requested effort medium. Effective effort and Ultracode availability: UNVERIFIED from inside the subagent (MW-04).
- **Branch:** `m3/backend` (worktree `.worktrees/backend`), based on `efbcbd6`. Not pushed, not merged.
- **Contract base:** tag `contract-m1-draft.4` (commit `c48b62f`). `git diff --exit-code contract-m1-draft.4 -- apps/control-plane/migrations/0001_init.sql packages/proto/clusterx api/openapi.yaml` exits 0: the DDL, proto and OpenAPI are unchanged.
- **Requirements:** FR-API-04, FR-ENG-24 (runner), NFR-OPS-01, NFR-DEP-01, AC-08 (readiness part), AC-20 (proto codegen).

## Commits

| SHA | Content |
|---|---|
| `621d1cd` | `packages/proto`: Go module, `generate.sh` (pinned tools), generated `gen/go/clusterx/worker/v1/{worker.pb.go,worker_grpc.pb.go}` |
| `e5c098c` | `packages/sdk-go`: `Handler[P,R]`, `HandlerFunc`, `Registry` (name + version), `RetryableError`/`PermanentError`, `Classify`, limits, tests |
| `a6d1c4e` | control plane: `internal/config` (fail closed), `internal/migrate` (embedded runner), `migrations/embed.go`, `internal/store` (pool + runtime params), `internal/pgtest`. pgx `v5.7.5` → `v5.13.0` |
| `c26002c` | control plane: `internal/api` (health, metrics, request id, access log), `internal/workerrpc`, `cmd/clusterx-control-plane`, `Dockerfile` |
| `029326f` | worker: `internal/config`, `internal/rpcclient`, `cmd/clusterx-worker`, `Dockerfile` |
| (this) | this handoff |

## Modules

| Module | Path | Purpose |
|---|---|---|
| `github.com/Sendiw14/Clusterx/packages/proto` | `packages/proto` | generated worker v1 Go code |
| `github.com/Sendiw14/Clusterx/packages/sdk-go` | `packages/sdk-go` | worker handler SDK (package `sdk`), no dependencies |
| `github.com/Sendiw14/Clusterx/apps/control-plane` | `apps/control-plane` | coordinator binary `clusterx-control-plane` and the existing `poc` package |
| `github.com/Sendiw14/Clusterx/apps/worker` | `apps/worker` | reference worker binary `clusterx-worker` |

## Pinned versions (for `docs/architecture/dependency-matrix.md`)

| Item | Version | Where |
|---|---|---|
| Go toolchain | 1.27.4 (`go 1.27` in every go.mod; Docker `GOTOOLCHAIN=local`) | local `/opt/homebrew/bin/go` |
| PostgreSQL (tests) | 18.7 | local throwaway cluster |
| protoc | 29.3 (`libprotoc 29.3`; header says `v5.29.3`) | `generate.sh` refuses any other version |
| protoc-gen-go | v1.36.12 | `generate.sh`, `go install` into `packages/proto/.bin/` (gitignored) |
| protoc-gen-go-grpc | v1.6.2 | `generate.sh` |
| google.golang.org/grpc | v1.85.0 | proto, control-plane, worker |
| google.golang.org/protobuf | v1.36.12 | proto (indirect elsewhere) |
| github.com/jackc/pgx/v5 | v5.13.0 | control-plane (was v5.7.5; PoC re-tested green) |
| github.com/prometheus/client_golang | v1.25.0 | control-plane |
| Indirect deps worth tracking | golang.org/x/net v0.60.0, x/sys v0.48.0, x/text v0.42.0, x/sync v0.23.0, genproto/googleapis/rpc v0.0.0-20260817212433-ac3dfec99bb1, prometheus/common v0.72.0, procfs v0.22.0, client_model v0.6.3, puddle/v2 v2.2.3 | go.sum |
| Build image | `golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d` (index digest from Docker Hub, 2026-11-18) | both Dockerfiles |
| Runtime image | `gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931` (index digest from gcr.io, 2026-11-18) | both Dockerfiles |

## Implemented (not integrated)

- **Config, fail closed (NFR-OPS-01).** `CLUSTERX_ENV` is required (`development` | `production`); there is no default. All errors are reported together, and error text never contains secret values (tested). Production refuses:
  - a missing or short (< 32) `CLUSTERX_METRICS_TOKEN`, or a placeholder value;
  - a default or placeholder DB password (`postgres`, `clusterx`, `changeme*`, …);
  - gRPC without TLS unless the listener is loopback;
  - HTTP without TLS unless the listener is loopback or `CLUSTERX_HTTP_TLS_OFFLOADED=true` (a TLS reverse proxy on a private network).
  - Migrate-on-start defaults to `true` in development and `false` in production (least-privilege runtime role).
- **Control-plane variables:** `CLUSTERX_ENV`, `CLUSTERX_DATABASE_URL`, `CLUSTERX_MIGRATE_ON_START`, `CLUSTERX_HTTP_ADDR` (`:8080`), `CLUSTERX_HTTP_TLS_CERT_FILE`/`_KEY_FILE`, `CLUSTERX_HTTP_TLS_OFFLOADED`, `CLUSTERX_METRICS_ADDR` (`:9091`), `CLUSTERX_METRICS_TOKEN`, `CLUSTERX_GRPC_ADDR` (`:9090`), `CLUSTERX_GRPC_TLS_CERT_FILE`/`_KEY_FILE`, `CLUSTERX_LOG_LEVEL`, `CLUSTERX_SHUTDOWN_TIMEOUT` (`20s`). The integrator needs these for `.env.example` and compose.
- **Worker variables:** `CLUSTERX_ENV`, `CLUSTERX_WORKER_CONTROL_PLANE_ADDR`, `CLUSTERX_WORKER_CREDENTIAL` (`cxw_…`, required in production), `CLUSTERX_WORKER_INSECURE` (rejected in production unless the target is loopback), `CLUSTERX_WORKER_TLS_CA_FILE`, `CLUSTERX_WORKER_TLS_SERVER_NAME`, `CLUSTERX_WORKER_NAME`, `CLUSTERX_WORKER_CAPACITY` (1..256, default 4), `CLUSTERX_LOG_LEVEL`, `CLUSTERX_SHUTDOWN_TIMEOUT` (`30s`).
- **Migration runner (FR-ENG-24).**
  - The binary embeds `migrations/*.sql` through `migrations/embed.go`. `0001_init.sql` is byte-identical to the tag.
  - Applied versions are recorded in `schema_migrations(version, name, checksum sha256 hex, applied_at)`.
  - Runners are serialized by a session `pg_advisory_lock(0x636c757374657278)` on a dedicated connection with `statement_timeout = 0`.
  - Each migration and its bookkeeping row commit in one transaction. The runner is forward-only.
  - Start is refused when an applied file was edited (checksum), when the DB is ahead of the binary, or when there is a gap.
  - `clusterx-control-plane migrate` applies and exits. `serve` applies only when `CLUSTERX_MIGRATE_ON_START=true`.
- **HTTP.**
  - `GET /health/live` → `{"status":"ok"}`.
  - `GET /health/ready` runs a DB ping and `migrate.Verify` (exact version and checksums) with a 2 s bound. It returns 200 `{"status":"ready"}`, or 503 `Error{code:"unavailable", message:"Not ready.", request_id}` with `Retry-After: 5`. The reason is logged, not returned.
  - `GET /metrics` runs on the separate internal listener. It needs `Authorization: Bearer <token>`, compared as SHA-256 digests in constant time. Otherwise it returns 401 `Error{code:"unauthenticated"}` with `WWW-Authenticate: Bearer`. An empty configured token always denies.
  - `X-Request-Id` is echoed when it matches `^[A-Za-z0-9._:-]{1,64}$`; otherwise `req_<24 hex>` is generated.
  - Every request writes one JSON `slog` access log line (request_id, route pattern, status, duration_ms). Timestamps are UTC.
  - Panic recovery returns a 500 `internal` error body.
  - Metrics `clusterx_http_requests_total{server,route,code}` and `clusterx_http_request_duration_seconds{server,route}` use the route pattern, so cardinality is bounded. Unmatched paths get `route="unmatched"`. Go and process collectors are included.
  - Server timeouts: ReadHeader 5 s, Read 15 s, Write 30 s, Idle 120 s, MaxHeaderBytes 16 KiB.
- **DB pool runtime params (ADR-0001 §6):** `statement_timeout=5s`, `idle_in_transaction_session_timeout=10s`, `synchronous_commit=on`, `timezone=UTC`, `application_name`. The pool is created without waiting for the DB, so DB down means readiness 503, not a crash. A start with migrate-on-start and the DB down exits 1.
- **gRPC:** `WorkerService` is registered with the generated `UnimplementedWorkerServiceServer`, so every RPC returns `Unimplemented`. Limits are 256 KiB receive and send (tested: oversize → `ResourceExhausted`). TLS is used when configured. Shutdown is `GracefulStop` with a forced `Stop` after `CLUSTERX_SHUTDOWN_TIMEOUT`, then the HTTP servers shut down and the pool closes.
- **Worker:**
  - `grpc.NewClient` with the 256 KiB call limits and a TLS 1.2+ config (system roots or `CLUSTERX_WORKER_TLS_CA_FILE`).
  - The per-RPC bearer is `authorization: Bearer cxw_…`. Its `RequireTransportSecurity()` is true unless plaintext was explicitly configured, so grpc-go will not send the credential over plaintext.
  - `WaitReady` logs each state transition. The worker exits 0 on SIGTERM/SIGINT.
  - It does not claim, register or heartbeat yet.
- **SDK:**
  - `Register[P,R](reg, name, version, Handler[P,R])` validates `^[a-z][a-z0-9_.-]{0,62}$` and version 1..1000, and rejects duplicates.
  - The payload is decoded strictly (unknown fields and trailing data → `PermanentError invalid_payload`). The payload limit is 64 KiB. A result over 128 KiB → `PermanentError result_too_large` and is never truncated.
  - A panic → `RetryableError handler_panic`. An untyped error → retryable `handler_error`. `PermanentError` wins in `Classify`.
  - `Keys()` returns the advertised capabilities in sorted order. The package doc covers at-least-once delivery, idempotency and cancellation.
- **Dockerfiles:** `apps/{control-plane,worker}/Dockerfile`, multi-stage. Build context is the **repository root**, because the proto module is copied. The image holds a static `CGO_ENABLED=0` binary on distroless `nonroot` (`USER 65532:65532`). It works with `--read-only --cap-drop ALL` (tested).

## Commands, exit codes, results

Environment: `PATH=/opt/homebrew/bin:$PATH`. PG cluster:

```
initdb -D <scratchpad>/pgbe -U clusterx --auth=trust -E UTF8
LC_ALL=en_US.UTF-8 pg_ctl -D <scratchpad>/pgbe -o "-p 55432 -k '' -c listen_addresses=localhost" -w -l <scratchpad>/pgbe.log start
```

`initdb`, `pg_ctl` and `psql` all exited 0 (PostgreSQL 18.7). `CLUSTERX_PG_ADMIN_DSN=postgres://clusterx@localhost:55432/postgres`. `<WS>` is a scratch `go.work` created by `go work init` that lists the four modules.

| Check | Command (run in each module dir unless noted) | Exit | Result |
|---|---|---|---|
| Baseline PoC (before changes) | `go test -race -count=1 ./poc/` in `apps/control-plane` | 0 | PASS |
| Build | `GOWORK=off go build ./...` × 4 modules | 0 ×4 | PASS |
| Vet | `GOWORK=off go vet ./...` × 4 modules | 0 ×4 | PASS |
| Race tests, standalone | `GOWORK=off go test -race -count=1 ./...` × 4 modules (DSN set) | 0 ×4 | PASS: proto (no tests), sdk-go, control-plane (api, config, migrate, workerrpc, **poc**), worker (config, rpcclient) |
| Race tests, workspace | `GOWORK=<WS> go test -race -count=1 ./...` × 4 modules | 0 ×4 | PASS |
| Build+vet, workspace | `GOWORK=<WS> go build ./... && GOWORK=<WS> go vet ./...` × 4 | 0 ×4 | PASS |
| Skips | `go test -race -count=1 -v ./...` in control-plane, grep `^--- (SKIP\|FAIL)` | — | none. 35 top-level PASS: integration tests ran against PG18, none skipped |
| Tidy | `GOWORK=off go mod tidy -diff` × 4 | 0 ×4 | no diff |
| gofmt | `gofmt -l apps packages` | 0 | no output |
| Codegen reproducible | `./packages/proto/generate.sh && git diff --exit-code packages/proto/gen` (second run, after the commit) | 0 / 0 | PASS: no diff |
| Frozen contract | `git diff --exit-code contract-m1-draft.4 -- apps/control-plane/migrations/0001_init.sql packages/proto/clusterx api/openapi.yaml` | 0 | unchanged |
| Binary smoke | build both, run cp (dev, 127.0.0.1 ports) against `clusterx_smoke`, curl live/ready/metrics, start worker, `kill -TERM` both | cp 0, worker 0 | migration applied → live 200, ready 200, metrics 401 without the token and 200 with it, worker "connected", clean shutdown |
| Fail closed (binary) | `CLUSTERX_ENV=production CLUSTERX_DATABASE_URL=postgres://clusterx:clusterx@… clusterx-control-plane` | 1 | four errors reported, no secrets |
| Docker daemon | `docker info` | 0 | Server 29.5.2 (Colima). It was down at the start of the ticket and came up later |
| Image build | `docker build -f apps/control-plane/Dockerfile -t clusterx-control-plane:m3-dev .` (repo root) | 0 | arm64, 6.95 MB, User 65532:65532 |
| Image build | `docker build -f apps/worker/Dockerfile -t clusterx-worker:m3-dev .` | 0 | arm64, 5.23 MB, User 65532:65532 |
| Image smoke | `docker run --rm --read-only clusterx-control-plane:m3-dev` | 1 | fail closed (`CLUSTERX_ENV is required`) |
| Image smoke | `docker run -d --read-only --cap-drop ALL --security-opt no-new-privileges -p 28080:8080 -e CLUSTERX_ENV=development -e CLUSTERX_MIGRATE_ON_START=false -e CLUSTERX_DATABASE_URL=postgres://clusterx@127.0.0.1:1/none?connect_timeout=1 …` then curl, then `docker stop` | container exit 0 | live 200, ready 503 `unavailable` (DB unreachable), graceful stop |
| Image smoke | worker image with `CLUSTERX_ENV=production`, insecure to `cp:9090` | 1 | fail closed |

### Tests added (what they prove)

- `internal/config`: a fail-closed table with 23 cases (dev and prod), secrets absent from errors, loopback detection.
- `internal/migrate`:
  - `Load` validation.
  - On fresh PG18: apply → version 1 and core tables exist; a second run applies 0.
  - **6 concurrent runners apply exactly once.**
  - An edited applied file → `ErrDrift`. The DB ahead of the binary → `ErrDrift`. Pending → `ErrPending`.
  - A failing migration leaves no partial DDL and no bookkeeping row.
- `internal/api`:
  - live/ready with a fake check; the 503 body matches the Error schema (`DisallowUnknownFields`) and the internal reason is not leaked.
  - Request-id echo and replacement; JSON access log fields.
  - Metrics bearer: 7 bad header forms → 401, the right token → 200, an empty configured token → always 401; bounded `route` label.
  - Panic → 500 `internal`.
  - **PG18 readiness:** 503 unmigrated → 200 migrated (and the runtime params are applied) → 503 against an unreachable DB, while live stays 200.
- `internal/workerrpc`: real gRPC server → `Unimplemented`; an oversized message → `ResourceExhausted`.
- worker `internal/config`: an 18-case table. `internal/rpcclient`: connect + bearer metadata received by the server; `WaitReady` honours ctx while the control plane is down; TLS mode against a plaintext server never succeeds.
- sdk-go: registration validation, typed round trip, 9 failure classes, concurrent register/invoke under `-race`, a runnable `ExampleRegister`.

## Deviations and integration needs (please read)

1. **`replace` directives remain (deviation from deliverable 1, with evidence).** `apps/control-plane/go.mod` and `apps/worker/go.mod` keep `replace github.com/Sendiw14/Clusterx/packages/proto => ../../packages/proto` with `require … v0.0.0-00010101000000-000000000000`.
   - I removed them and tested with a `go.work` that lists all four modules. On Go 1.27.4, `go build` and `go vet` then fail with `github.com/Sendiw14/Clusterx/packages/proto@v0.0.0-00010101000000-000000000000: invalid version: unknown revision 000000000000`. The workspace does not override an explicit requirement when external imports force a module-graph load.
   - A minimal two-module repro (`example.com/a` requires `example.com/b`, both in `go.work`) shows the same behaviour. The same build passes when the replace is in the module go.mod, or in `go.work`, or when the require line is removed.
   - Keeping the module-level replace makes `GOWORK=off` builds, `go mod tidy`, and the per-app Docker builds work. A root `go.work` also works on top of it (tested: workspace build, vet and race tests are all exit 0).
   - **Integrator decision needed:** keep the module replaces (recommended), or move them into `go.work` (`replace github.com/Sendiw14/Clusterx/packages/proto v0.0.0-00010101000000-000000000000 => ./packages/proto`). The second option makes standalone module builds and Docker builds depend on `go.work`.
   - Suggested root `go.work`: `go 1.27.4` plus `use ./apps/control-plane ./apps/worker ./packages/proto ./packages/sdk-go`.
2. **New SQL object `schema_migrations` (integrator review required).** The migration runner creates it (`CREATE TABLE IF NOT EXISTS`, in `internal/migrate/migrate.go`), not `0001_init.sql`. The ticket requires it, but it is a schema object outside the frozen DDL. Please record it in the schema contract or in an ADR note. Advisory lock key: `0x636c757374657278`.
3. **CI tools for AC-20:** CI must have protoc **29.3** on PATH. `generate.sh` exits 1 for any other version, because the version is written into the generated headers. The plugins are fetched with `go install`, which needs network or a module cache.
4. **Docker build context is the repo root** (`docker build -f apps/<app>/Dockerfile .`). There is no root `.dockerignore`, because root config is forbidden to this lane. The integrator should add one (exclude `.git`, `.worktrees`, `node_modules`, `apps/web`, `packages/proto/.bin`). The images were built on arm64 only; amd64 is NOT RUN.
5. **Contract observation for M4 (no change made):** OpenAPI `Error.code` has no generic `not_found` or `method_not_allowed`. Unknown routes and wrong methods therefore currently get Go's default `text/plain` 404/405, not the Error schema. Suggest adding this to the NF change request.
6. **Metrics listener TLS** reuses the HTTP TLS settings. With `CLUSTERX_HTTP_TLS_OFFLOADED=true`, `/metrics` is plaintext on the internal listener and is protected only by the bearer token plus network placement. This is documented here and needs a decision in the deployment docs.
7. **Request-id format:** generated ids are `req_` plus 24 hex characters. Inbound ids must match `^[A-Za-z0-9._:-]{1,64}$` (OpenAPI only says maxLength 64).

## NOT RUN

- amd64 image build and run, and any multi-arch build.
- Image vulnerability scan, SBOM and provenance (M5 / integrator CI).
- TLS end-to-end (real certificates) for HTTP, metrics and gRPC. The code paths compile and production config demands TLS, but no handshake test with a real certificate was run.
- CI execution. There is no CI yet; all evidence above is local.
- `go test -race` on Linux. All runs were darwin/arm64.
- Least-privilege runtime DB role (migrations ran as the superuser of the throwaway cluster).

## Security / compatibility review notes

- No secrets in logs or errors: the DB URL, metrics token and worker credential are never logged (tested for config errors). Readiness and metrics errors expose no internals.
- The metrics token comparison is constant time over SHA-256 digests.
- The worker credential cannot leave over plaintext unless plaintext was explicitly configured, and production forbids that for non-loopback targets.
- No auth on gRPC yet. This is acceptable because every RPC is `Unimplemented`. Credential verification (ADR-0005 §4) must land together with the first real RPC.
- Critical review is deferred to the M5 security pass, per the ticket.

## Next owner

The integrator: decide deviation 1, record deviation 2, add root `go.work`, `.dockerignore`, CI (protoc 29.3, PG18 service, `CLUSTERX_PG_ADMIN_DSN`) and the dependency matrix rows above. After that, backend M4 tickets (store, claim/lease/report, auth).

## Traceability

FR-API-04 (health/metrics part), FR-ENG-24 (runner), NFR-OPS-01, NFR-DEP-01, AC-08 (readiness part), AC-20: **implemented + tested locally**. Not integrated, not verified in CI.
