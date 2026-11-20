# Test matrix

Where each class of test lives, how to run it, and what it proves. Acceptance-ID status: `docs/product/traceability.md`.
All suites run locally (macOS arm64 dev host / Colima VM). GitHub CI workflows exist but **have not run** (OD-11/OD-20).

| Layer | Location | Command | Covers |
|---|---|---|---|
| State machine / store | `apps/control-plane/internal/store` | `go test -race ./internal/store/...` (needs `CLUSTERX_PG_ADMIN_DSN`) | AC-02, AC-03, AC-05, AC-06, AC-07, AC-09, AC-10; lock order, fencing, idempotency, retention guards |
| Race PoC (M1) | `apps/control-plane/poc` | `go test -race ./poc/...` | AC-02, AC-03, AC-06 at SQL level |
| REST API + auth | `internal/api`, `internal/auth` | `go test -race ./internal/api/... ./internal/auth/...` | AC-01, AC-07, AC-13 route × principal matrix, stream (AC-16 server), rate limits, metrics bounds |
| Worker protocol | `internal/workerrpc`, `packages/sdk-go`, `apps/worker` | `go test -race ./...` per module | AC-04, AC-09, AC-11, SDK drain/logs |
| Scheduler / recovery | `internal/scheduler` | `go test -race ./internal/scheduler/...` | due promotion, lease-expiry recovery |
| Process-level chaos | `internal/chaos` | `go test -race ./internal/chaos/...` (PG 18 `initdb`/`pg_ctl` on PATH; fails in CI if missing) | AC-04 real `kill -9`, AC-08 DB outage, AC-11 SIGTERM |
| In-process e2e | `internal/e2e` | `go test -race ./internal/e2e/...` | AC-18 (1,000 jobs, 3 workers) |
| Migrations | `internal/migrate` | `go test -race ./internal/migrate/...` | runner, drift refusal, 0002→0004 upgrade drill |
| Contract | `api/openapi.yaml`, `packages/proto`, `packages/sdk-ts` | `pnpm contract:lint`, `pnpm sdk:check`, `packages/proto/generate.sh` + `git diff --exit-code` | AC-20 |
| Web unit | `apps/web/src/__tests__` | `pnpm --filter web test` | BFF core, assertions, cookies, proxy, relay, providers, docs |
| Web E2E (live stack) | `apps/web/e2e/*.spec.ts` | `pnpm --filter web build && CLUSTERX_E2E_PG_DIR=<dir> pnpm --filter web e2e` — Chromium, Firefox, WebKit | AC-13/14/15/16/21/23, demo isolation, CSP, proxy/stream hardening, axe |
| Accessibility | E2E axe scans; manual plan `docs/design/accessibility-plan.md` | as above | AC-22 automated part; **manual SR/keyboard NOT RUN** |
| Performance | `e2e/perf-console.spec.ts`, Lighthouse (`docs/design/evidence/lighthouse-m9.md`) | see evidence files | AC-24 (lab only; budgets not frozen, OD-06) |
| Benchmarks | `scripts/bench` | see `docs/testing/benchmarks.md` | AC-18, AC-19 on dev VM |
| Ops drills | `scripts/ops`, `infra/production/verify-local-proof.sh`, `infra/monitoring/alerts_test.yml` | see `docs/operations/*` | AC-17, AC-27 (local), alerts |
| Supply chain | `scripts/security/scan.sh` | `scripts/security/scan.sh <out> [govulncheck pnpm osv gitleaks sbom]` | AC-25 |
