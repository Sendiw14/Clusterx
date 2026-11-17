# HANDOFF — TASK-M1-FE-02 (consumer sign-off of backend contract)

- Author, actual model/effort, Ultracode availability: frontend lane subagent, model `claude-sonnet-5-5`. Effort high requested via Agent param; effective effort and Ultracode **UNVERIFIED** (MW-04).
- Branch, commit(s), changed files: branch `m1/frontend`, one commit on top of `eabf4ff` (see `git log -1`). Files: `docs/design/consumer-requirements.md` (new section 11, CCR table dispositions, status line), `docs/design/route-inventory.md` (in-place changes plus section 7 change log), this file. No other path touched; backend worktree and main were read-only via `git show`.
- Contract SHA, generated code version, migration impact: reviewed `api/openapi.yaml` 1.0.0-draft.3 and companion docs at backend `f82325b`; decisions at `main` `12c004e`. No generated code, no migration.
- Implemented/Integrated (distinguish): specified/reviewed only. No code. No live call, no TS client generation: **NOT RUN**.
- Exact commands, exit codes and PASS/FAIL/BLOCKED/NOT RUN: only `git show`/`git diff` reads and local text edits. No test, lint or build exists or was run. Cross-checks were manual reading of the documents.
- Screenshots, traces, CI links, logs and other evidence: none.
- Auth/security, UX/accessibility and backwards-compatibility review: not independently reviewed. Security-relevant frontend decisions: 401 `user_disabled` goes to the no-entitlement panel (avoids a re-login loop); `assertion_*` never redirect to login; `csrf_failed` single auto-retry; one stream socket per tab; secrets never persisted. Residual: GAP-SESS-01 (idle enforcement has no stated store).

## Verdict: SIGN-OFF WITH GAPS

Every CCR is satisfied, deferred by integrator decision with an interim, or covered by a gap with a fallback already written into the routes. No gap blocks a route outright. `/app/settings/tokens` is **unblocked** (CCR-01 accepted).

Dispositions: CCR-01 SATISFIED (GAP-CRED-01); CCR-02 SATISFIED; CCR-03 SATISFIED with gaps; CCR-04 SATISFIED with gap; CCR-05 DEFERRED (M6, interim `/audit`); CCR-06 DEFERRED (M6); CCR-07 SATISFIED; CCR-08 DEFERRED (M8); CCR-09 SATISFIED with gaps (ADR-0006 still Proposed); CCR-10 SATISFIED. Confirmed for backend: 401 `details.reason` mapping; BFF codes `upstream_auth` (502) and `csrf_failed` (403); `/bff/session` shape; 403 `not_allowlisted` and `user_disabled` render the `/login` no-entitlement state (distinct copy, no loop).

## GAP list, ranked by the route it blocks (full detail in `consumer-requirements.md` 11.8)

| Rank | GAP | Route(s) | Missing | Fallback |
|---|---|---|---|---|
| 1 | GAP-SESS-01 | all `/app*` | Go `sessions.last_activity_at`; BFF has no stated store for idle timers | best-effort idle logout; absolute expiry enforced |
| 2 | GAP-BFF-01 | `/login` | closed set of `?error=` / `?reason=` and auth-failure codes in ADR-0006 | frontend proposal (11.4), needs ADR amendment |
| 2 | GAP-BFF-02 | all API routes | BFF must proxy Go errors/status/headers unchanged and use the same error envelope for its own codes | frontend builds it that way; record in ADR-0006 |
| 3 | GAP-ERR-01 | `/app/jobs/new`, `/app/jobs` | enum for `details.fields[].rule`; no codes for window/`run_after`/step bounds | key on `field`, show `message` |
| 3 | GAP-ERR-02 | `/app/jobs/new` | `field` path format for `payload` violations | summary list; client-side schema validation |
| 4 | GAP-SORT-01 | `/app/jobs`, `/app/workers` | `sort` param; `available_after/before`, `failure_class`, handler-version filters | one fixed sort; workers sorted client-side |
| 5 | GAP-RT-01 | RT routes | pong/heartbeat interval | UI silence timeout then refetch |
| 5 | GAP-RT-02 | workers, activity, home | hint for liveness decay, `audit` entity, `seq` | timed refetch |
| 5 | GAP-RT-03 | RT routes / AC-16 | test seams to drop/reorder/duplicate frames, expire sessions | integrator test-build seams |
| 6 | GAP-AUDIT-01 | `/app/activity` | actor/action/outcome/request_id filters | category/subject/window only |
| 6 | GAP-AUDIT-02 | activity, worker detail, job detail | `action` enum; which worker lifecycle events are audited; audit `request_id` equals API `request_id` | render verbatim, no promises |
| 7 | GAP-OBS-01 | `/app/observability` | queue-depth series, duration p50, workers series | 8 real metrics + overview |
| 7 | GAP-OBS-02 | observability | null vs 0 bucket semantics | null = gap, 0 = zero |
| 7 | GAP-RET-01 | observability, activity | behavior for windows before retention (422 vs empty) | clamp using `/config` |
| 8 | GAP-CRED-01 | `/app/settings/tokens` | idempotency on credential creates | client double-submit block, unknown-outcome copy |
| 9 | GAP-WRK-01 | `/app/workers/[id]` | `revoked_at` | badge without time |
| 10 | GAP-CFG-01 | `/app/settings` | environment name, build/commit, contract SHA | `api_version` only |
| 11 | GAP-TIME-01 | job/worker detail | `as_of` on single resources | HTTP `Date` passthrough, `hello.server_time` |
| 12 | GAP-OVR-01 | `/app` | named window for terminal counts | label from `/config` |
| 13 | GAP-DOC-01 | none | doc mismatches: unschedulable shape, hint shape, `retry_child_job_id` vs `retried_as_job_id`, 404-vs-410 wording, `api_version` example, `internal` without 500 response | UI follows openapi |

## Route changes made (details in `route-inventory.md` section 7)
`/login` (real BFF flow, new outcomes), `/app` (no alerts), `/app/jobs` (no sort, explicit window, reason filter), `/app/jobs/new` (`/handlers`, replay 200 + header), `/app/jobs/[id]` (cancel 200/202, 410, unmasked events, diagnostics via `/audit`), `/app/workers` (client sort, de-dup), `/app/workers/[id]` (grace input, no 410), `/app/observability` (real metric set), `/app/activity` (reduced filters, viewer privileged-only), `/app/settings` (`/config`), `/app/settings/tokens` (unblocked, composed rotate). New global states S-ERR-502 and S-CSRF. `/demo` and public static routes unchanged (no API).

- Known defects, warnings, deferred scope: OQ-03 (viewer `/audit`), OQ-11 (local-only preferences) and OQ-13 (typeface) still need owner nods. The 2-hour idle / 12-hour absolute session lifetimes are owner-pending. E2E IDs remain proposed, NOT RUN.
- Next owner and required merge/contract step: backend closes GAP-ERR/SORT/OBS/AUDIT/CFG/CRED/WRK/TIME/DOC items or accepts the fallbacks; integrator amends ADR-0006 (GAP-BFF-01/02, GAP-SESS-01) and records dispositions before freeze at M1 exit; RV-01 reviews ADR-0006.
- Reviewer independent from author; reviewer feedback: none yet (integrator Opus).
- Updated requirement IDs and release gate disposition: FR-API-01..03, FR-RT-01, FR-SEC-01..03/05, FR-WEB-02/08/09 consumer side `specified` and signed off with gaps; traceability file not edited (integrator-owned). No release gate satisfied.
