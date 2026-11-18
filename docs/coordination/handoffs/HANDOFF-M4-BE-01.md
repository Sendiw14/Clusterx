# HANDOFF — TASK-M4-BE-01 (Engine: durable admission, atomic claims, registry, SDK, results) incl. CR-M4-01

- **Author:** backend lane subagent, model `claude-opus-5-5`, requested effort medium. Effective effort and Ultracode availability: UNVERIFIED from inside the subagent (MW-04).
- **Branch:** `m4/backend` (worktree `.worktrees/backend`), based on main `6cffb17`. Not pushed, not merged.
- **Contract base:** `contract-m1-draft.4` + CR-M4-01 → OpenAPI `1.0.0-draft.5`, migration `0002_m4_integrity_roles.sql`. `0001_init.sql` byte-identical (unchanged in every commit). Proto unchanged.
- **Requirements:** FR-ENG-01..09, 17, 18, 20, 21; FR-API-01 (M4 subset), FR-API-02; FR-SEC-03; FR-SEC-05; AC-01, AC-02, AC-07, AC-10, AC-18 (in-process), AC-20; CR-M4-01 NF-01..06, M3-1, M3-2.
- **Slices completed:** all six (1 migration/roles/store · 2 gRPC + scheduler · 3 worker/SDK transport + example handlers · 4 REST + tokens + CLI · 5 1,000-job multi-worker test · 6 curl guide executed against running binaries). Slice 5's test landed in the slice-3 commit.

## Commits

| SHA | Content |
|---|---|
| `7f7f5d6` | Migration 0002 (NF-02/03/04/05), `internal/store` (admission+idempotency, claim, renew/report/poll, register/heartbeat/drain/deregister, recovery, promotion, cancel, tokens, worker credentials, 40P01/40001 retry), config (migration vs runtime DSN, ADR-0003 timing bounds), docs (global lock order, NF-06 stale text, M3-1) |
| `7af4d14` | gofmt |
| `51f4142` | `internal/workerrpc` (all 9 RPCs), `internal/catalog`, `internal/scheduler`, `sdk.HandlerSpec`, `examples/basic-jobs` |
| `4df0425` | `sdk.Worker` transport, `clusterx-worker` runs basic-jobs, `internal/e2e` (AC-18, AC-04 preview, drain, cancel), Dockerfiles |
| `8fddad7` | `/api/v1` M4 operations, API-token auth, bootstrap CLI, production privileged-role refusal, OpenAPI draft.5 |
| `08633ae` | Fix found by new test: revoke/drain/deregister lock-then-stamp; revocation + lock-wait timing tests |
| (this) | this handoff |

## Changed / new files (by area)

- SQL: `apps/control-plane/migrations/0002_m4_integrity_roles.sql`.
- Store: `apps/control-plane/internal/store/{store,workers,jobs,lease,scheduler,tokens,reads}.go` + tests `{helpers,engine,lockorder}_test.go`.
- gRPC: `apps/control-plane/internal/workerrpc/server.go` (+ test rewritten for real DB).
- REST: `apps/control-plane/internal/api/{v1,canonical}.go`, `server.go`, `middleware.go` (+ `v1_test.go`).
- Other control-plane: `internal/catalog`, `internal/scheduler`, `internal/testenv`, `internal/e2e`, `internal/config`, `cmd/clusterx-control-plane/{main,admin}.go` (+ `admin_test.go`), `Dockerfile`, `go.mod/go.sum`, `internal/migrate/migrate_test.go` (counts now `len(migs)`).
- SDK: `packages/sdk-go/{worker,spec}.go`, `go.mod/go.sum` (now depends on proto, grpc, genproto rpc).
- Worker: `apps/worker/cmd/clusterx-worker/main.go`, `Dockerfile`, `go.mod/go.sum`.
- Examples: `examples/basic-jobs/{basicjobs.go,basicjobs_test.go,go.mod,go.sum}` (new module).
- Contracts/docs: `api/openapi.yaml` (draft.5), `docs/architecture/{state-machine.md,api-contracts.md}`, ADR-0002/0003/0004.

