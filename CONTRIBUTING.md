# Contributing

Thanks for your interest. CLUSTERX is pre-1.0; the scope and invariants are fixed by
[`docs/product/MASTER_PROMPT.md`](docs/product/MASTER_PROMPT.md) and the ADRs in `docs/architecture/adrs/`.

## Before you change code

- Read the job state machine (`docs/architecture/state-machine.md`) before touching scheduling, leases,
  retries or cancellation. Every job/attempt/event change must stay in one short transaction with CAS checks.
- REST changes start in `api/openapi.yaml`; regenerate the TypeScript client with `pnpm sdk:generate`
  (never hand-edit `packages/sdk-ts/src/schema.d.ts`). gRPC changes start in `packages/proto`; run
  `packages/proto/generate.sh`.
- Migrations are forward-only (expand/contract); never edit a shipped migration.

## Checks

Run before opening a pull request: `go vet` + `go test -race` for each Go module (with `CLUSTERX_PG_ADMIN_DSN`
set to a throwaway PostgreSQL 18 superuser), `pnpm contract:lint`, `pnpm sdk:check`, and
`pnpm --filter web lint typecheck test build e2e`. Tests that could not run must be reported as not run.

## Pull requests

Keep changes small and reviewable, describe behaviour and evidence, never commit secrets, and expect security
or concurrency-sensitive changes to get an independent review. By contributing you agree your work is licensed
under Apache-2.0.

## Conduct

See [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).
