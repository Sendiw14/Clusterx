# HANDOFF — TASK-M5-BE-01 (reliability & security: mutations, sessions/BFF assertions, stream, audit, metrics, outage tests) incl. CR-M5-01

- **Author:** backend lane subagent, model `claude-opus-5-5`, requested effort medium. Effective effort and Ultracode availability: UNVERIFIED from inside the subagent (MW-04).
- **Branch:** `m5/backend` (worktree `.worktrees/backend`), base main `aecd77e`. Not pushed, not merged.
- **Contract:** OpenAPI `1.0.0-draft.6` (additive over draft.5), migration `0003_m5_metrics_indexes.sql` (indexes only; 0001/0002 byte-identical), proto unchanged.
- **Requirements:** FR-ENG-12, 13, 14, 15, 16, 19, 22, 23; FR-API-01 (all operations now routed); FR-SEC-01..07, 11; FR-RT-01 (server); AC-03, AC-04, AC-06, AC-08, AC-09, AC-11, AC-12 (rate limits), AC-13, AC-16 (server), AC-17 (retention part).
- **Slices completed:** all (CR items → mutations+audit → tokens/creds REST → sessions/assertions → stream → metrics → rate limit → retention → outage/kill -9 tests → compose check). Session work landed before mutations because token/credential REST needs session principals.

## Commits

| SHA | Content |
|---|---|
| `6dbd6dd` | CR-M5-01: `credential_mismatch`, `attempts_exhausted` on recovery, worker id persistence, SDK `log_tail` capture, state-machine.md reasons |
| `b536f68` | `internal/auth` (Ed25519 JWS), sessions/me, cancel/retry/drain, tokens + worker-credentials REST, audit, metrics, migration 0003, WebSocket stream + DB change feed, rate limits, retention, dev CLI, user CLI |
| `8c0fd29` | Retention test (real `clusterx_retention` login), AC-03 over gRPC, AC-11 REST drain, AC-06 REST cancel; SDK exits cleanly after an operator drain |
| `8ffbe41` | Process-level chaos: AC-04 `kill -9`, AC-08 PostgreSQL outage, AC-11 SIGTERM (real binaries) |
| `8b06dcc` | AC-13 route × principal matrix |
| `bcee061` | OpenAPI draft.6, ADR-0004/0006 implementation notes |
| `e6a6068` | AC-09 gRPC identity matrix |
| (this) | this handoff |

## Changed / new files

- Control plane: `cmd/clusterx-control-plane/{main,admin,dev}.go`; `internal/auth/{assertion,mint,assertion_test}.go` (new);
  `internal/api/{auth,ratelimit,mutations,sessions,reads_m5,hub,stream}.go` (new), `v1.go` (routes, guard, error mapping);
  tests `internal/api/{m5,stream,hub_internal}_test.go`, `v1_test.go`; `internal/store/{sessions,audit,mutations,credentials,metrics,retention}.go` (new),
  `tokens.go`, `workers.go`, `lease.go`, `scheduler.go`; tests `retention_test.go`, `engine_test.go`; `internal/config/config.go`;
  `internal/workerrpc/server.go` + `{ac03,ac09}_test.go`, `server_test.go`; `internal/e2e/{e2e,m5}_test.go`; `internal/chaos/chaos_test.go` (new);
  `internal/migrate/migrate_test.go`; `migrations/0003_m5_metrics_indexes.sql`; `go.mod/go.sum` (+ `github.com/coder/websocket v1.8.14`).
- SDK: `packages/sdk-go/{logcap,logcap_test}.go` (new), `worker.go`, `handler.go` (`Job.Log`). Worker: `apps/worker/cmd/clusterx-worker/main.go`, `internal/config/config.go(+_test)`. Example: `examples/basic-jobs/basicjobs.go` (logs through `job.Log`).
- Contracts/docs: `api/openapi.yaml` (draft.6), `docs/architecture/state-machine.md`, ADR-0004, ADR-0006.

## CR-M5-01 disposition