## CR-M4-01 disposition

| Item | Done | Evidence |
|---|---|---|
| NF-01 lock order | Global order `worker_credentials → workers → jobs → job_attempts` in state-machine.md + ADR-0003; worker mutations take credential/worker `FOR SHARE` then job `FOR UPDATE`; no share-locked identity row is written in the same tx (credential `last_used_at` bump is a separate statement); revoke/drain/deregister write only their row, recovery is by lease expiry; 40P01/40001 retried ≤3 runs then `unavailable`; `lock_timeout=1s` on worker mutations | `TestNF01_NoDeadlockAcrossWorkerPaths` (48 concurrent reports/renews + heartbeats + claims + recovery + drain/deregister/revoke, 3 rounds), `TestNF01_DeadlockRetryBounded`, `TestNF01_RealDeadlockIsRetried` (real 40P01 retried), `TestRevocationSerializesWithInFlightReport` |
| NF-02 | All worker/cancel/scheduler stamps are `clock_timestamp()` after locks; event/audit `occurred_at` default `clock_timestamp()`; `version` documented as authoritative | `TestNF02_TerminalAtAfterCancelRequested` (report tx starts before cancel, waits; asserts `terminal_at >= cancel_requested_at` and event time order agrees with version) |
| NF-03 | Finished-attempt trigger allows exactly `log_tail→NULL` + `log_expired false→true` on attempts finished ≥7 d ago; `clusterx_expire_attempt_logs(n)`; new columns `log_truncated`, `log_expired` | `TestNF03_LogRetention` (young rejected, other-column change rejected, aged expired, un-expire rejected) |
| NF-04 | Roles `clusterx_runtime` (SELECT/INSERT/UPDATE; DELETE only sessions, submission_idempotency, worker_capabilities) and `clusterx_retention`; `REVOKE ALL … FROM PUBLIC`; `jobs_retention_delete_only` trigger (terminal ≥30 d); BEFORE TRUNCATE triggers on jobs/attempts/events/audit/tombstones; `CLUSTERX_MIGRATE_DATABASE_URL` separate from runtime `CLUSTERX_DATABASE_URL`; production refuses to serve with a superuser / CREATE / DELETE-jobs runtime role | `TestNF04_PrivilegesAndDeleteGuards` (real login role in `clusterx_runtime`: DELETE jobs/events/attempts/audit, TRUNCATE, CREATE/ALTER/DROP TRIGGER, INSERT schema_migrations → 42501), curl-guide step 3 (superuser runtime refused, exit 1) |
| NF-05 | Deferred constraint triggers on `jobs` and `job_attempts`: at commit `running ⇔ exactly one active attempt = current_attempt_id` | `TestNF05_JobAttemptCoupling` (both directions rejected 23514; next claim batch succeeds) |
| NF-06 | N6/N12/N13 tests; stale text fixed (state-machine.md cancel/success paragraph, ADR-0002 consequences) | mutant table below |
| M3-1 | `schema_migrations` documented in api-contracts.md | existing runner tests green |
| M3-2 | `Error.code` += `not_found_route`, `method_not_allowed`; Go returns typed JSON (405 with `Allow`) | `TestV1AuthMatrix`; curl guide |

## Commands and exit codes (all on this branch, PG 18.7 throwaway cluster `localhost:55432`, Go 1.27.4, darwin/arm64)

