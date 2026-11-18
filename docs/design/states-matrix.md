# CLUSTERX States Matrix: route x state x permission

- Ticket: TASK-M2-FE-01, deliverable 4. Requirement IDs: FR-WEB-03, FR-WEB-08, NFR-A11Y-01, AC-13 (UI side), AC-15, AC-16, AC-21; RV01-10 (UI mapping, section 6).
- Source: `docs/design/route-inventory.md` (the per-route "Extra states", "Empty", "Error" and "Offline" rows, state catalogue `S-*` in section 1.4), `docs/security/permissions.md` (read as the matrix owner for masking), `docs/architecture/state-machine.md`, API contract `contract-m1-draft.4`.
- Status: **specified**. Prototype coverage is stated per row ("P" = a prototype state exists). All E2E IDs are `proposed`, **NOT RUN**.

## 1. How to read

- **Y** = the state can occur on that route and has a designed rendering. **-** = not applicable, with the reason in the per-route notes. **G** = handled only by the global surface (global 404, global error boundary).
- State names are the `S-*` catalogue in route-inventory section 1.4. Short names used as column headers:

| Column | State IDs |
|---|---|
| LOAD | S-LOAD (skeleton after 200 ms, timeout 10 s) |
| EMPTY | S-EMPTY-FIRST |
| FILT | S-EMPTY-FILTER |
| ERR | S-ERR-500, S-ERR-503, S-ERR-502, S-ERR-TIMEOUT |
| AUTH | S-401, S-SESSION-EXPIRED |
| 403 | S-403 (route) and S-403-ACTION |
| 404 | S-404 |
| EXP | S-EXPIRED-DATA (HTTP 410, `log_expired`, retention clamp) |
| 409 | S-409 (state conflict, idempotency conflict) |
| VAL | S-422, S-413, S-400 |
| 429 | S-429 |
| STALE | S-STALE, S-OFFLINE, S-RECONNECT |
| MASK | S-MASKED |
| NOWK | S-NO-WORKER |
| NOMET | S-NO-METRICS |
| MANY | S-TOO-MANY |

- Roles: **ANON** (no session), **VIEWER** (`viewer`), **OPERATOR** (`operator`). The UI hides what a role cannot use but Go is the boundary: every denied direct call still renders `S-403-ACTION` (AC-13).

## 2. Global surfaces

| Surface | Trigger | Rendering | Prototype |
|---|---|---|---|
| GLOBAL-404 | Unknown URL (public or app), and any 1.1 route in 1.0 | Names the path; links to `/`, `/docs`, and `/app` when signed in; full shell; HTTP 404 | none |
| GLOBAL-500 | Uncaught server or render error | No stack trace; digest or request ID with copy; Retry and Home; app shell stays usable; HTTP 500 | none |
| Console segment boundary | Error inside one console route | Per-route error boundary so sidebar and nav survive | none |
| Session expiry | 401 `session_expired`/`session_revoked`, stream close 4401 | Non-dismissible focus-trapped dialog "Your session ended"; sign in again returns to the same URL; drafts kept per route rule | job detail and login prototypes show the outcome copy |
| Stream close codes | 4401, 4403, 4408, 4429, 1008, 1001/1012 | 4401 session dialog; 4403 S-403; 4408 reconnect with backoff; 4429 stop reconnecting and fall to Polling; 1008 S-ERR-500 with request id; 1001/1012 reconnect | live indicator states in all console prototypes |
| Mobile menu | Menu button | Drawer with focus trap, `Esc`, focus return | all prototypes |
| Role banner | VIEWER | Persistent "Read-only access (viewer)" | console prototypes (role toggle) |

## 3. Route x state matrix

