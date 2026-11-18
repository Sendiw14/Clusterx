# CLUSTERX — Traceability Matrix

Vocabulary (§0): `proposed → specified → approved → implemented → integrated → tested → verified → released`.
A status is recorded only with evidence. Unrun tests are **NOT RUN**.

Master revision: R2 · adopted 2026-11-17 (OD-01).
**Contract freeze candidate (M1):** `api/openapi.yaml` `1.0.0-draft.4`, `worker.proto`, `0001_init.sql` at merge `c48b62f` (backend `182566e`). Status **specified, independently reviewed — awaiting owner acceptance (OD-12)**. Not "approved" until owner accepts.

## M1 evidence (main `2ea9d0b`, 2026-11-18, dev laptop, not CI)

| Check | Command | Result |
|---|---|---|
| DDL | `psql -v ON_ERROR_STOP=1 --single-transaction -f apps/control-plane/migrations/0001_init.sql` (fresh PG 18.7) | exit 0 |
| Go vet | `go vet ./...` (Go 1.27.4) | exit 0 |
| Race PoC | `go test -race -count=3 ./poc/...` | exit 0 (`ok … 30.410s`; 19 tests × 3) |
| Proto | `protoc --descriptor_set_out=/dev/null clusterx/worker/v1/worker.proto` | exit 0 |
| OpenAPI | `npx @redocly/cli@2.79.0 lint api/openapi.yaml` | exit 0, 5 expected warnings |
| Mutation | author 21 runs (20 killed, M4 equivalent); reviewer R2: M1–M5, M11 + 7 author + 11 own | per REVIEW-M1-RV-01 R2 |
| Independent review | REVIEW-M1-RV-01 rounds 1–2 (Opus high, read-only) | **M1 exit PASS**, 0 P0/P1 open |
| CI | — | **NOT RUN** (no CI yet; M3) |

## Requirement status

| ID | Status | Evidence | Next gate |
|---|---|---|---|
| FR-ENG-02/03 states + transitions | specified, reviewed | state-machine.md, ADR-0002 | M4 impl |
| FR-ENG-04 atomic claim | specified; PoC tested | AC-02 PoC tests | M4 |
| FR-ENG-10/15/16 lease, recovery, fencing | specified; PoC tested (fence clauses, mutant-killed) | fence_test.go, ADR-0003 | M5 |
| FR-ENG-11 retry budget | specified; PoC tested (no hidden 4th) | TestBudgetExhaustedNoHiddenAttempt | M5 |
| FR-ENG-13 cancel precedence | specified; PoC tested (forced both orders) | AC-06 tests | M5 |
| FR-ENG-17 idempotency | specified | ADR-0004, DDL | **PoC NOT RUN** — M4 |
| FR-ENG-01, 05–09, 12, 14, 18–24 | specified | ADRs, DDL, proto, OpenAPI | M4/M5 |
| FR-API-01..04 | specified (OpenAPI draft.4 lint-valid) | api/openapi.yaml | M3 codegen (TS client NOT RUN) |
| FR-SEC-01..05, 11 | specified, reviewed | ADR-0005/0006, permissions.md, trust-boundaries.md | M5; token authority SQL-tested only |
| FR-SEC-06..10 | specified | requirements.md | RC |
| FR-RT-01 | specified (hints + snapshot) | OpenAPI `/stream`, ADR-0006 | M6 (RT gaps deferred) |
| FR-WEB-01..09, FR-DEMO-01 | specified | route-inventory.md (24 routes), consumer-requirements.md | M2 design |
| NFR-* | specified | requirements.md | per release |
| AC-02, AC-03, AC-06, AC-09 (DB layer) | **tested at PoC level** — not service-level | PoC under `-race` | re-test in M4/M5 against real services |
| AC-05 | tested at PoC level (budget) | — | jitter/backoff M5 |
| Other AC | specified | MASTER §11 | NOT RUN |
| MW-01 | met | tasks/*, one owner per ticket | ongoing |
| MW-02 | partial | contract drafts; generated client NOT RUN | M3 |
| MW-03 | met | 2 worktrees, merged without shared-file conflict; reviewer ≠ author | ongoing |
| MW-04 | partial | session-log.md | owner `/effort`, `/status` |
| MW-05 | met | reviewer read-only, no self-approval | ongoing |
| MW-08 | met | OWNER_DECISIONS.md; no push/deploy/spend | ongoing |
