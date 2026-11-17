# HANDOFF — TASK-M1-BE-01

- **Author, actual model/effort, Ultracode availability:** backend lane subagent. Model `claude-opus-5-5` (Opus 5.5, from the session system prompt). Requested effort: medium (Agent param). Effective effort and Ultracode were **not observable** from inside the subagent: UNVERIFIED (MW-04, R-12).
- **Branch, commits, changed files:** `m1/backend` (worktree `.worktrees/backend`), base `5a5ca99`.
  - `6c41f4e` docs(arch): ADRs 0001–0005 and normative job state machine
  - `85eef97` feat(proto): worker gRPC protocol v1 draft
  - `397e339` feat(api): OpenAPI 3.1 draft for /api/v1 with typed errors, pagination, masking
  - `b7953b5` docs(security): permissions matrix, trust boundaries, architecture overview
  - `d713679` feat(db): draft migration 0001 with lifecycle constraints and indexes (unvalidated)
  - `ad9df49` test(poc): race PoC for AC-02/03/06 against PostgreSQL 18 (not yet run)
  - `5182019` docs(handoff): HANDOFF-M1-BE-01
  - follow-up: `08f30ab` go.sum, `060383f` claim EXPLAIN evidence, plus this handoff update
  - Files: `docs/architecture/adrs/000{1..5}-*.md`, `docs/architecture/{state-machine,overview,api-contracts}.md`, `docs/security/{permissions,trust-boundaries}.md`, `apps/control-plane/{go.mod,migrations/0001_init.sql,poc/README.md,poc/store.go,poc/store_test.go}`, `packages/proto/clusterx/worker/v1/worker.proto`, `api/openapi.yaml` (temporary ownership transfer), this file. Nothing outside the allowed paths.
- **Contract SHA, generated code, migration impact:** source contract = master R2 @ `5a5ca99`. No generated code. Migration 0001 is the initial schema (no existing data).
- **Implemented/Integrated:** all 7 deliverables are written. The PoC passes under `-race` on PG 18.7 (follow-up 2026-11-17, commits `08f30ab`, `060383f`). Nothing is integrated.

## Commands, exit codes, results

| Check | Command | Exit | Result |
|---|---|---|---|
| Proto compiles | `cd packages/proto && protoc -I . -I /opt/anaconda3/include --descriptor_set_out=/dev/null clusterx/worker/v1/worker.proto` (libprotoc 29.3) | 0 | **PASS** |
| OpenAPI parses as YAML | `python3 -c 'import yaml; d=yaml.safe_load(open("api/openapi.yaml")); …'` (PyYAML 6.0.3) → `yaml ok; paths: 15 ops: 16` | 0 | **PASS** |
| OpenAPI `$ref`s resolve + every `/api/v1` op has 401/403/429/503 | inline python3 script over the parsed YAML → `refs 35 unresolved []`, no missing codes | 0 | **PASS** |
| OpenAPI full schema validation | — (no `openapi-spec-validator`/redocly installed) | — | **NOT RUN** |
| OpenAPI lint (follow-up, BE-02) | `npx --yes @redocly/cli@2.79.0 lint api/openapi.yaml` | 0 | **PASS** after fixing 8 struct errors that the PyYAML parse missed (comma-split flow descriptions). See HANDOFF-M1-BE-02. |
| DDL applies on PG18 | `psql -h localhost -p 55432 -U clusterx -d ddlcheck -v ON_ERROR_STOP=1 --single-transaction -q -f apps/control-plane/migrations/0001_init.sql` (PostgreSQL 18.7 Homebrew) | 0 | **PASS** on the first apply, no edits needed |
| Module | `go mod tidy` (go1.27.4 darwin/arm64) | 0 | go.sum committed `08f30ab` |
| Vet | `go vet ./...` | 0 | **PASS** |
| PoC race | `CLUSTERX_PG_ADMIN_DSN='postgres://clusterx@localhost:55432/postgres?sslmode=disable' go test -race -count=1 ./poc/...` | 0 | **PASS**: `ok github.com/Sendiw14/Clusterx/apps/control-plane/poc 2.920s` |
| PoC race ×3, verbose | same with `-count=3 -v` | 0 | **PASS**: all 8 tests passed in each of 3 runs; `ok … 4.472s`; no SKIP lines |
| AC-06 orderings | from the `-v` log | — | run 1 `cancel-first:96 success-first:4`; runs 2–3 `cancel-first:98 success-first:2`. Both orders were exercised. |
| Mutation check | removed `AND j.cancel_requested_at IS NULL` from the success CAS, ran `-run TestAC06` | 1 | **FAIL as expected**: `TestAC06_CancelThenSuccess` and `TestAC06_ConcurrentCancelVsSuccess` failed, and the DB CHECK `jobs_check7` also rejected the write. Restored with `git checkout`. |
| Claim EXPLAIN | `EXPLAIN (ANALYZE, BUFFERS, VERBOSE, SETTINGS)` on 100k seeded jobs | 0 | `docs/architecture/evidence/claim-explain.txt`: 6.6 ms. It sorts all eligible queued rows instead of walking the index top-N (finding recorded; follow-up in M4) |

History: the first pass ended with DDL and PoC **NOT RUN** because the toolchain was still installing. The integrator then installed it, and the rows above are from the follow-up run.