| Command | Exit |
|---|---|
| `cd apps/control-plane && go vet ./...` | 0 |
| `cd apps/control-plane && CLUSTERX_PG_ADMIN_DSN=postgres://clusterx@localhost:55432/postgres go test -race -count=1 ./...` (after `08633ae`) | 0 — cmd, api, catalog, config, e2e, migrate, scheduler, store, workerrpc, poc all `ok` |
| same with `-count=3 ./internal/e2e/ ./internal/workerrpc/ ./internal/store/` (before `08633ae`) | 0 |
| `cd packages/sdk-go && GOWORK=off go vet ./... && GOWORK=off go test -race -count=1 ./...` | 0 |
| `cd apps/worker && GOWORK=off go vet ./... && GOWORK=off go test -race -count=1 ./...` | 0 |
| `cd examples/basic-jobs && GOWORK=off go vet ./... && GOWORK=off go test -race -count=1 ./...` | 0 |
| `cd packages/proto && go vet ./...` | 0 |
| `go build ./...` in workspace mode for control-plane/worker/sdk-go | 0 |
| `npx -y @redocly/cli@2.79.0 lint api/openapi.yaml` | 0 — valid, 0 errors, 5 warnings (the same 5 `operation-4xx-response`-class warnings as draft.4) |
| `docker build -q -f apps/control-plane/Dockerfile -t clusterx-control-plane:m4 .` | 0 (30.4 MB) |
| `docker build -q -f apps/worker/Dockerfile -t clusterx-worker:m4 .` | 0 (22.5 MB) |
| `gofmt -l apps packages/sdk-go examples` | empty |

## Mutant evidence (each mutant applied alone, test run, file restored)

| Mutant | Change | Exit | First failure |
|---|---|---|---|
| N6 | drop `case terminal && !credOK` in `classify` | 1 | `TestAC09_FenceDenials/N6_revoked_retry_of_committed_success` |
| N12 | `DROP TRIGGER job_attempts_finished_immutable` appended to 0002 | 1 | `TestN12_FinishedAttemptImmutable` |
| N13 | recovery never classifies `deadline_exceeded` | 1 | `TestN13_RecoveryClassification` |
| NF02-clock | canceled `terminal_at = now()` | 1 | `TestNF02_TerminalAtAfterCancelRequested` |
| NF05-jobs | jobs coupling trigger → `AFTER DELETE` | 1 | `TestNF05…: job left running with active attempt: <nil>` |
| NF05-attempts | attempts coupling trigger → `AFTER DELETE` | 1 | `TestNF05…: attempt finished while job running: <nil>` |
| NF01-retry | `retryable()` always false | 1 | `TestNF01_DeadlockRetryBounded` |
| fence-cred | fence ignores presented credential (`w.credential_id = $6` removed) | 1 | `TestAC09_FenceDenials/other_credential_for_worker` |
| M-credlock | drop credential `FOR SHARE` in `lockForWorkerMutation` | 1 | `TestRevocationSerializesWithInFlightReport: revocation did not wait` |
| M-joblock | drop job `FOR UPDATE` | 1 | `TestLeaseJudgedAfterLockWait: report after lease expiry = <nil>` |
| M-fenceclock | fence lease clause `clock_timestamp()` → `now()` | 1 | same |
| M-skiplocked | claim without `FOR UPDATE SKIP LOCKED` | 1 | `TestAC02_ConcurrentClaimantsSingleJob: claim CAS … no rows` (claim aborts, never double-leases) |
| M-revoke-lockfirst | revoke without lock-before-stamp | 1 | `TestRevocation…: revocation ordered before the accepted report` (this found the real bug fixed in `08633ae`) |
| M-cancelcheck | success CAS without `cancel_requested_at IS NULL` | 1 | `TestAC06_CancelVersusSuccess` (DB CHECK rejects; test fails) |
| M-budget | claim CAS without `attempts_used < max_attempts` | 0 | **survived — equivalent**: the locked candidate scan has the same predicate and CHECK `attempts_used <= max_attempts` backs it |

## Acceptance coverage