| Route (code) | LOAD | EMPTY | FILT | ERR | AUTH | 403 | 404 | EXP | 409 | VAL | 429 | STALE | MASK | NOWK | NOMET | MANY | Prototype states |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `/` (HOME) | - | - | - | G | - | - | G | - | - | - | - | - | - | - | - | - | clipboard-denied fallback noted; Atlas step states |
| `/architecture` (ARCH) | - | - | - | G | - | - | G | - | - | - | - | - | - | - | - | - | none |
| `/docs` (DOCS-IDX) | Y index | - | Y no results | Y index fail | - | - | G | - | - | - | - | - | - | - | - | - | none |
| `/docs/quickstart` (DOCS-QS) | - | - | - | G | - | - | G | - | - | - | - | - | - | - | - | - | unverified notice on home |
| `/docs/concepts` (DOCS-CON) | - | - | - | G | - | - | G | - | - | - | - | - | - | - | - | - | none |
| `/docs/api` (DOCS-API) | - | - | - | G | - | - | G | - | - | - | - | - | - | - | - | - | none |
| `/docs/workers` (DOCS-WRK) | - | - | - | G | - | - | G | - | - | - | - | - | - | - | - | - | none |
| `/docs/reliability` (DOCS-REL) | - | - | - | G | - | - | G | - | - | - | - | - | - | - | - | - | none |
| `/docs/deployment` (DOCS-DEP) | - | - | - | G | - | - | G | - | - | - | - | - | - | - | - | - | none |
| `/docs/security` (DOCS-SEC) | - | - | - | G | - | - | G | - | - | - | - | - | - | - | - | - | none |
| `/changelog` (CHANGELOG) | - | Y no releases | - | G | - | - | G | - | - | - | - | - | - | - | - | - | none |
| `/login` (LOGIN) | Y redirect | - | - | Y BFF 503/502 | Y outcomes | Y outcome panels | G | - | - | - | - | Y offline | - | - | - | - | P: 10 outcomes |
| `/demo` (DEMO) | Y chart chunk | Y fixture | Y fixture | Y fixture parse | - | - | Y in-demo | Y shown as example | Y shown | - | - | - (offline-safe) | Y toggle | Y shown | Y shown | Y shown | none |
| `/app` (APP-HOME) | Y per region | Y | - | Y per region | Y | Y if role removed | - | Y chart window | - | - | - | Y | - | Y | Y | - | P: loaded, loading, first use, no worker, no metrics, 503 stale, offline, viewer |
| `/app/jobs` (JOBS-LIST) | Y | Y | Y | Y | Y | - | - | Y history window | - | Y 422 filter/window, 400 cursor | Y | Y | - | Y reason chip | - | Y | P: loaded, loading, empty, filtered, 500, stale, offline, too many, invalid filter, viewer |
| `/app/jobs/new` (JOBS-NEW) | Y catalogue | Y no handlers | - | Y | Y draft kept | Y route (VIEWER) and action | - | - | Y idempotency | Y 422, 413 | Y | Y offline | - | Y warning | - | - | none |
| `/app/jobs/[id]` (JOBS-DETAIL) | Y per section | Y no attempts, no logs | - | Y per section | Y | Y action | Y | Y 410 and `log_expired` | Y cancel, retry | Y | Y | Y | Y | Y | - | Y events | P: running, cancel requested, failed, succeeded with rejected late result, unschedulable, expired, not found, offline, viewer |
| `/app/workers` (WORKERS-LIST) | Y | Y | Y | Y | Y | - | - | - | - | Y 422 filter | Y | Y heartbeat ages freeze | - | Y no live worker | - | Y fleet cap | P: loaded, loading, none, all offline, stale, offline, 1/10/100 |
| `/app/workers/[id]` (WORKERS-DETAIL) | Y | Y idle | - | Y | Y | Y action | Y (no 410) | - | Y drain | - | Y | Y | Y hostname | - | - | - | none |
| `/app/observability` (OBS) | Y per chart | - | - | Y per chart | Y | - | - | Y clamp to retention, 422 | - | Y 422 range | Y | Y | - | - | Y three variants | - | none |
| `/app/activity` (ACTIVITY) | Y | Y | Y | Y | Y | Y if VIEWER denied | - | Y 90 d | - | Y 422 | Y | Y | Y details | - | - | Y | none |
| `/app/settings` (SETTINGS) | Y live block | - | - | Y live block only | Y | - | - | - | - | - | - | Y | - | - | - | - | none |
| `/app/settings/tokens` (TOKENS) | Y | Y | - | Y | Y dialog persists | Y route (VIEWER) | - (stale item to 409/refresh) | - | Y | Y 422 | Y | Y mutations off | - | - | - | - | none |
| `/app/settings/preferences` (PREFS) | Y session block | - | - | Y session | Y | - | - | - | - | - | - | Y local prefs work | - | - | - | - | none |
| 1.1 routes (GATED) | - | - | - | - | - | - | G (404) | - | - | - | - | - | - | - | - | - | none |

