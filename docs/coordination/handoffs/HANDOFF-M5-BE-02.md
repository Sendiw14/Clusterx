# HANDOFF — TASK-M5-BE-02 (REVIEW-M5-RV-01 fixes, M5 exit)

- **Author:** backend lane subagent, model `claude-opus-5-5`, requested effort medium. Effective effort and Ultracode availability: UNVERIFIED from inside the subagent (MW-04).
- **Branch:** `m5fix/backend` (worktree `.worktrees/backend`), base main `02e041d`. Not pushed, not merged.
- **Source contract:** REVIEW-M5-RV-01 (RV5-01..11), TASK-M5-BE-02 decisions, ADR-0003/0004/0006, OpenAPI `1.0.0-draft.6` → **`1.0.0-draft.7`** (text-only change in `RateLimited`, RV5-05).
- **Schema contract change (integrator review required):** migration `0004_m5_feed_cursor_grants.sql` adds `job_events.txid xid8` (+ partial index), the credential-idempotency index, and narrows `clusterx_runtime` grants. 0001–0003 byte-identical. Proto unchanged.
- **Requirements / acceptance:** AC-08 (RV5-01), AC-16 server side (RV5-04), AC-12 rate limits (RV5-05/RV5-11), AC-13 (RV5-07), FR-SEC-* (RV5-03, RV5-09, RV5-10), ADR-0003 drain (RV5-02).
- **Review owner:** RV-01 reviewer re-checks the P1 and every FIX item (status below is "fixed, pending re-review" for all).

## Commits

| SHA | Item | Content |
|---|---|---|
| `4680503` | RV5-01 (P1) | `classifyDBError` maps transport/pool failures (`io.EOF`, `io.ErrUnexpectedEOF`, `net.Error`, `pgconn.ErrConnClosed`, `puddle.ErrClosedPool`, plus the previous ConnectError/SafeToRetry/Timeout) to `ErrUnavailable`; REST (`v1.fail`) and gRPC (`toStatus`) use new `store.IsUnavailable`, so unclassified read paths cannot leak 500/INTERNAL. SDK: `codes.Internal` is transient (backoff, never fatal). AC-08 asserts 12 admissions during the outage + no `api internal error` / `worker rpc internal error` in the coordinator log. |
| `c4c3eb0` | RV5-05 | Auth-failure bucket drained only by `token_invalid` / `assertion_invalid`; no pre-auth `Peek` block (a verified credential is never refused by the bucket); OpenAPI `RateLimited` text + version draft.7. |
| `25b609a` | RV5-02 | Drain sticky to `worker_id`; deregistered id → new worker row; live-incarnation takeover audited (`security`/`worker.incarnation_takeover`); ADR-0003 section. |
| `f45d3b5` | RV5-03 | Strict JOSE: header members ⊆ {alg,typ,kid}, claims ⊆ closed set, exact lower-case names, no duplicates; `Claims.Kid` `json:"-"`; ADR-0006 text. |
| `33fa2f6` | RV5-04, RV5-08, RV5-10 | Migration 0004; feed cursor = previous poll's `pg_current_snapshot()` (reads rows visible now and not visible then); resync only on overflow, at most once per `ResyncEvery` (5 s); idempotency index + query predicate; grant revokes; ADR-0004 notes; NF-04 test extended. |
| `9974461` | RV5-07 | Hub filters hints by subscriber scopes (`job`→`jobs:read`, `worker`→`workers:read`, `metrics`→`metrics:read`); permissions.md. |
| `b489b0d` | RV5-11 (+RV5-05 gRPC) | `Store.DenialAudit` per-credential budget (1 row/s, burst 60) wired in `main.go`; gRPC unknown/malformed/missing worker secret charges a per-peer bucket (`CLUSTERX_RATE_AUTH_FAILURES_PER_MINUTE`) → `RESOURCE_EXHAUSTED rate_limited`; state-machine.md + ADR-0004. |
| `b4313cb` | RV5-06 | Tests killing N08, N12, N18, N22, N28 (N01/N03 in `f45d3b5`, N14 in `33fa2f6`). |
| `efb2118` | RV5-04 | Late-commit feed test forces a concurrent commit so the snapshot lists the open tx in `xip`. |
| `78e38c5` | RV5-09 | `*_FILE` variants: `CLUSTERX_DATABASE_URL_FILE`, `CLUSTERX_MIGRATE_DATABASE_URL_FILE`, `CLUSTERX_RETENTION_DATABASE_URL_FILE`, `CLUSTERX_METRICS_TOKEN_FILE`, worker `CLUSTERX_WORKER_CREDENTIAL_FILE` (both set / unreadable ⇒ fail closed, no value echo). |
| (this) | RV5-09 | compose secrets files, JWKS via `dev-keygen`, retention login + optional `retention` service, image prefix/ports, `APIVersion` draft.7, this handoff. |