| AC | Status | Test / evidence |
|---|---|---|
| AC-01 | tested | `TestAC01_AckOnlyAfterCommitAndRestart` (failure at `admit.before_commit` → 503 + Retry-After, 0 rows; generic failure → 500; new pool/handler after "restart" reads the job; replay after restart 200). Real-binary restart in curl guide (job persisted across SIGTERM + restart, then succeeded). Durability under PG crash: NOT RUN |
| AC-02 | tested | store: `TestAC02_ConcurrentClaimantsSingleJob` (24 claimants × 5 rounds), `TestAC02_FailpointsLeaveNoPartialWrites` (admit ×2, claim ×3, report failpoints); service level: `TestAC02_TwentyFourGRPCClaimants` (24 gRPC workers, 200 jobs, each leased exactly once) |
| AC-04 | preview tested | `TestAC04_WorkerLostMidJobRecovers` (connection killed mid-handler → `lease_expired` attempt → second worker succeeds). Real process kill -9: NOT RUN |
| AC-05 | tested | `TestAC05_BudgetExactlyThree` (no 4th claim; injected backoff values) |
| AC-06 | tested | `TestAC06_CancelVersusSuccess` (both orders + 30 concurrent races), `TestCancelRunningJob` (e2e worker ack) |
| AC-07 | tested | `TestAC07_Idempotency` (10 concurrent replays, conflict with job_id, other principal scope, expiry reuse), HTTP replay with reordered keys in `TestV1CreateValidationAndIdempotency`, curl guide |
| AC-09 | tested (worker side) | `TestAC09_FenceDenials` (lease, deadline, incarnation, deregistered, revoked, N6, other credential, stale generation), `TestAuthAndLimits` (gRPC unauthenticated/garbage/unknown/other worker), `TestRegisterBinding` |
| AC-10 | tested | `TestAC10_PriorityDueUnschedulable` (9 before 0, future `run_after` not claimed, `no_capable_worker`/`capable_workers_draining`/`capable_workers_offline`, unknown handler), HTTP `unschedulable_reason` in `TestV1Reads…` |
| AC-11 | partial | `TestGracefulDrain` (short handler finishes, long one canceled after grace → `worker_shutdown` retry, worker deregistered), real-binary SIGTERM in curl guide; operator drain REST is M5 |
| AC-12 | partial | body/payload/result/depth/NUL bounds, handler schemas reject unknown fields (`cmd`, `url`) — tests above; no adversarial fuzzing: NOT RUN |
| AC-13 | partial (tokens only) | `TestV1AuthMatrix` (401 reasons, 403 viewer/scope/demoted owner, forged headers ignored), masking for viewer tokens. BFF assertions/sessions/CSRF/WebSocket: NOT RUN (M5) |
| AC-18 | tested in-process | `TestAC18_ThousandJobsThreeWorkers`, raw output: `submitted=1000 terminal=1000 succeeded=1000 attempts=1250 retries=250 active_attempts=0 attempts_per_worker=[411,417,422] submit_wall=309ms total_wall=868ms` (laptop, 10 ms backoff base; not a throughput claim). Three separate OS processes / docker compose: NOT RUN |
| AC-20 | partial | proto codegen unchanged; Go server + SDK client compiled from the same generated package; OpenAPI lint 0 errors. TS client regeneration: NOT RUN (integrator) |
| AC-03, AC-08 (DB outage), AC-17, AC-19 | NOT RUN in M4 | AC-03 shape covered by fence tests + PoC `TestAC03*`; scaling/EXPLAIN at 100k (RV01-15 re-measure) NOT RUN |

## Bootstrap commands (FR-SEC-03/05; no default credentials)

```
clusterx-control-plane migrate                    # uses CLUSTERX_MIGRATE_DATABASE_URL (schema owner) if set
psql … -c "CREATE ROLE clusterx_app LOGIN PASSWORD '<generated>' IN ROLE clusterx_runtime"   # out of band
clusterx-control-plane token create --role operator --name ops --owner-github-id <N> --owner-login <login> [--scopes a,b] [--expires-in-days 90]
clusterx-control-plane token revoke --id <uuid>
clusterx-control-plane worker-credential create --name worker-1 [--expires-in-days 0]
clusterx-control-plane worker-credential revoke --id <uuid>
```
Each create prints `id, prefix, …, secret=` once on stdout; only SHA-256 is stored; the owner user is created (allowlisted GitHub numeric id, given role) only if absent.