Notes that explain the "-" cells on the rows that matter most:

- Public static routes have no data at runtime, so only the global boundary applies. Docs search is the only runtime state (index load, no results, index failure).
- `/login` is the auth surface: AUTH and 403 are outcomes shown as panels (`signed_out`, `session_expired`, `no_entitlement`, `user_disabled`, `auth_error`, `provider_unavailable`, `upstream_auth`, BFF 503, offline), not redirects.
- Workers have no 410 (a removed worker looks unknown), so WORKERS-DETAIL shows 404 only; metrics and audit have no 410, so OBS and ACTIVITY use a client clamp and a 422 (route-inventory section 4.7, GAP-RET-01).
- `/app/settings` and `/app/settings/preferences` have no mutation: no 409, 422, 429.

## 4. Permission matrix (what each role sees and can do)

The role arrives from the BFF session (`GET /bff/session`). Hidden is a courtesy; Go returns 403 on every denied call, and the UI must render it (S-403-ACTION, AC-13).

| Route | ANON | VIEWER | OPERATOR |
|---|---|---|---|
| Public routes (`/`, docs, `/architecture`, `/changelog`, `/demo`) | Full | Full | Full |
| `/login` | Sign-in and outcomes | Redirect to `/app` | Redirect to `/app` |
| `/app` | Redirect to `/login?next=` | KPIs, charts, topology, recent jobs; no "Submit job"; viewer banner | All plus "Submit job" |
| `/app/jobs` | Redirect | Same columns (none sensitive); no "Submit job" | All plus "Submit job" |
| `/app/jobs/new` | Redirect | S-403 route state with link to `/app/jobs`; nav item absent | Full |
| `/app/jobs/[id]` | Redirect | All metadata, state, attempts' non-sensitive fields, events (events are not masked, they carry no payload), lineage; **masked:** payload, result, job error message, attempt error message, log tail; no Cancel, no Retry; no diagnostic rows | All fields; Cancel (200 pending, 202 running); Retry (failed only); diagnostic rows from the audit log |
| `/app/workers` | Redirect | Full list; hostname masked in detail only | Full list |
| `/app/workers/[id]` | Redirect | Read-only; hostname masked; privileged events only ("No privileged events visible to your role" when empty); no Drain | Full; Drain (disabled with reason when revoked or deregistered) |
| `/app/observability` | Redirect | Full (aggregates only) | Full |
| `/app/activity` | Redirect | `privileged` category only, `details` masked (OD-13); category filter limited accordingly | All categories and details |
| `/app/settings` | Redirect | Full; Tokens link absent | Full; Tokens link |
| `/app/settings/tokens` | Redirect | S-403 route state; nav absent; direct API 403 | Full; secret shown once |
| `/app/settings/preferences` | Redirect | Own session only | Own session only |

**Masked fields (UI keys only on the API's `masked_fields`; raw values never reach a viewer's browser):**

