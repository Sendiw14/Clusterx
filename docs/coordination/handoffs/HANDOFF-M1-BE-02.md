# HANDOFF — TASK-M1-BE-02 (CCR-M1 implementation) + BE-01 gate follow-up

- **Author, actual model/effort, Ultracode availability:** backend lane subagent, model `claude-opus-5-5`. Requested effort: medium. Effective effort and Ultracode: UNVERIFIED (MW-04).
- **Branch:** `m1/backend` (worktree `.worktrees/backend`). Not pushed, not merged.
- **Source decisions:** `main:docs/coordination/CCR-M1-decisions.md` @ `969bc30`.

## Commits (this follow-up)
| SHA | Content |
|---|---|
| `08f30ab` | `go mod tidy`, go.sum |
| `060383f` | claim EXPLAIN evidence (`docs/architecture/evidence/claim-explain.txt`) |
| `096a47e` | HANDOFF-M1-BE-01 updated with real results |
| `e0d04e8` | DDL: `jobs.failure_class` (+2 CHECKs), `workers.version`, `job_tombstones`; PoC sets failure_class; new test `TestBudgetExhaustedNoHiddenAttempt` |
| `7f02634` | OpenAPI `1.0.0-draft.2`: CCR-01/02/03/04/07, field additions; redocly-valid |
| `f760f04` | ADR-0006, state-machine (failure_class, 409 details, 410), permissions, api-contracts, ADR-0004/0005 touch-ups |
| (this) | this handoff |

## CCR implementation map
| Decision | Where |
|---|---|
| CCR-01 tokens / worker credentials | `GET/POST /api/v1/tokens`, `POST /tokens/{id}/revoke`, same for `/worker-credentials`. Secret appears only in the 201 body (`no-store`). Session-only: API tokens get 403. Audited. |
| CCR-02 handler catalogue | `GET /api/v1/handlers` returns versions, `payload_schema`, `capable_workers`, `live_capable_workers` |
| CCR-03 realtime | `GET /api/v1/stream`; `StreamMessage` = hello / hint `{entity,id,version}` / resync / auth_expiring; close codes 4401/4403/4408/4429/1001/1008/1012; reached through the BFF (ADR-0006 §4) |
| CCR-04 config | `GET /api/v1/config` returns limits, retention and timing (no secrets) |
| CCR-05/06 deferred | `/audit` filters renamed to `subject_type` / `subject_id` (interim) |
| CCR-07 | Attempt gets `log_tail`, `log_truncated`, `log_expired`, `lease_expires_at` |
| CCR-08 deferred | nothing added |
| CCR-09 | `docs/architecture/adrs/0006-bff-session-contract.md` |
| Field additions | `masked_fields` (already present); `retried_as_job_id`; `unschedulable_reason` + `unschedulable_since` + repeatable filter; `Idempotent-Replayed: true` + 200; **410 `expired`** on every job-addressed op, backed by `job_tombstones`; post-state `{status, version}` in 409 details; worker sort `(last_seen_at DESC, id DESC)` with `workers.version` |
| OQ-08 | `failure_class` column + CHECKs + table in state-machine.md |
| OQ-09 | renewals create no events; `lease_expires_at` on the active attempt |

## Commands, exit codes, results
| Check | Command | Exit | Result |
|---|---|---|---|
| Cluster init | `initdb -D <scratchpad>/pg18data -U clusterx --auth=trust` | 0 | ok |
| Cluster start | `LC_ALL=en_US.UTF-8 pg_ctl -D … -o "-p 55432 -k '' -c listen_addresses=localhost" -w start` | 0 | TCP localhost only. A Unix socket was impossible because the scratchpad path exceeds the 103-byte limit. |
| DDL (with CCR changes) | `dropdb/createdb ddlcheck && psql -h localhost -p 55432 -U clusterx -d ddlcheck -v ON_ERROR_STOP=1 --single-transaction -q -f migrations/0001_init.sql` | 0 | **PASS** |
| Vet | `go vet ./...` | 0 | **PASS** |
| Race ×3 | `CLUSTERX_PG_ADMIN_DSN='postgres://clusterx@localhost:55432/postgres?sslmode=disable' go test -race -count=3 -v ./poc/...` | 0 | **PASS**: all 9 tests ×3; `ok …/poc 6.329s`; AC-06 orderings `cancel-first:94/95/97, success-first:6/5/3` |
| OpenAPI parse | PyYAML: `yaml ok; paths: 22 ops: 25 refs: 46 unresolved: []` | 0 | **PASS** |
| OpenAPI lint | `npx --yes @redocly/cli@2.79.0 lint api/openapi.yaml` | 0 | **PASS**: "Your API description is valid", 0 errors, 5 warnings (see below) |
| Proto | `protoc -I . -I /opt/anaconda3/include --descriptor_set_out=/dev/null clusterx/worker/v1/worker.proto` | 0 | **PASS** (unchanged) |
| Cluster stop | `pg_ctl -D … -m fast stop` | 0 | stopped; nothing is listening on 55432 |

The first redocly run exited 1 with **8 struct errors**. Unquoted commas inside flow-mapping `description:` values had
silently split them into stray keys. PyYAML parsed this without complaint, so the BE-01 "YAML parse PASS" did not
catch it. All were fixed by quoting, and lint was re-run until it exited 0.

The 5 warnings left are intended:
- `info-license` (OD-02 pending)
- `no-server-example.com` (localhost dev server)
- `operation-2xx-response` (the WebSocket returns 101)
- 2× `operation-4xx-response` (`/health/*` are unauthenticated by design)

## NOT RUN / known limits
- TS client generation from draft.2 was NOT RUN (integrator, M3).
- The PoC still does not exercise RenewLease, ReportFailure (retryable/permanent classes), drain, idempotent
  admission, manual retry or tombstones. `failure_class` `retryable_exhausted`/`permanent`/`timeout` are covered only by the DB CHECK.
- The claim query sorts all eligible queued rows (6.6 ms at 20k queued). Optimization is deferred to M4.
- `GET /stream` described in OpenAPI 3.1 is documentation only. The message schema is attached to the 101 response.
- The `last_seen_at` keyset sort uses a mutable key, so workers can move between pages. Clients de-duplicate by id (documented).

## Open questions
- **Integrator: CCR-10 (blocking real login, M6).** Go owns `users`/`sessions`, but there is no op for the BFF to create
  or revoke a session or read the effective role. Proposal in ADR-0006 §5:
  - `POST /api/v1/sessions`
  - `DELETE /api/v1/sessions/current`
  - `GET /api/v1/me`
  
  These are not added to OpenAPI.
- **Owner:**
  - OQ-03: may viewers read `/audit` (privileged rows only, details masked, actor as principal ref)?
  - ADR-0006 session lifetimes: idle 2 h, absolute 12 h. Are these acceptable?
- **Frontend (FE-02):**
  - Confirm the 401 `details.reason` mapping (ADR-0006 §2).
  - Confirm the BFF-owned codes `upstream_auth` (502) and `csrf_failed` (403).
  - Confirm `/bff/session` shape and `/bff/stream`.
- **Reviewer (RV-01):**
  - `jti` single-use cache is in-memory (single coordinator).
  - Session DB re-check on every request.
  - Stream session re-check ≤ 30 s.
  - Tokens/credentials are session-only (API tokens get 403).

## Next owner
Integrator: freeze `api/openapi.yaml` draft.2, decide CCR-10, then route to RV-01 and FE-02.
