# HANDOFF — TASK-M1-BE-01

- **Author, actual model/effort, Ultracode availability:** backend lane subagent. Model `claude-opus-5-5` (Opus 5.5, from the session system prompt). Requested effort: medium (Agent param). Effective effort and Ultracode were **not observable** from inside the subagent: UNVERIFIED (MW-04, R-12).
- **Branch, commits, changed files:** `m1/backend` (worktree `.worktrees/backend`), base `5a5ca99`.
  - `6c41f4e` docs(arch): ADRs 0001–0005 and normative job state machine
  - `85eef97` feat(proto): worker gRPC protocol v1 draft
  - `397e339` feat(api): OpenAPI 3.1 draft for /api/v1 with typed errors, pagination, masking
  - `b7953b5` docs(security): permissions matrix, trust boundaries, architecture overview
  - `d713679` feat(db): draft migration 0001 with lifecycle constraints and indexes (unvalidated)
  - `ad9df49` test(poc): race PoC for AC-02/03/06 against PostgreSQL 18 (not yet run)
  - plus the commit that adds this handoff
  - Files: `docs/architecture/adrs/000{1..5}-*.md`, `docs/architecture/{state-machine,overview,api-contracts}.md`, `docs/security/{permissions,trust-boundaries}.md`, `apps/control-plane/{go.mod,migrations/0001_init.sql,poc/README.md,poc/store.go,poc/store_test.go}`, `packages/proto/clusterx/worker/v1/worker.proto`, `api/openapi.yaml` (temporary ownership transfer), this file. Nothing outside the allowed paths.
- **Contract SHA, generated code, migration impact:** source contract = master R2 @ `5a5ca99`. No generated code. Migration 0001 is the initial schema (no existing data).
- **Implemented/Integrated:** all 7 deliverables are written (specified/proposed). Nothing is integrated. The PoC is written but **has not been compiled or run**.

## Commands, exit codes, results

| Check | Command | Exit | Result |
|---|---|---|---|
| Proto compiles | `cd packages/proto && protoc -I . -I /opt/anaconda3/include --descriptor_set_out=/dev/null clusterx/worker/v1/worker.proto` (libprotoc 29.3) | 0 | **PASS** |
| OpenAPI parses as YAML | `python3 -c 'import yaml; d=yaml.safe_load(open("api/openapi.yaml")); …'` (PyYAML 6.0.3) → `yaml ok; paths: 15 ops: 16` | 0 | **PASS** |
| OpenAPI `$ref`s resolve + every `/api/v1` op has 401/403/429/503 | inline python3 script over the parsed YAML → `refs 35 unresolved []`, no missing codes | 0 | **PASS** |
| OpenAPI full schema validation | — (no `openapi-spec-validator`/redocly installed) | — | **NOT RUN** |
| DDL applies on PG18 | `psql -v ON_ERROR_STOP=1 --single-transaction -f apps/control-plane/migrations/0001_init.sql` | — | **NOT RUN**: PostgreSQL 18 was not installed |
| PoC `go test -race` | `CLUSTERX_PG_ADMIN_DSN=… go test -race -count=1 -v ./poc/` in `apps/control-plane` | — | **NOT RUN**: Go was not installed; `go.sum` not generated |

Toolchain status: polled every 60 s for 15 min (19:51–20:06 +0530). At 20:06 the `postgresql@18` 18.7 bottle had downloaded but `/opt/homebrew/opt/postgresql@18/bin/pg_ctl` did not exist. The Go 1.27.4 bottle was still downloading (about 32 MB). No throwaway cluster was created or started.