Cluster: `initdb -D <scratchpad>/pg18data -U clusterx --auth=trust`, then `pg_ctl … -o "-p 55432 -k '' -c listen_addresses=localhost"`. There were two deviations from the brief:
1. **No Unix socket.** The scratchpad path is longer than the 103-byte socket path limit (`FATAL: could not create any Unix-domain sockets`), so the cluster used TCP on localhost only (`-k ''`).
2. **`LC_ALL=en_US.UTF-8` was required.** Without it the server stopped at startup with `FATAL: postmaster became multithreaded during startup`.

The cluster was stopped at the end of TASK-M1-BE-02 (see that handoff).

## PoC coverage (passing)
- AC-02: `TestAC02_ConcurrentClaimantsSingleJob` runs 25 claimants × 20 rounds and expects exactly 1 lease. `TestAC02_ConcurrentClaimantsManyJobs` runs 20 claimants on 200 jobs and expects each job leased once. `TestAC02_FailpointsLeaveNoPartialWrites` injects failpoints at `claim.after_attempt_insert`, `claim.after_job_update`, `admit.after_job_insert` and `report.before_event`. `TestAC02_SchemaRejectsSecondActiveAttempt` checks the DB guard.
- AC-03: `TestAC03_StaleSuccessRejectedAfterExpiryAndReclaim` covers A expiring, B reclaiming at generation 2 and succeeding, then A rejected (`lease_expired`, then `job_terminal`). It also checks that B's result is kept, the attempts are `1:lease_expired,2:succeeded`, the event sequence, 2 diagnostic audit rows, and that the DB refuses an UPDATE of the terminal row.
- AC-06: `TestAC06_CancelThenSuccess` and `TestAC06_SuccessThenCancel` test each fixed order. `TestAC06_ConcurrentCancelVsSuccess` runs 100 racing iterations and expects exactly one terminal outcome and one terminal event each time.

## Security / compatibility review
- No secrets anywhere. Credentials, tokens and sessions are stored only as 32-byte SHA-256 hashes (CHECK `octet_length = 32`).
- Fencing is evaluated inside the CAS `UPDATE ... RETURNING` with row-count checks. Terminal jobs, finished attempts and the event/audit logs are guarded by DB triggers.
- READ COMMITTED re-check semantics are load-bearing for cancel vs success (ADR-0002).

## Known limits, defects, deferred scope
1. Full OpenAPI validation was NOT RUN in BE-01; see BE-02. The PoC proves the SQL patterns only, not the M2 service.
2. The PoC leaves out RenewLease, ReportFailure, drain, idempotent admission and manual retry. M2 must implement these from `state-machine.md`.
3. ADR-0004 retention for `log_tail` needs an exception to the finished-attempt trigger. It is deferred to M5 and not in 0001.
4. Unschedulable reasons are derived at read time, so there are no columns for them. Saturation is deliberately not counted as unschedulable.
5. The 1.0 limit of one manual-retry child per failed job is enforced by a unique index. Retrying the child again creates a chain.
6. Timing values in ADR-0003 are proposals until the pilot (OD-06).
7. pgx v5.7.5 was pinned by hand and confirmed by `go mod tidy`.
8. The claim query sorts all eligible queued rows: 6.6 ms at 20k queued (`claim-explain.txt`). Optimization is deferred to M4.

## Open questions
**For the frontend consumer (TASK-M1-FE-02 sign-off):**
- Is the masking shape OK: `null` plus `masked_fields[]`?
- Is a 31-day max and 7-day default window on the jobs list acceptable?
- Can the job detail page reconcile using `version` + `GET /jobs/{id}/events?after_version=`?
- The job-detail lineage field `retry_child_job_id` needs confirmation.
- Unschedulable reasons are an enum of 3. Is that enough UI copy?
- The §6 operation list has no endpoints for these. Which ones do you need (see `api-contracts.md` open items)?
  - Token issuance (`/app/settings/tokens`)
  - Worker credential issuance
  - WebSocket path
  - Handler registry listing for `/app/jobs/new`

**For the reviewer (TASK-M1-RV-01):**
- ADR-0002: is it right that a success report after a committed cancel request finalizes the job as `canceled`, as the cancel acknowledgment?
- EvalPlanQual under READ COMMITTED with `UPDATE … FROM` joins: the joined credential/worker rows are not re-locked. Is revocation bound = "committed before statement start" (ADR-0003) acceptable?
- Is cancel's `SELECT … FOR UPDATE` before the CAS right? It serializes with success through the row lock.
- Is the deferred composite FK `jobs(current_attempt_id, id) → job_attempts(id, job_id)` sound?

**For the integrator:** freeze `api/openapi.yaml` at M1 exit and decide on the four open contract items.

## Next owner / merge step
Integrator: pass to TASK-M1-RV-01 for review and TASK-M1-FE-02 for sign-off. The PoC passes.

## Requirement / gate disposition
FR-ENG-01..24, FR-API-01..04, FR-SEC-01..05, FR-SEC-11 and FR-RT-01 are **specified**. AC-02, AC-03 and AC-06 are **tested at PoC level** (passing under `-race` against PG 18.7 on a dev laptop; not in CI yet). MW-01 is ok: single owner, allowed paths only. MW-04 is partial: effort and Ultracode are unverified.