| Object | Maskable (viewer) | Unmasked for viewer |
|---|---|---|
| Job | `payload`, `result`, `error_message` | status, timestamps, attempts counts, `failure_class`, unschedulable reason, lineage |
| Attempt | `error_message`, `log_tail` | state, class, error code, worker, timing |
| Job event | none (closed schema with no payload content) | all |
| Worker | `hostname` | everything else |
| Audit event | `details` | actor, action, outcome, subject, request ID |
| Token or credential | not reachable | n/a |

## 5. State catalogue to rendering (compact)

| State | Trigger | First words of the message | Recovery | Where shown in prototypes |
|---|---|---|---|---|
| S-LOAD | Request over 200 ms | none (skeleton), region `aria-busy` | none; times out at 10 s | `loading` on overview, jobs, workers |
| S-EMPTY-FIRST | Zero records ever | "No jobs yet" / "No worker has ever registered" | Role-aware CTA, docs link | overview, jobs, workers |
| S-EMPTY-FILTER | Filters exclude all | "No jobs match these filters" | Remove chip, Clear filters | jobs |
| S-ERR-500 | HTTP 500 | "Jobs could not be loaded" | Retry, request ID copy | jobs |
| S-ERR-503 | HTTP 503 | "The coordinator is not ready (503)" | Auto-retry with backoff, last data kept as stale | overview |
| S-ERR-502 | `upstream_auth` | "The console could not reach its own API" | Retry; never a login redirect | login |
| S-401 / SESSION | 401 | "Your session ended" | Sign in again, same URL | login (outcome) |
| S-403 / ACTION | 403 | "Read-only access (viewer)" banner; action absent | Return to allowed page | role toggle |
| S-404 | Unknown ID | "Job not found" | Back to jobs | job-detail |
| S-EXPIRED-DATA | HTTP 410 | "This job was removed by retention" | Back to list | job-detail |
| S-409 | `job_terminal`, `retry_already_exists`, `idempotency_conflict`, `worker_deregistered` | "This job already finished as succeeded; cancel was not applied" | Show refreshed state, offer allowed action | job-detail copy (conflict text in dialog flow) |
| S-NO-WORKER | `unschedulable_reason` | "No capable worker" | Link to workers | jobs, job-detail, overview |
| S-NO-METRICS | Empty, unavailable or too fresh | "No jobs finished in this range" | Widen range | overview |
| S-MASKED | `masked_fields` | "Hidden for viewer role" | none | job-detail (viewer) |
| S-STALE / OFFLINE | Snapshot age or no network | "This list is stale" / "You're offline" | Refresh, auto-recover | jobs, overview, workers, job-detail |
| S-TOO-MANY | More than 500 rows | "More than 500 jobs match" | Narrow filters | jobs |

## 6. RV01-10: UI mapping of the state-machine denial cells

RV01-10 (REVIEW-M1-RV-01) is a **backend** test gap: no executable evidence yet for AC-07, manual retry, RenewLease cap, ReportFailure classes and drain. Backend owns the table-driven event x state Go suite. This section is the **frontend consumer side**: for each row of `docs/architecture/state-machine.md` that a human can trigger or observe, how the UI renders it and which proposed E2E checks it. Nothing here proves backend behavior: **backend test status: NOT RUN in this lane**.

Worker-side events (E3 to E7, E10, E12) are not human-triggerable, so the UI only shows their results (state, attempts, events, diagnostic audit rows).