## Curl guide — executed 2026-11-18 against the built binaries (production mode, loopback)

Setup actually run: fresh DB `clusterx_demo`; `clusterx-control-plane migrate` (exit 0, `applied=2`); `GRANT clusterx_runtime TO clusterx_app`; serve with `CLUSTERX_DATABASE_URL=<superuser>` → exit 1 `runtime database role is superuser or may CREATE…`; tokens and 2 worker credentials created (exit 0); control plane serve with `CLUSTERX_ENV=production CLUSTERX_HTTP_ADDR=127.0.0.1:18080 CLUSTERX_GRPC_ADDR=127.0.0.1:19090 CLUSTERX_METRICS_ADDR=127.0.0.1:19091 CLUSTERX_DATABASE_URL=postgres://clusterx_app@…/clusterx_demo CLUSTERX_LEASE_TTL=5s CLUSTERX_HEARTBEAT_INTERVAL=2s CLUSTERX_LIVENESS_TIMEOUT=6s CLUSTERX_METRICS_TOKEN=…`; two `clusterx-worker` processes (`CLUSTERX_WORKER_INSECURE=true` loopback, capacity 4). `$OP`/`$VIEWER` are the printed secrets.

| # | Command | Result |
|---|---|---|
| 1 | `curl 127.0.0.1:18080/health/live` / `/health/ready` | 200 `{"status":"ok"}` / 200 `{"status":"ready"}` |
| 2 | `curl -X POST …/api/v1/jobs -H "Authorization: Bearer $OP" -H 'Content-Type: application/json' -H 'Idempotency-Key: guide-1' -d '{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"hello"},"priority":5}'` | 201, `status":"queued"`, `Location` |
| 3 | same key, keys reordered | 200 `Idempotent-Replayed: true`, same id |
| 4 | same key, `"data":"other"` | 409 `idempotency_conflict`, `details.job_id` = original |
| 5 | POST `fail_n_times {"n":1}` | 201 |
| 6 | POST `sleep_ms {"ms":"x","cmd":"rm -rf /"}` | 422 `/payload/cmd` + `/payload/ms` rule `schema` |
| 7 | POST with `$VIEWER` | 403 `forbidden` `{"required":"jobs:write","role":"viewer"}` |
| 8 | POST without token | 401 `unauthenticated` `reason: token_invalid` |
| 9 | `GET /api/v1/jobs/<id1>` as operator / viewer | 200 `succeeded`, `attempts_used 1`; viewer: payload/result null, `masked_fields` |
| 10 | `GET /api/v1/jobs/<id2>/attempts` | attempt 1 `failed` `retryable` `planned_failure`, attempt 2 `succeeded` |
| 11 | `GET /api/v1/jobs/<id2>/events?after_version=0` (viewer) | admitted, claimed, failed_retryable, promoted, claimed, succeeded |
| 12 | `GET /api/v1/jobs?status=succeeded&sort=-priority&limit=1` | 200 + `next_cursor` |
| 13 | `GET /api/v1/workers?liveness=live` (viewer) | 2 live workers, `hostname: null` (masked) |
| 14 | `GET /api/v1/handlers`, `GET /api/v1/config` | 200; config `api_version 1.0.0-draft.5`, `build.commit 8fddad7` |
| 15 | `POST /api/v1/jobs/<id>/cancel` | 404 `not_found_route` (M5 route, deliberately absent) |
| 16 | `DELETE /api/v1/jobs` | 405 `method_not_allowed`, `Allow: GET, HEAD, POST` |
| 17 | `GET /api/v1/jobs/<unknown uuid>` | 404 `job_not_found` |
| 18 | `curl 127.0.0.1:19091/metrics` (no token) | 401 |
| 19 | POST `sleep_ms {"ms":20000}`; `kill -TERM` both workers (`CLUSTERX_SHUTDOWN_TIMEOUT=5s`) | both exit; worker-1 logs `drain grace elapsed; canceling running handlers`, both `deregistered`; attempt `failed` `worker_shutdown`; workers `deregistered` |
| 20 | `kill -TERM` control plane, restart it | job still present (`queued` after promotion); new worker with credential 1 → job `succeeded`, `attempts_used 2` |