**To finish the acceptance checks (next owner, once brew finishes):**
```
SP=/private/tmp/claude-501/-Users-sendiwkithnada-Documents-Clusterx/972024f7-8f0d-4184-982f-9b74a395640b/scratchpad
PGB=/opt/homebrew/opt/postgresql@18/bin
$PGB/initdb -D $SP/pg18data -U clusterx --auth=trust
$PGB/pg_ctl -D $SP/pg18data -o "-p 55432 -k $SP -c listen_addresses=localhost" -l $SP/pg18.log start
$PGB/createdb -h $SP -p 55432 -U clusterx ddlcheck
$PGB/psql -h $SP -p 55432 -U clusterx -d ddlcheck -v ON_ERROR_STOP=1 --single-transaction -f apps/control-plane/migrations/0001_init.sql
cd apps/control-plane && go mod tidy && go vet ./... && \
  CLUSTERX_PG_ADMIN_DSN="postgres://clusterx@localhost:55432/postgres?host=$SP" go test -race -count=1 -v ./poc/
$PGB/pg_ctl -D $SP/pg18data stop
```
Expect first-run fixes. The code has never been compiled. Likely spots: pgx `time.Duration`→`interval` encoding, and `uuid`→`string` scanning.

## PoC coverage (written, not run)
- AC-02: `TestAC02_ConcurrentClaimantsSingleJob` runs 25 claimants × 20 rounds and expects exactly 1 lease. `TestAC02_ConcurrentClaimantsManyJobs` runs 20 claimants on 200 jobs and expects each job leased once. `TestAC02_FailpointsLeaveNoPartialWrites` injects failpoints at `claim.after_attempt_insert`, `claim.after_job_update`, `admit.after_job_insert` and `report.before_event`. `TestAC02_SchemaRejectsSecondActiveAttempt` checks the DB guard.
- AC-03: `TestAC03_StaleSuccessRejectedAfterExpiryAndReclaim` covers A expiring, B reclaiming at generation 2 and succeeding, then A rejected (`lease_expired`, then `job_terminal`). It also checks that B's result is kept, the attempts are `1:lease_expired,2:succeeded`, the event sequence, 2 diagnostic audit rows, and that the DB refuses an UPDATE of the terminal row.
- AC-06: `TestAC06_CancelThenSuccess` and `TestAC06_SuccessThenCancel` test each fixed order. `TestAC06_ConcurrentCancelVsSuccess` runs 100 racing iterations and expects exactly one terminal outcome and one terminal event each time.

## Security / compatibility review
- No secrets anywhere. Credentials, tokens and sessions are stored only as 32-byte SHA-256 hashes (CHECK `octet_length = 32`).
- Fencing is evaluated inside the CAS `UPDATE ... RETURNING` with row-count checks. Terminal jobs, finished attempts and the event/audit logs are guarded by DB triggers.
- READ COMMITTED re-check semantics are load-bearing for cancel vs success (ADR-0002).

## Known limits, defects, deferred scope
1. DDL, PoC and full OpenAPI validation are **NOT RUN**, so the AC-02/03/06 gates for M1 are **open**.
2. The PoC leaves out RenewLease, ReportFailure, drain, idempotent admission and manual retry. M2 must implement these from `state-machine.md`.
3. ADR-0004 retention for `log_tail` needs an exception to the finished-attempt trigger. It is deferred to M5 and not in 0001.
4. Unschedulable reasons are derived at read time, so there are no columns for them. Saturation is deliberately not counted as unschedulable.
5. The 1.0 limit of one manual-retry child per failed job is enforced by a unique index. Retrying the child again creates a chain.
6. Timing values in ADR-0003 are proposals until the pilot (OD-06).
7. `go.mod` pins `pgx/v5 v5.7.5` by hand and has no `go.sum`.
8. `EXPLAIN (ANALYZE, BUFFERS)` index evidence: NOT RUN (needs seeded data).

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
Integrator: finish the NOT RUN checks above, then pass to TASK-M1-RV-01 for review and TASK-M1-FE-02 for sign-off. Do not merge to main until the PoC passes under `-race` on PG18.

## Requirement / gate disposition
FR-ENG-01..24, FR-API-01..04, FR-SEC-01..05, FR-SEC-11 and FR-RT-01 are **specified**. AC-02, AC-03 and AC-06 have tests **written but not run**. MW-01 is ok: single owner, allowed paths only. MW-04 is partial: effort and Ultracode are unverified.