| Event | Cell | UI outcome | Prototype | E2E (proposed, NOT RUN) |
|---|---|---|---|---|
| E1 Admit | 201 created, 200 replay (`Idempotent-Replayed`), 409 `idempotency_conflict`, 413, 422, 429, 503 | "Job accepted" with link; "Same job as before"; "Use a new key"; limits; field errors; retry with same key; never "queued" on failure | none (`JOBS-NEW`) | `E2E-JOBS-NEW-20`, `-24`, `-25`, `-26`, `-33` |
| E2 Due promotion | scheduled to queued | Status changes after snapshot; "Available" time shown; no optimistic change | job rows show Scheduled with a later Available time | `E2E-JOBS-DETAIL-20` |
| E3 Claim | running with attempt | Attempt card, lease generation, worker, incarnation | job-detail running | `E2E-JOBS-DETAIL-21` |
| E4 RenewLease | extends lease; no event, no version bump | Lease countdown only; never announced; paused offline | job-detail countdown | `E2E-JOBS-DETAIL-32` |
| E5 ReportSuccess after cancel committed | job finalizes `canceled`, worker reply `OUTCOME_CANCELED`, event `data.worker_reported="success"` | Final status Canceled; timeline shows the cancel and the worker's uncommitted report note | job-detail cancel-requested copy | `E2E-JOBS-DETAIL-23` |
| E5/E6 stale or duplicate worker report | Denied with `stale_attempt`, `lease_expired`, `credential_revoked`, `incarnation_mismatch`, `result_mismatch` | Unchanged job; **diagnostic** timeline row (operator only) "Late report rejected (stale attempt)"; VIEWER sees no diagnostic rows | job-detail failed and succeeded states | `E2E-JOBS-DETAIL-28`, `-38` |
| E6 retryable, budget left | retry_wait with backoff | Retry wait status, attempts used, backoff text | jobs and job-detail | `E2E-JOBS-DETAIL-21` |
| E6/E7 failure, budget exhausted or permanent | failed with `failure_class` | Failed plus class text ("Retries exhausted", "Permanent error"); dead-letter filter | job-detail failed, jobs failed preset | `E2E-JOBS-LIST-21`, `-34`, `E2E-JOBS-DETAIL-36` |
| E8 Lease expiry or deadline | attempt `lease_expired`/`deadline_exceeded`; job retry_wait, failed or canceled | Attempt badge "Lease expired"; log may be expired | job-detail attempts | `E2E-JOBS-DETAIL-21` |
| E9 Cancel | 200 canceled now (pending, or already canceled), 202 running with cancel requested, 409 `job_terminal` | "Canceled" vs "Cancellation requested" (different wording and status); terminal conflict as an explanatory S-409 notice, not an error toast | job-detail cancel dialog and cancel-requested state | `E2E-JOBS-DETAIL-22`, `-23`, `-34` |
| E11 Manual retry | 201 child, 200 replay with same key, 409 `retry_not_allowed`, 409 `retry_already_exists` with child link | "New job created" with link; replay shows the existing child; not allowed explains why; button only on failed jobs | job-detail failed (retry dialog) | `E2E-JOBS-DETAIL-24`, `-25`, `-26`, `-37` |
| E12 Drain deadline | Renewals denied after deadline; lease expires, then E8 | Worker detail shows draining with the deadline; attempts recover per E8; text never claims the process stopped | none (`WORKERS-DETAIL`) | `E2E-WORKERS-DETAIL-20`, `-23`, `-29` |
| Drain request | 202 draining, already draining keeps original deadline, 409 `worker_deregistered` | "Drain requested", state changes after snapshot; 409 shows refreshed post-state | none | `E2E-WORKERS-DETAIL-21`, `-30` |
| Unschedulable (derived) | `no_capable_worker`, `capable_workers_offline`, `capable_workers_draining` | Reason chip on queued rows, explanation panel on detail, filter on list; never a status | jobs, job-detail, overview | `E2E-JOBS-LIST-25`, `-33`, `E2E-APP-HOME-24` |

**Frontend test obligations created by RV01-10 (for M6):** table-driven Playwright test per row above against a seeded live stack (never mocks for the `integrated` claim), plus the deterministic gap, reorder and duplicate-hint cases (`E2E-APP-HOME-22/-23`, `E2E-JOBS-DETAIL-29`). Reviewer mutants N12 and N13 (finished-attempt immutability, `deadline_exceeded` classification) concern backend code and are untouched here.