`grep -ci 'secret\|cxt_\|cxw_'` over control-plane, worker and curl logs: 0 (no credential in logs). All processes stopped afterwards.

## Limitations / open issues

1. **Contract gaps (need integrator/CR):** gRPC reason `invalid_argument` (malformed requests, invalid result JSON) is not in the state-machine reason list; a valid credential naming another credential's worker answers `UNAUTHENTICATED/credential_revoked` with detail "credential is not bound to this worker" — propose a dedicated reason (e.g. `credential_mismatch`) in the next CR. Recovery (E8) with budget exhausted emits event `lease_expired`/`deadline_exceeded` with `to_status=failed` (not `attempts_exhausted`); confirm.
2. **go.work:** `examples/basic-jobs` is a new module not in root `go.work` (root config is integrator-owned). Workspace builds work through per-module `replace`; `go test ./...` from `examples/basic-jobs` needs `GOWORK=off` until the integrator adds `./examples/basic-jobs` to `go.work` (and CI).
3. **Catalogue:** the coordinator's compiled allowlist is `examples/basic-jobs.Specs()`; payload schemas are enforced with a documented JSON-Schema subset (unsupported keywords rejected at start). A deployment-specific catalogue mechanism is future work.
4. **Auth:** only direct API tokens in M4; BFF service assertions and sessions are M5 (any non-`cxt_` bearer → 401 `token_invalid`). No rate limiting (429 never returned). `last_used_at` is throttled to once/60 s.
5. **Roles:** migration 0002 needs CREATE ROLE (or pre-created group roles); login users are created out of band. `REVOKE ALL ON ALL TABLES FROM PUBLIC` applies to existing tables only.
6. **Claim:** per-capability top-N with `FOR UPDATE SKIP LOCKED` may briefly lock up to N rows per capability that are not taken; no strict FIFO. RV01-15 re-measure at ≥100k queued: NOT RUN.
7. **Register policy:** a credential bound to a live worker is refused a new incarnation (`incarnation_mismatch`) unless `worker_id` is given; the reference worker does not persist its id, so a fast restart waits up to the liveness timeout (SDK retries with backoff; observed in the guide only as normal behavior since the first worker had stopped).
8. **SDK:** handler log capture is not implemented (`log_tail` empty). A lease-lost handler's outcome is discarded without a report.
9. `build.commit` defaults to `0000000` unless set via `-ldflags -X main.commit=…`.
10. PoC package (`poc/`) still tests migration 0001 only, unchanged.

## Integration needs

- Regenerate `packages/sdk-ts` from `api/openapi.yaml` 1.0.0-draft.5 (integrator).
- Add `./examples/basic-jobs` to `go.work` and CI module list; CI needs `CLUSTERX_PG_ADMIN_DSN` (superuser on a throwaway PG 18) for integration tests and CREATE ROLE rights for migration 0002.
- `.env.example`/compose: new vars `CLUSTERX_MIGRATE_DATABASE_URL`, `CLUSTERX_HEARTBEAT_INTERVAL`, `CLUSTERX_LIVENESS_TIMEOUT`, `CLUSTERX_LEASE_TTL`, `CLUSTERX_CANCEL_GRACE`, `CLUSTERX_DRAIN_GRACE`, `CLUSTERX_SCHEDULER_TICK`, `CLUSTERX_BACKOFF_BASE`, `CLUSTERX_BACKOFF_CAP`; runtime login role in `clusterx_runtime`.
- Critical review (M5 security pass): lock order/revocation bound, Register binding policy, fence with presented credential, migration 0002 grants and triggers.