| # | Done | Evidence |
|---|---|---|
| 1 `invalid_argument` in reason list | state-machine.md worker-side table | existing gRPC tests assert it |
| 2 `credential_mismatch` (PERMISSION_DENIED) | store `identityDenial`, `classify` (credential match split from validity), Register re-bind; security audit category | `TestAC09_FenceDenials/other_credential_for_worker` (report, renew, heartbeat, register), `TestAuthAndLimits`, `TestAC09_GRPCIdentityMatrix`; mutant M5-credmismatch |
| 3 `attempts_exhausted` on recovery | event `attempts_exhausted`, `data.failure_class` (`lease_expired`/`timeout`), `data.attempt_state`; attempt row keeps `lease_expired`/`deadline_exceeded`; `JobEventData.attempt_state` (draft.6) | `TestN13_RecoveryClassification` (event assertions added) |
| 4 worker id persistence | `CLUSTERX_WORKER_ID` or `CLUSTERX_WORKER_ID_FILE` (atomic temp+rename; malformed file fails closed); SDK `OnRegistered`; stored id of another credential ⇒ one fresh registration | `TestWorkerIDFile`; `TestAC04_KillDashNineWorkerProcess` (restart registered < 3 s with liveness 30 s, same id) |
| 5 SDK `log_tail` | `Job.Log` (text, redacts secret-like attribute keys, tee to worker log), keeps last 17 KiB so the coordinator's 16 KiB cut sets `log_truncated` | `TestAttemptLoggerBoundedAndRedacted`; AC-18 now asserts 750 captured tails; compose step 8; mutant M5-logredact |

## Commands and exit codes (this branch, PG 18.7 throwaway cluster `localhost:55432`, Go 1.27.4, darwin/arm64)

| Command | Exit |
|---|---|
| `cd apps/control-plane && go vet ./...` | 0 |
| `cd apps/control-plane && CLUSTERX_PG_ADMIN_DSN=postgres://clusterx@localhost:55432/postgres go test -race -count=1 ./...` (after `e6a6068`) | 0 — cmd, api, auth, catalog, chaos, config, e2e, migrate, scheduler, store, workerrpc, poc all `ok` (chaos also needs `initdb`/`pg_ctl` on PATH) |
| `GOWORK=off go vet ./... && go test -race -count=1 ./...` in `packages/sdk-go`, `apps/worker`, `examples/basic-jobs`, `packages/proto` | 0 each |
| `gofmt -l apps packages examples` | empty |
| `npx -y @redocly/cli@2.79.0 lint api/openapi.yaml` | 0 — valid, 0 errors, 5 warnings (same 5 as draft.5) |
| Mutant script (26 mutants, each applied alone, file restored) | table below |
| `COMPOSE_PROJECT_NAME=clusterx-m5 COMPOSE_FILE=compose.yaml:<scratch override> ./infra/compose/bootstrap.sh` on a clean volume | 0 (second run; first run exit 1, see Limitations 9) |
| `docker compose … down -v` (clusterx-m5) | 0; `.env`/`.secrets.env` deleted; m5 images removed |

Important: without `CLUSTERX_PG_ADMIN_DSN` the DB tests **skip** (they report `ok`); every result above was taken with it set (verified with `-v`).

## Acceptance coverage