## Per-item status

| ID | Status | Tests (new / changed) |
|---|---|---|
| RV5-01 P1 | fixed, pending re-review | `store/classify_test.go` `TestClassifyDBErrorConnectionLoss` (reviewer proof ported); `sdk-go/transient_test.go` `TestRunSurvivesCoordinatorInternalErrors` (fake conn: Claim INTERNAL/UNAVAILABLE, Heartbeat INTERNAL ⇒ Run keeps going, returns nil on ctx end; fails without the fix); `chaos_test.go` AC-08 strengthened |
| RV5-02 | fixed, pending re-review | `store/rv5_drain_test.go` `TestRV5_OperatorDrainSurvivesReRegister` (reviewer proof + deregistered ⇒ new row), `TestRV5_ConcurrentIncarnationsNewestWins` (both directions fenced, 2 audit rows) |
| RV5-03 | fixed, pending re-review | `auth/strict_test.go` `TestVerifyStrictJOSE` (reviewer's 3 cases + unknown claim, header jku/crit/dup kid/ALG, wrong/missing typ, future nbf, non-object payload) |
| RV5-04 | fixed, pending re-review | `api/feed_internal_test.go` `TestFeedLateCommitIsHinted` (event `occurred_at` 1 h in the past, tx open 500 ms across polls; hinted, no resync), `TestFeedOverflowResyncIsBounded` |
| RV5-05 | fixed, pending re-review | `TestRV5_AuthFailureBucketIsNotCrossUser` (reviewer proof inverted), `TestM5_RateLimit` changed, `workerrpc/limits_test.go` `TestWorkerAuthFailureLimit` |
| RV5-06 | fixed, pending re-review | see mutant table: all 8 survivors killed |
| RV5-07 | fixed, pending re-review | `api/hub_internal_test.go` `TestHubHintsFilteredByScope` |
| RV5-08 | fixed, pending re-review | `TestM5_TokensAndWorkerCredentials` (operator B reusing A's key ⇒ 201 own token); ADR-0004 interpretation + retention ≥ 24 h; index in 0004 (EXPLAIN NOT RUN) |
| RV5-09 | fixed, pending re-review | `config/file_test.go` (control plane), `apps/worker/internal/config/file_test.go`; compose check below |
| RV5-10 | fixed, pending re-review | `TestNF04_PrivilegesAndDeleteGuards` extended (INSERT/UPDATE tombstones, UPDATE job_events/audit_events ⇒ 42501) |
| RV5-11 | fixed (implemented), pending re-review | `TestRV5_DenialAuditRateLimited` |

## Commands and exit codes

Environment: `go1.27.4 darwin/arm64`; PostgreSQL 18.7, own cluster `<S>/pgbe` on `localhost:55432` (`-k ''`, `LC_ALL=en_US.UTF-8`, `max_connections=300`), `CLUSTERX_PG_ADMIN_DSN=postgres://clusterx@localhost:55432/postgres?sslmode=disable`, PG 18 bin on `PATH`. `<S>` = session scratchpad; logs in `<S>/be2/`.

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `apps/control-plane: go test -race -count=20 -run TestAC08 -v ./...` (after RV5-01) | 0 | 20/20 `--- PASS: TestAC08_DatabaseOutageMidRun`, 0 FAIL; `ok …/internal/chaos 181.677s` (`<S>/be2/ac08_x20.txt`) |
| 2 | Same with the EOF classification removed (mutant B01, `-count=3`) | 1 | `chaos_test.go:332: admission 0 during outage: 500 map[code:internal …]` — proves AC-08 now hits the stale-pool-connection path every run |
| 3 | `apps/control-plane: go vet ./... && go test -race -count=1 ./...` (final, HEAD before the compose commit) | 0 | all 12 packages ok incl. chaos, e2e (`<S>/be2/cp_race_final.txt`) |
| 4 | `GOWORK=off go vet ./... && go test -race -count=1 ./...` in `packages/sdk-go`, `apps/worker`, `packages/proto`, `examples/basic-jobs` | 0 each | ok |
| 5 | Mutant runner `<S>/be2/mut.sh` / `muts.sh` (in-place `perl -0pi`, `go test -count=1`, `git checkout` restore; tree clean afterwards) | — | table below (`<S>/be2/mutants.txt`) |
| 6 | Compose check | see below | |

## Mutant evidence

Reviewer survivors (same substitutions as `rv5/muts2.sh`):

| Mutant | Exit | Killed by |
|---|---|---|
| N01-typ | 1 | `TestVerifyStrictJOSE` `wrong typ: got <nil>` |
| N03-nbf | 1 | `TestVerifyStrictJOSE` `future nbf: got <nil>` |
| N08-logtailmask | 1 | `TestRV5_LogTailMaskAndAnyReadScope` `viewer attempt not masked` |
| N12-locktimeout | 1 | `TestRV5_WorkerLockTimeoutBound` `blocked renew … after 4.0s (want unavailable within ~1 s)` |
| N14-credidemactor | 1 | `TestM5_TokensAndWorkerCredentials` `status 200 want 201` |
| N18-cancelleasecap | 1 | `TestRV5_RunningCancelCapsLease` |
| N22-revokedregister | 1 | `TestRV5_RegisterRechecksRevokedCredential` `err = <nil>; want denied credential_revoked` |
| N28-anyread | 1 | `TestRV5_LogTailMaskAndAnyReadScope` `status 200 want 403` |

Fix mutants (each fix reverted alone):

| Mutant | Exit | Killed by |
|---|---|---|
| B01 EOF not unavailable | 1 | `TestClassifyDBErrorConnectionLoss`; AC-08 (row 2 above) |
| B02 Register resets drain | 1 | `TestRV5_OperatorDrainSurvivesReRegister` `status after re-register = active` |
| B02b resurrect deregistered | 1 | same test, `credential_mismatch` |
| B02c no takeover audit | 1 | `TestRV5_ConcurrentIncarnationsNewestWins` `audit rows = 0, want 2` |
| B03 claim strictness off | 1 | `TestVerifyStrictJOSE` `duplicate sub` |
| B03b header strictness off | 1 | `TestVerifyStrictJOSE` `header jku` |
| B04 cursor `txid >= xmax(prev)` | 1 | `TestFeedLateCommitIsHinted` `never hinted` |
| B04b resync unbounded | 1 | `TestFeedOverflowResyncIsBounded` `resyncs = 3, want exactly 1` |
| B05 every 401 counts | 1 | `TestRV5_AuthFailureBucketIsNotCrossUser` `status 429 want 401` |
| B05b pre-auth Peek block back | 1 | `TestM5_RateLimit` `status 429 want 200` |
| B05c gRPC guess limit off | 1 | `TestWorkerAuthFailureLimit` |
| B07 no hint scope filter | 1 | `TestHubHintsFilteredByScope` |
| B10 / B10b grants not revoked | 1 / 1 | `TestNF04_PrivilegesAndDeleteGuards` |
| B11 denial audit unlimited | 1 | `TestRV5_DenialAuditRateLimited` `rows = 5, want 2` |

Totals: 8/8 reviewer survivors killed; 15/15 fix mutants killed. (B04 first survived: with no other commit, the poll snapshot's `xmax` never passed the open transaction; the test now commits a concurrent admission — commit `efb2118`.)

## Compose check (RV5-09)

Isolation: a worktree-local gitignored `.env` (generated passwords; `CLUSTERX_PG_PORT=55440`, `CLUSTERX_HTTP_PORT=18080`,
`CLUSTERX_METRICS_PORT=19091`, **`CLUSTERX_IMAGE_PREFIX=clusterx-m5fix`**) and `COMPOSE_PROJECT_NAME=clusterx-m5fix` (equivalent of
`-p clusterx-m5fix`; bootstrap's `docker compose` calls inherit it). Images were tagged `clusterx-m5fix/control-plane:local` and
`clusterx-m5fix/worker:local` through the new `${CLUSTERX_IMAGE_PREFIX:-clusterx}` image names, so `clusterx/*:local` was not
overwritten (`clusterx/control-plane:local` id `sha256:31e24a20…`, created before this run, unchanged). The running `clusterx`
project was never touched (`docker compose ls` shows it `running(5)` before and after).

| Step | Exit | Evidence |
|---|---|---|
| `COMPOSE_PROJECT_NAME=clusterx-m5fix infra/compose/bootstrap.sh` on a clean volume | **0** | `Ready: http://127.0.0.1:18080/health/ready` (`<S>/be2/bootstrap.txt`); migrate `Exited (0)`, 3 workers up |
| `GET /health/ready` | — | 200 |
| Operator token from `.secrets.env`: submit `hash_sha256`, poll | — | `succeeded` (workers authenticated via `CLUSTERX_WORKER_CREDENTIAL_FILE`) |
| JWKS wired | — | log `BFF assertion keys loaded … kids:["dev-1"]`; `POST /sessions` with garbage ⇒ 401 (not 503 "sessions disabled") |
| `docker inspect` env of control-plane, worker-1, migrate | — | only `*_FILE=/run/secrets/…` paths; no DB URL, token or credential values |
| `docker compose --profile retention run --rm retention` | 0 | `retention pass … complete:true` as `clusterx_retention_job` |
| `docker compose --profile workers --profile retention down -v`; `docker image rm clusterx-m5fix/…` | 0 | volume + network removed; worktree `.env`, `.secrets/`, `.secrets.env` deleted |

Secrets files: `.secrets/` is mode 0700 on the host; files are 0644 inside it so the container users (65532, postgres 999)
can read the bind mounts on Linux hosts; `bff-dev-private.jwk.json` is 0600 and mounted nowhere.

## Design notes

- **RV5-01.** Classification is a whitelist of transport signals, so application errors (`pgx.ErrNoRows`, SQLSTATE 23xxx, `Denial`) stay as they were (unit-tested). Pool-acquire timeouts are `context.DeadlineExceeded`: REST already maps them to 503; gRPC answers `DEADLINE_EXCEEDED`, which the SDK retries. A drop during `COMMIT` now answers 503 "retry with the same Idempotency-Key" (ambiguous outcome resolved by idempotency); a failpoint test for that is still NOT RUN.
- **RV5-02.** A self-drain (SIGTERM) is sticky too: a process that crashes mid-drain and restarts with its id file stays draining, gets `worker_draining` on Claim, deregisters, exits 0; the supervisor restart then registers a new row. One extra restart, no lost work.
- **RV5-04.** `pg_current_snapshot()` inside the poll statement is that statement's snapshot, so "visible now" and the new cursor agree. A failed poll keeps the cursor (no resync: nothing is lost). Polls now run with or without subscribers (one indexed query per second) so a client that subscribes mid-interval still gets hints. Rows from before 0004 have `txid NULL` and are never fed. Cost of the cursor: the `xip` list of the previous snapshot is sent as text each poll (bounded by concurrent transactions).
- **RV5-05.** Behind the BFF all users share a peer; only forgery signals count, and a verified credential is served even when the bucket is empty. Residual: guessing still costs one token lookup / one Ed25519 verify per request (accepted: 256-bit secrets).
- **RV5-11.** Denials beyond the budget are answered but not audited (the limiter is per process, reset on restart, ADR-0001). No new env var; values are constants in `main.go`.

## Limitations / NOT RUN

- EXPLAIN for the credential-idempotency index and the feed query at scale (RV01-15 still open).
- Feed behaviour > 180 events/s measured: NOT RUN (overflow path is unit-tested only).
- TLS paths, fuzzing (AC-12), backup/restore (AC-17): unchanged, NOT RUN.
- COMMIT-time connection-drop failpoint test: NOT RUN.

## Integration needs

1. **Integrator review of migration 0004** (schema contract change: new column + index, grant narrowing).
2. **Contract bump `1.0.0-draft.7`** (OpenAPI text only, `RateLimited` description; `APIVersion` constant). Web lane: regenerate the API reference / any version string pinned to draft.6; tag `contract-m5-draft.7` after merge.
3. **Web lane (RV5-04/AC-16):** keep a bounded periodic snapshot refresh as a safety net (reviewer condition); server now never silently drops a committed hint except on overflow (then `resync{gap_detected}`, ≤ 1 per 5 s).
4. **Web lane (RV5-02):** drain is sticky to the worker id — a drained worker that restarts shows as a **new** worker row; the old row ends `deregistered`.
5. **Web lane (RV5-07):** token streams without `workers:read`/`metrics:read` no longer receive those hints.
6. **Docs (web lane owns `apps/web/content/docs/*`):** quickstart/deployment should mention `infra/compose/.secrets/` (secrets files), the dev BFF key pair (`.secrets/bff-dev-private.jwk.json`, development only), the optional `retention` profile, and the new optional `.env` keys (`CLUSTERX_HTTP_PORT`, `CLUSTERX_METRICS_PORT`, `CLUSTERX_IMAGE_PREFIX`). `.secrets.env` is still written (operator token) as before.
7. **Existing local stacks:** `initdb` runs only on a clean volume, so an existing `clusterx` volume has no `clusterx_retention_job` login; the `retention` service fails auth there until the volume is recreated (or the role is created by hand). The main services keep working (`POSTGRES_PASSWORD_FILE` is only read at init).

## TASK-M5-BE-03 (review round 2 follow-ups, same branch)

Source: REVIEW-M5-RV-01 "## Round 2 (3456189)" (verdict PASS conditional on RV5R-01) and the integrator's TASK-M5-BE-03 message.

| SHA | Items |
|---|---|
| `5cffaa6` | **RV5R-01 (MUST):** in `Register`, a `worker_id` naming a deregistered row now falls through to the same candidate selection as an empty `worker_id`: reuse the matching incarnation; `incarnation_mismatch` if the credential has a live non-deregistered row; take over a stale row; otherwise insert. **RV5R-02:** the `worker.incarnation_takeover` audit row is charged to the per-credential `DenialAudit` budget (the takeover itself is unconditional). **RV5-02 ratification** in ADR-0003: drain under a supervisor acts like a rolling restart (revoke the credential to keep a host out); every graceful restart creates a new `workers` row. |
| `b52de4a` | **RV5R-03(b):** a poll whose snapshot `xmax` is below the previous cursor's (restore/failover) counts as a gap ⇒ `resync{gap_detected}` (rate-bounded), cursor adopted. **RV5R-03(a):** documented only (Feed doc comment: long xid holders make polls rescan from `xmin`). **RV5R-06:** worker version map pruned to the ids the workers query still returns. **RV5R-04:** tests for R04 (trailing data after the claims object), R06 (revoked worker credential never charges the gRPC guess bucket), R07 (`/stream` subscribes with the principal's scopes, end to end). **RV5R-05:** `bootstrap.sh` runs `dev-keygen` as the host uid/gid into a fresh `.keygen.XXXXXX` temp dir (no other secrets mounted), then moves the keys into `.secrets/` (private 0600, public 0644). |
| (this) | handoff section |

### RV5R-01 before / after

| Run | Exit | Output |
|---|---|---|
| Ported proofs `TestRV5R_DeregisteredIdBypassesSingleLiveBinding`, `TestRV5R_TakeoverAuditBudgeted` against `3456189`'s `workers.go` | 1 | `rv5_drain_test.go:127: err = <nil>; want denied incarnation_mismatch`; `rv5_drain_test.go:155: takeover audit rows with the budget spent: 50` |
| Same tests on `5cffaa6` (`-race`, with all `TestRV5*`, `TestAC09*`, `TestNF*` in store) | 0 | all PASS |

The ported RV5R-01 test asserts the reviewer's condition (exactly one non-deregistered row per credential after 10 attempts with the deregistered id, each answered `incarnation_mismatch`) and, additionally, that once the live row is stale the deregistered id takes it over instead of inserting.

### Mutants (round 3, `<S>/be2/mutants_r3.txt`)

| Mutant | Exit | Killed by |
|---|---|---|
| R04-trailing | 1 | `TestVerifyStrictJOSE` `trailing data: got <nil>` |
| R06-guesslifecycle (`\|\| d == nil` form) | 1 | `TestWorkerRevokedCredentialNotChargedAsGuess` `charged the guess bucket 3 times` |
| R07-streamscopes | 1 | `TestStreamHintScopeWiring` `jobs:read stream received … entity:worker` |
| R08-deregresurrect | 1 | `TestRV5_OperatorDrainSurvivesReRegister` |
| RV5R01 deregistered id skips candidate selection | 1 | `TestRV5R_DeregisteredIdBypassesSingleLiveBinding` |
| RV5R02 takeover audit unbudgeted | 1 | `TestRV5R_TakeoverAuditBudgeted` `rows … 50` |
| RV5R03b regression not a gap | 1 | `TestFeedXidRegressionAndWorkerPrune` `gap=false` |
| RV5R06 no prune | 1 | `TestFeedXidRegressionAndWorkerPrune` `not pruned` |

R02 (feed without `NOT pg_visible_in_snapshot`) stays as the reviewer classified it: equivalent at the client; not addressed.

### Commands (BE-03)

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `apps/control-plane: go vet ./... && go test -race -count=1 ./...` | 0 | 12/12 packages ok incl. chaos, e2e, poc (`<S>/be2/cp_race_be03.txt`) |
| 2 | `go test -race -count=5 -run TestAC08 -v ./internal/chaos/` | 0 | 5/5 PASS, 0 FAIL (`<S>/be2/ac08_x5_be03.txt`) |
| 3 | `GOWORK=off go vet ./... && go test -race -count=1 ./...` in `packages/sdk-go`, `apps/worker`, `packages/proto`, `examples/basic-jobs` | 0 each | ok |
| 4 | Compose check | see below | |

Compose check (BE-03): same isolation as above (worktree-local `.env`, `COMPOSE_PROJECT_NAME=clusterx-m5fix`,
`CLUSTERX_IMAGE_PREFIX=clusterx-m5fix`, ports 55440/18080/19091); `clusterx/control-plane:local` id `sha256:31e24a20…` identical
before and after; project `clusterx` still `running(5)`.

| Step | Exit | Evidence |
|---|---|---|
| `COMPOSE_PROJECT_NAME=clusterx-m5fix infra/compose/bootstrap.sh`, clean volume | **0** | `Ready: http://127.0.0.1:18080/health/ready` (`<S>/be2/bootstrap_be03.txt`) |
| readiness; operator-token job `hash_sha256` | — | 200; `succeeded` |
| JWKS from host-uid keygen | — | `BFF assertion keys loaded` ×1; `.secrets/bff-dev-private.jwk.json` `-rw-------`, `bff-dev.jwks.json` `-rw-r--r--`; no `.keygen.*` dir left |
| `docker inspect` env (control-plane, worker-1) for `postgres://`, `cxw_`, `cxt_` | — | 0 matches each |
| `--profile retention run --rm retention` | 0 | |
| `down -v` (workers + retention profiles), `docker image rm clusterx-m5fix/*` | 0 | 0 `m5fix` volumes left; worktree `.env`, `.secrets/`, `.secrets.env` deleted |

### Still open / NOT RUN (BE-03)

- Linux-host bootstrap run (RV5R-05 fix is reasoned for native Linux, verified on macOS only).
- Suppressed-audit counter metric (reviewer recommendation, not required): not added.
- EXPLAIN for `audit_events_idempotency_idx`; feed > 180 events/s; COMMIT-time drop failpoint: unchanged, NOT RUN.
