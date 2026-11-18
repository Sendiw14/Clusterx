# TASK-M3-BE-01 — Go foundation: control-plane, worker, SDK skeletons, migrations, health, codegen

- Milestone: M3 foundation. Requirement IDs: FR-API-04, FR-ENG-24 (runner), NFR-OPS-01 (fail-closed config), NFR-DEP-01; AC-08 (readiness part), AC-20 (proto codegen).
- Owner: backend lane, `claude-opus-5-5`, effort medium (requested). Branch `m3/backend`, worktree `.worktrees/backend`.
- Contract base: tag `contract-m1-draft.4` (frozen). **No contract changes in this ticket**; NF-01..06 fixes come in M4 via change request.
- Allowed files: `apps/control-plane/**`, `apps/worker/**`, `packages/sdk-go/**`, `packages/proto/**` (incl. generated Go under `packages/proto/gen/go/**`), `docs/architecture/adrs/**` (new ADRs only), `docs/coordination/handoffs/HANDOFF-M3-BE-01.md`.
- Forbidden: root config (`go.work`, `package.json`, `.github/**`, `infra/**` — integrator creates these), `api/openapi.yaml`, frozen DDL content of `0001_init.sql`, `apps/web/**`, `docs/design/**`, `docs/product/**`.

## Deliverables

1. Go modules: `apps/control-plane` (keep existing `poc/` package green), `apps/worker`, `packages/sdk-go`, `packages/proto` — module path prefix `github.com/Sendiw14/Clusterx/...`. Integrator adds root `go.work` listing them; until then use `replace` directives only if needed and remove them before handoff.
2. Proto Go codegen: pinned `protoc-gen-go` and `protoc-gen-go-grpc` versions installed via `go install …@vX`; reproducible script `packages/proto/generate.sh`; generated code committed under `packages/proto/gen/go`. CI will check `generate.sh` produces no diff.
3. Control plane `cmd/clusterx-control-plane`: config from env, **fail closed** on missing/invalid prod settings (`CLUSTERX_ENV=production` requires TLS settings, non-default secrets, etc.); `pgxpool`; embedded migration runner (`schema_migrations`, forward-only, advisory lock, applies `0001_init.sql` unchanged); HTTP server with `/health/live`, `/health/ready` (DB ping + migrations at expected version), `/metrics` (Prometheus, protected by bearer token from config); structured JSON logging (`log/slog`) with request IDs; graceful SIGTERM shutdown; gRPC server listening (no RPC bodies yet — `Unimplemented` from generated stubs is fine).
4. Worker `cmd/clusterx-worker`: config, connects gRPC, graceful SIGTERM; no claiming logic yet.
5. `packages/sdk-go`: public package skeleton — `Handler` interface (ctx, typed payload), registry with name+version, typed `RetryableError`/`PermanentError`; doc comments. No transport yet.
6. Dockerfiles for control-plane and worker: multi-stage, pinned Go base image tag, static binary, distroless/nonroot (or scratch with nonroot UID), read-only-FS compatible, `HEALTHCHECK` not required.
7. Tests: config fail-closed table tests; migration runner applies to fresh PG18 and is idempotent on second run; `/health/ready` returns 503 when DB down and 200 when up; `go vet`, `go test -race ./...` across all modules.

## Acceptance

`go build ./...` + `go vet ./...` + `go test -race -count=1 ./...` in each module green against PG18 (throwaway cluster on 55432, TCP only, `LC_ALL=en_US.UTF-8`); `generate.sh` reproducible; `docker build` for both images if a Docker daemon is available (Colima is being installed by the integrator — check `docker info`; else NOT RUN). Handoff with exact commands/exit codes and pinned versions for the dependency matrix.
Reviewer: integrator; critical review deferred to M5 security pass.