| AC | Status | Test / evidence |
|---|---|---|
| AC-03 | tested (service level) | `TestAC03_StaleSuccessAfterExpiryAndReclaim` (real gRPC + scheduler tick): A expires, B claims gen+1 and succeeds; A's late success/renew/failure all denied (`job_terminal`: A's own attempt on a final job, per state-machine E5), B's result kept, events `admitted,claimed,lease_expired,promoted,claimed,succeeded`, version 6 = 6 events, 3 rejection audit rows |
| AC-04 | tested with real `kill -9` | `TestAC04_KillDashNineWorkerProcess`: worker binary SIGKILLed mid-handler (exit status checked), attempts `1:lease_expired,2:succeeded`; raw: `recovered_and_succeeded_after=9.361s` (lease 5 s, sleep 4 s) |
| AC-06 | tested via REST | `TestAC06_RESTCancelVersusSuccess` (SDK worker, 202 then canceled, result NULL), `TestM5_CancelRetryDrain` (late success after REST cancel ⇒ `OUTCOME_CANCELED`), M4 race tests unchanged |
| AC-08 | tested | `TestAC08_DatabaseOutageMidRun`: dedicated cluster `pg_ctl -m immediate stop` mid-run; `/health/ready` 503, `/health/live` 200, `POST /jobs` 503 `unavailable` + `Retry-After` (never 2xx), processes keep running, scheduler ticks fail; after restart readiness 200, same Idempotency-Key ⇒ 201 once, all 9 acknowledged jobs `succeeded`, version = event count, `attempts_used` = attempt rows, no active attempt; raw `outage_s=3.2 readiness_back_after_restart_s=4.3` |
| AC-09 | tested | M4 `TestAC09_FenceDenials` + `TestAuthAndLimits` + new `TestAC09_GRPCIdentityMatrix` (superseded incarnation on heartbeat/claim/renew/report, revoked credential mid-attempt on all four, `credential_mismatch` on claim/register) |
| AC-11 | tested | `TestAC11_RESTDrain` (REST drain: claims stop, short finishes, long one recovered by lease expiry after deadline — not canceled — and completed elsewhere; SDK deregisters, `Run` nil; audit `worker.drain`), `TestAC11_WorkerSIGTERM` (binary exits 0, attempt `failed/worker_shutdown/retryable`, worker deregistered), M4 `TestGracefulDrain` |
| AC-12 | partial | 429 + `Retry-After` (`TestM5_RateLimit`), body bounds on new routes (8 KiB), stream read limit 512 B, per-stream queue 256 frames. Adversarial fuzzing: NOT RUN |
| AC-13 | tested (Go side) | `TestAC13_RouteMatrix` (22 operations × operator/viewer session × operator/viewer token, anonymous, forged `X-User`/`X-Forwarded-User`/`X-Clusterx-Role`), `TestM5_SessionsLifecycle` (expired/wrong-aud/wrong-key/replayed assertions, sid↔sub, role hint, idle/absolute, disabled, logout), `TestM5_TokensAndWorkerCredentials`, WebSocket after logout ⇒ 4401 and reconnect 401 (`TestM5_StreamAuthHintsAndRevalidation`). CSRF: n/a in Go (Go never sees cookies; BFF-owned, ADR-0006 §3/§7) |
| AC-16 (server) | tested | per-entity monotonic hints (stale/duplicate dropped), `resync{buffer_overflow}` (`TestHubOverflow`), `resync{gap_detected}` broadcast, auth_expiring then 4401 (`TestM5_StreamSessionExpiry`), 5-stream limit 4429. Client-side reconciliation: web lane |
| AC-17 (retention) | tested | `TestRetentionPass`: runtime role refused (42501); retention login deletes terminal ≥30 d jobs with tombstones (GET ⇒ 410), cascades history, nulls child lineage, keeps young/non-terminal, expires 8-day log, deletes 91-day audit, 100-day tombstone, expired idempotency, expiry+7 d session; floors hold on direct DELETEs (23514); bounded pass reports `complete=false`. Backup/restore: NOT RUN |

## Mutant evidence (each applied alone with the script in the scratchpad, test run, file restored; `git status` clean afterwards)

| Mutant | Change | Exit | First failure |
|---|---|---|---|
| M5-sidsub | session `sid` of another user accepted | 1 | `TestM5_SessionsLifecycle` |
| M5-dbrole | principal role from assertion hint instead of DB | 1 | `TestM5_SessionsLifecycle` |
| M5-jti | mutation jti not consumed | 1 | `TestVerify` |
| M5-idle / M5-absolute / M5-disabled / M5-revoked | each session check removed | 1 | `TestM5_SessionsLifecycle` |
| M5-sessiononly | API tokens allowed on session-only routes | 1 | `TestM5_TokensAndWorkerCredentials` |
| M5-operatoronly | operator requirement dropped | 1 | `TestM5_TokensAndWorkerCredentials` |
| M5-signature | Ed25519 verification always passes | 1 | `TestVerify` |
| M5-audience | `aud` not checked | 1 | `TestVerify` |
| M5-expiry | `exp` not checked | 1 | `TestVerify` |
| M5-loginonly | login assertion accepted on normal routes | 1 | `TestM5_SessionsLifecycle` |
| M5-auditmask / M5-auditcat | viewer audit details / category filter removed | 1 | `TestM5_AuditAndMetrics` |
| M5-streamreval / M5-tokenreval | stream session / token re-validation always ok | 1 | `TestM5_StreamAuthHintsAndRevalidation` |
| M5-origin | stream Origin allowlist ignored | 1 | `TestM5_StreamAuthHintsAndRevalidation` |
| M5-ratelimit | bucket never empties | 1 | `TestM5_RateLimit` |
| M5-cancelaudit | job.cancel audit row renamed | 1 | `TestM5_CancelRetryDrain` |
| M5-retryfailedonly / M5-retrydup | retry of non-failed / duplicate child allowed | 1 | `TestM5_CancelRetryDrain` |
| M5-drainclaim | draining worker may claim | 1 | `TestM5_CancelRetryDrain` |
| M5-credmismatch | mismatch reported as revoked | 1 | `TestAC09_FenceDenials` |
| M5-logredact | SDK log redaction off | 1 | `TestAttemptLoggerBoundedAndRedacted` |
| M5-viewertokscope | REST check "viewer token only *:read" removed | 0 | **survived — equivalent**: store `validateTokenCreate` rejects the same input (422) |

