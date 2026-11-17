# TASK-M1-BE-01 — Core engine contracts, ADRs and race proof

- Scope/release/milestone: M1 architecture (Alpha/Beta contracts)
- Requirement IDs: FR-ENG-01..24, FR-API-01..04, FR-SEC-01..05, FR-SEC-11, FR-RT-01; AC-01..AC-09, AC-13, AC-16; MW-01..04
- Owner/model/effort: backend lane — `claude-opus-5-5`, effort medium (requested via Agent param; see session-log.md)
- Branch/worktree: `m1/backend` at `.worktrees/backend`
- Contract base: master R2 @ M0 commit (see HANDOFF)
- Dependencies: none upstream. Downstream: TASK-M1-FE-02 sign-off, TASK-M1-RV-01 review
- Allowed files:
  - `docs/architecture/adrs/**`, `docs/architecture/state-machine.md`, `docs/architecture/api-contracts.md`, `docs/architecture/overview.md`, `docs/security/permissions.md`, `docs/security/trust-boundaries.md`
  - `apps/control-plane/**` (migrations draft, `poc/` throwaway experiments), `packages/proto/**`
  - **Temporary ownership transfer (integrator → backend, this ticket only):** `api/openapi.yaml` draft. Integrator freezes it at M1 exit.
  - `docs/coordination/handoffs/HANDOFF-M1-BE-01.md`
- Forbidden: `apps/web/**`, `packages/ui/**`, `docs/design/**`, `docs/product/**`, root config, `.github/**`, `CLAUDE.md`, `.claude/**`
- Deliverables:
  1. ADRs: 0001 single modular coordinator + PG authority; 0002 state machine & cancel/success commit-order precedence; 0003 leases, heartbeat vs renewal, fencing, DB clock, timing bounds; 0004 IDs, idempotency, payload bounds, retention vs immutability; 0005 trust boundaries (browser/BFF assertion/direct token/worker credential/demo isolation/WebSocket).
  2. `state-machine.md`: complete event × state table incl. every denial and its error code.
  3. `apps/control-plane/migrations/0001_init.sql`: DDL with CHECK/FK/unique/partial-unique constraints (current-lease uniqueness, attempt numbering, idempotency scope) + indexes per §6.
  4. `packages/proto/clusterx/worker/v1/worker.proto`: §6 RPCs, size limits/deadlines documented.
  5. `api/openapi.yaml` draft: every §6 `/api/v1` op, typed error schema, examples incl. 401/403/404/409/413/422/429/503, cursor pagination, viewer field masking.
  6. `docs/security/permissions.md`: route × role × field matrix.
  7. Race PoC in `apps/control-plane/poc/` (Go + real PostgreSQL 18): AC-02 (≥20 concurrent claimants, ≤1 authorized lease, failpoint no partial writes), AC-03 (stale success rejected after expiry+reclaim), AC-06 (cancel vs success both orders → exactly one terminal outcome). `go test -race`.
- Acceptance: PoC tests pass under `-race` against PG18; DDL applies cleanly; proto compiles with `protoc`; OpenAPI parses. Report anything NOT RUN.
- Security: no secrets in files; credentials only as hashes in schema.
- Reviewer: TASK-M1-RV-01 (Opus high, read-only)