Not mutated (reported honestly): the token-owner role ceiling in `CreateTokenFor` (`role_not_allowed`) is unreachable through REST because token routes already require an operator session — defense in depth.

## Executed curl additions (compose project `clusterx-m5`, ports 18480/19491, operator token from bootstrap; values never printed)

| # | Command | Result |
|---|---|---|
| 1 | `GET /health/ready` | 200 |
| 2–5 | `POST /jobs sleep_ms 8000`; `POST /jobs/{id}/cancel` | 202, `status running`, `cancel_requested_at` set; finalized `canceled` at the next renewal (≤ 15 s; overview shows `canceled 1`) |
| 6 | `POST /jobs/{failed}/retry` + `Idempotency-Key: c1` | 201, `retried_from_job_id` = parent |
| 7 | same again | 200 `Idempotent-Replayed: true` |
| 8 | `GET /jobs/{failed}/attempts` | `log_tail` contains the handler's `job.Log` line |
| 9 | `POST /workers/{id}/drain {"grace_seconds":5}` | 202 `draining` |
| 10 | `GET /metrics/overview` | counts by status + `workers {live 3, draining 1}` |
| 11 | `GET /audit?category=privileged` | `job.cancel, job.manual_retry, token.create, worker.drain, worker_credential.create` |
| 12–13 | `GET /tokens`, `POST /sessions` with an API token | 403 `credential_not_allowed` |
| 14 | `GET /me` with `a.b.c` (no JWKS configured) | 401 `assertion_invalid` |
| 15 | `GET /config` | `api_version 1.0.0-draft.6` |
| 16 | `GET /metrics/timeseries?metric=attempts_started&step=5m&from=-30m` | 6 complete buckets |
| 17 | `GET /stream` without Upgrade | 426 (library) |
| — | `grep -ciE 'cxt_|cxw_|cxs_|secret='` over all compose logs | 0 |

## New configuration (integrator: `.env.example` / compose)

Control plane: `CLUSTERX_BFF_JWKS_FILE` (Ed25519 JWKS, public keys only; unset ⇒ console sessions disabled), `CLUSTERX_SESSION_IDLE_TIMEOUT` (2h, 1m..24h), `CLUSTERX_SESSION_ABSOLUTE_TTL` (12h, 1h..7d, ≥ idle), `CLUSTERX_STREAM_ALLOWED_ORIGINS` (comma list; https only in production; empty = only Origin-less server clients), `CLUSTERX_RATE_MUTATIONS_PER_SECOND` (20; 0 disables), `CLUSTERX_RATE_AUTH_FAILURES_PER_MINUTE` (60), `CLUSTERX_RATE_LOGINS_PER_MINUTE` (10), `CLUSTERX_RETENTION_DATABASE_URL` (login in `clusterx_retention`), `CLUSTERX_RETENTION_INTERVAL` (0 = off, else ≥ 1m; needs the URL). Worker: `CLUSTERX_WORKER_ID` or `CLUSTERX_WORKER_ID_FILE` (needs a writable volume; compose workers are `read_only`, so not wired). **I did not edit `infra/compose/**`**: nothing there is required for the stack to boot; wiring the JWKS mount, a retention login role (`initdb/01-roles.sh`) and per-worker id volumes is integrator work.

CLI additions: `user upsert --github-id N --login L --role operator|viewer`, `user disable|enable --github-id N`, `retention run`, `dev-keygen --out-dir D [--kid k]`, `dev-assertion --key-file F (--login <github id> | --sub user:<uuid> --sid <uuid> [--role r]) [--ttl 60s]` (dev ones refuse `CLUSTERX_ENV=production`, never touch the DB).

## Open issues for the web lane

1. **Assertion key handling.** Generate the BFF Ed25519 key pair once per environment; the BFF keeps the private JWK (secret store, never in the browser bundle or logs); Go gets the public JWKS via `CLUSTERX_BFF_JWKS_FILE`. A JWKS with `d` is refused. Header must be exactly `{alg:"EdDSA", typ:"clusterx-assertion+jwt", kid}`; the claim set is **closed** (`iss, aud, sub, sid, role, purpose, iat, nbf, exp, jti, rid`) — e.g. `jose` `SignJWT` must not add other claims. `jti` 16..128 base64url chars, fresh per request; `exp - iat ≤ 60`. Rotation: add new `kid` to the JWKS, restart Go, switch signing, remove old after 2× TTL.
2. **Login flow.** After GitHub OAuth: `POST /api/v1/sessions` with a login assertion (`sub=github:<numeric id>`, `purpose=login`, no `sid`/`role`) and body `{"github_login": …}`; store the returned `secret` (cxs_…) only as the cookie value and `session_id` for `sid`. Users must be allowlisted first (`user upsert`).
3. **dev-assertion usage (local only).** `clusterx-control-plane dev-keygen --out-dir ./.dev-keys`; start Go with `CLUSTERX_BFF_JWKS_FILE=./.dev-keys/bff-dev.jwks.json`; `user upsert --github-id 1 --login me --role operator`; `dev-assertion --key-file ./.dev-keys/bff-dev-private.jwk.json --login 1` → POST /sessions; then `dev-assertion … --sub user:<id> --sid <session_id> --role operator` per request (one per mutation — jti is single use). It is a test helper, not a login bypass; never ship the private JWK.
4. **Stream.** Upgrade server-to-server without an `Origin` header (or allowlist the BFF origin); expect `hello` first, hints `{type:"hint", entity, id|null, version}`, `resync{buffer_overflow|gap_detected}`, `auth_expiring` ~60 s before expiry, closes 4401/4403/4408/4429/1001/1008. Server shutdown sends no `server_restart` frame (close 1001).
5. Session `idle_expires_at` moves only when Go bumps activity (≤ once/60 s on authenticated non-stream requests); `/me` returns the current values.

## Limitations / open issues

1. Rate limits, the jti replay cache and the stream hub are per process (single coordinator, ADR-0001); a restart resets them (RV01-12 residual unchanged).
2. Stream hints come from an in-process DB poll (1 s, 10 s look-back over `job_events.occurred_at`); a transaction committing > 10 s after its `occurred_at` would be missed (hints are best effort; clients reconcile). No LISTEN/NOTIFY (would need schema work).
3. Credential idempotency is stored in audit rows (no schema change); retention of audit (90 d) exceeds the 24 h window, fine. Needs integrator acknowledgement as a contract interpretation.
4. Operator drain then worker restart: Register sets the worker back to `active` (M4 behavior); a drained worker that restarts is undrained. Drain recovery bound = drain deadline + lease TTL.
5. Metrics `queue_age_p95_seconds` is measured at first claim only; timeseries over 7 d at 1h uses correlated subqueries — EXPLAIN at scale: NOT RUN (RV01-15 also still NOT RUN).
6. `unschedulable_jobs` overview counts queued jobs without a live **active** capable worker (all three reasons).
7. AC-08 covered an immediate PG shutdown with ~3 s outage; a connection drop **during COMMIT** (unknown outcome) is covered only by the existing 503 + Idempotency-Key contract, not by a failpoint test: NOT RUN.
8. TLS for WebSocket/HTTP and the real BFF↔Go path: NOT RUN (web lane / M6 integration). Adversarial fuzzing (AC-12), AC-17 backup/restore, AC-19: NOT RUN.
9. **Side effect on the integrator's images:** my first compose attempt (exit 1: parallel builds of the shared `clusterx/worker:local` tag) rebuilt and retagged `clusterx/control-plane:local` and `clusterx/worker:local` from this branch. The running `clusterx` containers were not touched; rebuild from main before the next `clusterx` restart. The successful run used `clusterx-m5/*:local` image names via a scratch override (`image:` + `build: !reset null` on worker-2/3) — the shared compose file has the same parallel-build race for any fresh clean build.
10. Chaos tests build binaries without `-race`; they skip without `initdb`/`pg_ctl` (AC-08) or `CLUSTERX_PG_ADMIN_DSN`.

## Integration needs

- Freeze/version OpenAPI `1.0.0-draft.6`; regenerate `packages/sdk-ts`; review migration 0003 (additive indexes) — SQL schema contract review required.
- CI: `initdb`/`pg_ctl` (PG 18) on PATH for `internal/chaos`; CREATE ROLE rights for retention/runtime role tests.
- Compose/env wiring listed above; consider unique image tags per service to avoid the build race.
- Critical review (TASK-M5-RV-01): assertion verifier, session re-check, stream re-validation, credential idempotency via audit rows, retention role.
