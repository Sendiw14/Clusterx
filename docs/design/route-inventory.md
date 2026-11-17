# CLUSTERX — Route Inventory and Route Contracts (frontend, M1 input)

- Ticket: TASK-M1-FE-01 · Lane: frontend (`m1/frontend`) · Author model: `claude-sonnet-5-5`, effort high (requested)
- Source contract: `docs/product/MASTER_PROMPT.md` R2 @ `5a5ca99eb44145100044bf9517e55813cb40903c` (M0). §8 is the route authority; §5 job states; §6 REST/realtime; §9 identity/motion; §10 a11y.
- Requirement IDs: FR-WEB-01..09, FR-RT-01, FR-DEMO-01, NFR-A11Y-01, NFR-RESP-01, AC-14/15/16/21, MW-01.
- Status: **specified** (not approved, not implemented). Nothing here claims code, tests or integration. API needs live in `consumer-requirements.md`; this file never designs endpoints.
- Evidence vocabulary: every E2E ID below is `proposed`. No test has run: **NOT RUN**.

## 0. Summary

| Group | Routes (1.0) | Count |
|---|---|---|
| Public | `/`, `/architecture`, `/docs`, `/docs/quickstart`, `/docs/concepts`, `/docs/api`, `/docs/workers`, `/docs/reliability`, `/docs/deployment`, `/docs/security`, `/changelog`, `/login`, `/demo` | 13 |
| Console | `/app`, `/app/jobs`, `/app/jobs/new`, `/app/jobs/[id]`, `/app/workers`, `/app/workers/[id]`, `/app/observability`, `/app/activity`, `/app/settings`, `/app/settings/tokens`, `/app/settings/preferences` | 11 |
| **Total 1.0** | | **24** |
| 1.1 gated/absent | `/app/workflows`, `/app/workflows/[id]`, `/app/workflow-runs/[id]` | 3 (not built, not linked, not routable in 1.0) |

Plus cross-route surfaces specified once in section 3: global 404, global error boundary, session-expiry flow, mobile menu, live-status indicator.

## 1. Conventions used by every route contract

### 1.1 Personas and roles

| Code | Meaning | Source |
|---|---|---|
| ANON | Anonymous visitor (P-EVAL). Never reaches control plane. | journeys.md |
| VIEWER | Authenticated role `viewer`: read-only, sensitive fields masked. | §4 |
| OPERATOR | Authenticated role `operator`: mutate jobs/workers, manage credentials. | §4 |
| DEV | P-DEV acts as OPERATOR in console, or via direct API tokens outside the console. | journeys.md |

The UI role comes from the BFF session. **The UI never decides authorization**: it hides what a role cannot do, and still renders the Go 403 if the API denies (role changed, stale session). Hidden is a courtesy; Go is the boundary (§4).

### 1.2 Data source vocabulary

- `STATIC` — build-time content (MDX/TSX), no runtime fetch.
- `BFF-AUTH` — Next.js BFF session/auth routes (web-lane, Opus owns the auth contract). Not a Go REST operation.
- `API:<op>` — a §6 operation reached through the BFF using the generated TS client behind the transport abstraction (§1.3 step 4 of the master). Example `API:GET /jobs`.
- `RT` — best-effort WebSocket hint channel (§6). Hints never carry authoritative data.
- `FIXTURE` — bundled, typed, seeded JSON used only by `/demo`.
- `NEED` — the route needs something absent from §6; tracked as a CCR in `consumer-requirements.md`. Route behavior is specified now, implementation is blocked until the CCR is decided. Until then the control is **omitted**, never rendered dead (§8 "every control must actually work or be omitted").

### 1.3 Standard E2E suite

Each route has a code `<RT>`. Standard IDs are `E2E-<RT>-NN`. A route lists which apply; omitted numbers are N/A with a reason given in the route block. Route-specific tests start at `20`.

| NN | Name | Assertion (summary) |
|---|---|---|
| 01 | Render | Landmarks, single `h1`, `<title>`, primary content visible, zero console errors, correct nav `aria-current`. |
| 02 | Loading | Shape-matched skeleton appears after delay budget, no CLS beyond 0.1 when data lands, 10 s timeout becomes `S-ERR-TIMEOUT`. |
| 03 | Empty | First-use empty and filtered-empty (where filters exist) render distinct content and recovery action. |
| 04 | Server error | 500 and 503 each render a distinct state with copyable `request_id` and working retry. |
| 05 | Session expiry | 401 / expired BFF session mid-use shows `S-SESSION-EXPIRED`; re-auth returns to same URL; no data from the old session remains. |
| 06 | Forbidden | VIEWER on operator route or action: route-level 403 or control absent; direct-API 403 renders `S-403-ACTION`. |
| 07 | Not found / expired | Unknown ID renders `S-404`; retention-expired renders `S-EXPIRED-DATA`; these are distinguishable. |
| 08 | Offline / stale / reconnect | Network offline, WS drop, polling fallback, gap reconcile; stale/disconnected state visibly shown; never a convincing stale view. |
| 09 | 320 px | At 320 CSS px and 400% desktop zoom: no horizontal page scroll, all actions reachable, no truncated-only critical data. |
| 10 | Reduced motion | With `prefers-reduced-motion: reduce`: no transform/translate/parallax motion, state changes still announced, no functional loss. |
| 11 | Keyboard + axe | Full keyboard journey, visible focus, focus not obscured, axe zero serious/critical; modal trap/restore where overlays exist. |
| 12 | Screen reader (manual) | NVDA and VoiceOver script: landmark order, names/roles, live-region behavior, table/chart alternatives. Evidence is manual, recorded as MANUAL-PASS/FAIL, never inferred from axe. |

Test widths for 09 and screenshot QA: 320, 360, 390, 768, 1024, 1440, 1920. Browsers: current Chrome, Firefox, Safari, Edge (exact matrix is a release-gate artifact, §10).

### 1.4 Shared state vocabulary (`S-*`)

Routes reference these by ID and add route-specific copy/recovery. All states use text plus icon, never color alone. Every error state includes a recovery action and, where an API error exists, the `request_id` with a copy button.

| ID | State | Meaning / behavior |
|---|---|---|
| S-LOAD | Loading | Skeleton matching final layout. Shown only if load exceeds 200 ms (avoid flash). Region has `aria-busy=true`. Timeout 10 s becomes S-ERR-TIMEOUT. |
| S-EMPTY-FIRST | Empty, first use | System has no data of this kind (e.g. no jobs ever). Explains what creates data, with a CTA only if the role may create. |
| S-EMPTY-FILTER | Empty, filtered | Filters/time window exclude everything. Shows active filters as removable chips and "Clear filters". Never shown as first-use. |
| S-ERR-500 | Server error | HTTP 500 or unclassified. Message from `code` mapping, not raw server text. `request_id`, Retry, link to `/docs/reliability`. |
| S-ERR-503 | Service unavailable | 503 (DB outage / not ready). Honors `Retry-After`; auto-retries with backoff; keeps last good data labelled stale; mutations disabled with reason. |
| S-ERR-TIMEOUT | Client timeout | No response in 10 s. Distinct from 503; Retry. Mutations: "outcome unknown, check before retrying" (never auto-resubmit a mutation without idempotency key). |
| S-429 | Rate limited | Shows `Retry-After` countdown, disables the triggering control only. |
| S-401 | Unauthenticated | No session. Redirect to `/login?next=<validated path>`. |
| S-SESSION-EXPIRED | Session expired mid-use | Non-dismissible dialog (focus-trapped): "Your session ended." Primary: sign in again (same tab, returns to same URL). Unsubmitted form drafts retained per route rule, tokens/secrets never. Background polling and WS stop. |
| S-403 | Forbidden (route) | Authenticated but role lacks access. Names the required role, no leak of resource existence beyond that. Link to allowed home. |
| S-403-ACTION | Forbidden (action) | API denied a mutation the UI offered. Inline message, refresh role/session, remove control. |
| S-404 | Not found | Resource ID does not exist (or caller may not know). Back to list. |
| S-EXPIRED-DATA | Data expired | Resource or sub-data (logs 7 d, history 30 d, audit 90 d; exact windows per retention ADR) was removed by retention. Says "expired by retention", gives the retention window, never "not found". |
| S-409 | Conflict | State changed under the user (e.g. cancel on terminal job; idempotency key reused with different body). Refetch, show what changed, offer allowed next action. |
| S-413 | Too large | Client pre-check and server 413. Shows limit and current size. |
| S-422 | Validation | Field-level errors mapped by `details` path to inputs; summary region links to first error; focus moves to summary. |
| S-400 | Malformed request | Should be unreachable from the UI. Generic message + `request_id`, flagged as bug. |
| S-TOO-MANY | Too many rows | Result set exceeds the bounded window. Shows capped count, instructs narrowing filters/time window; no "load everything". |
| S-NO-WORKER | No capable worker | Job/queue has an `unschedulable` reason. Shows reason, the handler type/version required, which workers exist and why they do not match, link to `/app/workers`. **A reason, never a job state.** |
| S-NO-METRICS | No metrics | Distinguishes (a) no data in window, (b) metrics source unavailable (503-class), (c) metrics too fresh/not yet aggregated. Different copy each. Never renders zeros as data. |
| S-MASKED | Viewer-masked field | Field shown as an explicit "Hidden for viewer role" placeholder with lock icon and text; never blank, never `null`-looking, never a fake value. |
| S-STALE | Stale data | Snapshot age exceeds threshold or live channel is not live. Banner with age and "Refresh". Charts/topology dim and label "as of <time>". |
| S-OFFLINE | Browser offline | `navigator.onLine=false` or consecutive fetch failures. Banner, last data retained and labelled, mutations disabled, auto-recover on `online`. |
| S-RECONNECT | Live reconnecting | WS dropped; indicator `Reconnecting`; polling fallback engaged; on reconnect force snapshot reconcile before showing `Live`. |
| S-SUCCESS | Success | Mutation accepted. Inline confirmation (polite live region) plus reconciled new state; toast only supplements, never sole signal. |
| S-PENDING | Mutation pending | Button shows spinner + text change, `aria-disabled`, double-submit blocked. State is not shown as changed until the snapshot confirms it. |

### 1.5 Live status indicator (FR-RT-01)

One component, six states, text plus icon: `Live` (WS connected, last snapshot reconciled), `Reconnecting`, `Polling` (WS unavailable, bounded polling active; interval shown), `Stale` (age over threshold), `Offline`, `Paused` (tab hidden: polling suspended, refresh on visibility). Pinned in the app topbar, with `role=status` and polite announcements **only on state change, never per event**. Per-route `RT` subscriptions are listed per route.

### 1.6 Timestamps (all routes)

Rendered via one `<Timestamp>`: `<time datetime="UTC ISO">`; text is user-local with explicit zone abbreviation/offset (preference from `/app/settings/preferences`, else browser zone); tooltip/popover and copy give UTC. Relative time ("3 s ago") is supplementary and always paired. Public pages that show dates (changelog) use UTC dates with the label "UTC". DST ambiguity tests are part of `E2E-PREFS-2x`.

### 1.7 Global motion matrix

Values are proposed starting values within §9 ranges, to be tuned by design review. Every row obeys: transform and opacity only; interruptible (new trigger reverses from current value, no jump); no scroll hijack; critical controls never wait on animation.

| ID | Used by | Trigger | Initial → end | Delay | Duration | Ease | Interruption / reversal | Repeat mount | Scroll up/down | Viewport / visibility | Reduced-motion fallback |
|---|---|---|---|---|---|---|---|---|---|---|---|
| M-HERO | `/` hero | First paint of route | opacity 0→1, translateY 12→0 px; stagger 60 ms across 4 elements | 0 | 560 ms (within 450–650) | `cubic-bezier(.2,0,0,1)` | Navigation away cancels; no reversal | Plays once per full load; not on client back-nav | N/A | Starts only when document visible | Instant final state, no translate |
| M-SECTION | Marketing/docs sections | Section enters viewport (IntersectionObserver, 20% visible) | opacity 0→1, translateY 8→0 px | 0, stagger ≤ 40 ms | 260 ms (180–350) | same | Scrolling away mid-way leaves it at current value; completes on re-enter | Once per page view; does not replay on scroll up | Down: plays. Up: stays visible, no replay | Not observed while tab hidden | Final state at mount |
| M-HOVER | Buttons, links, rows, cards | Hover / focus-visible | color/opacity/shadow-opacity; no layout shift | 0 | 150 ms (120–180) | `ease-out` | Reverses from current value on leave | N/A | N/A | Not applied on touch hover | Color change only, 0 ms |
| M-OVERLAY | Popover, dialog, drawer, toast | Open/close | opacity 0→1, scale .98→1 or translateX 16→0 (drawer) | 0 | 200 ms open / 160 ms close (160–240) | same | Close during open reverses; focus handling is immediate, not animation-gated | Every open | N/A | N/A | Opacity only, ≤ 80 ms, or instant |
| M-ROUTE | Route change | Client navigation | content opacity .6→1 | 0 | 180 ms (150–250) | `ease-out` | New navigation cancels; no outgoing animation blocks input | Every navigation | Scroll resets to top; focus to `h1` | N/A | Instant swap |
| M-LIVE | Console rows/cards updated by live data | Snapshot reconcile changes a row | background overlay opacity .35→0 | 0 | 400 ms | `ease-out` | Retrigger restarts overlay; capped at 1 per row per 2 s | Per change | N/A | Suppressed when tab hidden | No flash; changed cell gets "Updated" text chip for 3 s |
| M-ATLAS | Execution Atlas (marketing) | User presses Play/Step, or auto-play on first entering viewport once | Node/edge state class swaps, token travels along connector via `offset-distance`/transform | 0 | 600 ms per step | `linear` for token, `ease-out` for node | Pause stops at current step; Step jumps; no endless loop (auto-play runs the 8-step script once then rests) | Play on user demand only after first | N/A | Pauses when scrolled out of view or tab hidden | Step-through only; token position changes with no travel, all text identical |
| M-TOPO | Console topology | Active attempt on a node | Pulse ring opacity, only while a real attempt is running on that worker | 0 | 1200 ms, max 3 cycles per activation | `ease-in-out` | Stops immediately when attempt ends | N/A | N/A | Suppressed when hidden or at ≥ 100 workers (aggregate clusters do not pulse) | No pulse; static "active" badge with text |
| M-SKEL | Skeletons | S-LOAD | opacity .5↔1 shimmer substitute | 0 | 1000 ms loop, hard stop at timeout | `ease-in-out` | Stops on data | N/A | N/A | Stops when hidden | Static tint, no loop |

No parallax, no scroll-jacking, no counters that animate numeric values in the console (numbers are authoritative; they change instantly).

### 1.8 Responsive and a11y baselines (inherited by every route)

- Breakpoints: <480 mobile, 480–767, 768–1023, 1024–1439, ≥1440. Content max width 1200 (marketing), 1440 (console).
- **320 px rule:** no horizontal page scroll at 320 CSS px or 400% zoom. Data tables become labelled stacked cards below 640 px (same columns, same order, each cell prefixed by its header; sort/filter controls remain). Code blocks, JSON editor and wide diagrams scroll **inside their own focusable region** (two-dimensional-content exception), with an accessible summary alternative adjacent.
- Console navigation: ≥1024 fixed sidebar; <1024 topbar menu button opening a drawer (focus trap, Esc closes, focus returns, `aria-expanded`).
- Primary mobile actions (submit job, inspect job, cancel, retry, worker detail, drain) are never moved behind hover or hidden on mobile (§10).
- A11y baseline: skip link first in tab order; one `h1`; logical heading levels; landmarks `header/nav/main/footer`; focus moves to `h1` after client route change; focus ring ≥ 2 px, 3:1 contrast, never obscured by sticky bars (WCAG 2.4.11, scroll-padding); target size ≥ 24×24 CSS px (2.5.8); single-pointer alternatives for drag/pan/zoom (2.5.7); status messages via `role=status`/`aria-live=polite`, errors via `role=alert` only for blocking errors; forms keep entered values across validation errors (3.3.7); sign-in uses GitHub redirect, no cognitive test (3.3.8); text/icon status with legend; contrast tokens verified ≥ 4.5:1 text and ≥ 3:1 non-text before use (starting tokens in §9 are not yet measured).
- Sticky headers use `scroll-padding-top`; no page-level `overflow-x`.

---

## 2. Public routes (v1.0) — 13

Public routes have no session: states 401, 403, session-expiry are N/A (ANON), except `/login` which owns the session-outcome states. All public routes have no `RT`. All public routes render with JavaScript delayed or failing: content is server-rendered; interactive items degrade to static equivalents.

### 2.1 `/` — Home (`HOME`)

| Field | Contract |
|---|---|
| Purpose | Explain, in one screen then by scrolling, what CLUSTERX is and its honest guarantee: **at-least-once** distributed job processing with fencing. Get an evaluator to `/demo` or `/docs/quickstart`. |
| Persona | ANON (P-EVAL), DEV (first look). |
| Sections | Branded nav + wordmark; editorial hero (headline states at-least-once; CTAs: "Explore demo" `/demo`, "Quickstart" `/docs/quickstart`); interactive Execution Atlas (job, queue, attempt, lease, worker, recovery, result), **labelled "Illustrative, scripted"**; three capability pillars; step-by-step architecture strip linking `/architecture`; worker-failure/retry story (lease expiry then recovery then stale result rejected); console preview; quickstart code block with copy; limitations + FAQ; GitHub/docs CTA; substantial footer (docs, architecture, changelog, security, GitHub, license once OD-02 decided). |
| Data source | `STATIC`. Atlas script is a deterministic in-repo script (not telemetry). Console preview is an image/fixture-render of the shipped console, captioned with the commit/version it was captured from; it must match what ships (§9). |
| Actions | Nav links; CTAs; Atlas Play/Pause/Step/Reset; copy quickstart; FAQ expand; external GitHub link (opens in same tab with visible external indicator). No mutations, no auth. |
| RBAC / field scope | None (public). No production data, no live badges. Console preview never shows real job IDs. |
| Responsive | Hero stacks <768; Atlas collapses to a vertical step list with the same 8 steps and per-step captions (full Atlas diagram scrolls in its own region only ≥480 px); pillars stack; quickstart code scrolls in-region; nav becomes a drawer. At 320 px hero headline wraps without clipping; CTAs full width. |
| Interaction / motion | M-HERO, M-SECTION, M-HOVER, M-OVERLAY (nav drawer), M-ATLAS. Atlas auto-plays once on first view then rests; no looping. |
| Loading | Server-rendered; no skeleton. Console preview image uses fixed aspect-ratio box (no CLS). |
| Empty | N/A (static). |
| Error | Image load failure shows alt text + caption; copy-failure (clipboard denied) shows "Select and copy manually" with text pre-selected. Route-level 500 uses global error boundary (section 3). |
| Success | Copy success: polite status "Copied". |
| Offline | Static content works after load; external links fail natively. Copy still works. |
| Extra states | 401/403/session expiry: N/A. 404: global. Too-many-rows, no-capable-worker, no-metrics, expired data: N/A. Masked: N/A. |
| Accessibility | Atlas has: text alternative narrative (same content as steps), controls are native buttons with `aria-pressed`, current step announced politely (`role=status`), diagram `role=img` with long description linked, no information in color alone, pausing satisfies 2.2.2. Hero decorative art `aria-hidden`. |
| E2E IDs | `E2E-HOME-01`, `-02`(preview image only), `-04`(global boundary), `-08`, `-09`, `-10`, `-11`, `-12`. Specific: `-20` hero states at-least-once and contains no exactly-once or HA claim (AC-28/29 copy check); `-21` Atlas Play/Pause/Step/Reset keyboard-operable and finishes with 8 steps; `-22` Atlas is labelled illustrative and no element carries "Live"; `-23` quickstart copy puts exact text on clipboard; `-24` footer/nav links all resolve (no 404); `-25` no network request to control-plane or BFF API paths from `/`; `-26` no workflow/DAG claim (AC-29). |

### 2.2 `/architecture` — Architecture (`ARCH`)

| Field | Contract |
|---|---|
| Purpose | Make the real topology and trust boundaries understandable, with tradeoffs and limits stated plainly. |
| Persona | ANON, DEV, security reviewer. |
| Sections | Layered topology (Browser, Next.js SSR/BFF, Go control plane, PostgreSQL, workers) with boundary markers; job/lease path (admit, promote, claim, renew, report, recover); failure domains table; tradeoffs (single coordinator, SKIP LOCKED not FIFO, at-least-once); limitations; source links (repo docs/ADRs). Diagram plus textual alternative. |
| Data source | `STATIC`. |
| Actions | Anchor links, source links, expand/collapse failure-domain rows (native `details`). |
| RBAC | None. |
| Responsive | Diagram shows as stacked layers <768 (same nodes, vertical), full diagram in scroll region otherwise; failure-domain table becomes stacked cards. |
| Interaction / motion | M-SECTION, M-HOVER; diagram has optional hover highlight (focus equivalent) of a layer's description, no animation required. |
| Loading | Static; none. |
| Empty | N/A. |
| Error | Global boundary. Broken source link is a CI link-check failure, not runtime state. |
| Success | N/A. |
| Offline | Works after load. |
| Extra states | 401/403/session N/A; 404 global; others N/A. |
| Accessibility | Diagram `role=img` + `aria-describedby` long description + equivalent ordered list; headings per layer; tables with captions. |
| E2E IDs | `E2E-ARCH-01`, `-08`, `-09`, `-10`, `-11`, `-12`; `-20` diagram long description and list present and consistent with node set; `-21` page states limitations (single coordinator, at-least-once, not FIFO); `-22` source links resolve; `-23` no HA/DAG claim. |

### 2.3 Docs shared contract (`D-COMMON`) — applies to 2.3–2.10

- Layout: docs sidebar (section tree, `aria-current=page`), on-this-page (heading anchors, scroll-spy updates `aria-current` without stealing focus), search (client-side static index built at build time; opens with button or `Ctrl/Cmd+K` only, no single-character shortcuts per WCAG 2.1.4), prev/next, "Edit on GitHub" link.
- Search states: loading index (S-LOAD in popover), no results ("No matches; try fewer words" + link to `/docs`), index failed to load (S-ERR-500 variant, falls back to a plain link list of all docs), offline works if index cached.
- Code blocks: copy button with status; dark code block allowed (§9); horizontally scrollable in own focusable region; language label; command prompts excluded from copy.
- Stamp: any page containing commands shows "Verified against commit `<sha>` on `<date>`" only if the AC-26 clean-checkout run passed; otherwise a visible **"Not yet verified from a clean checkout"** notice. Never silent.
- Responsive: sidebar becomes a "Docs menu" drawer <1024; on-this-page collapses to a disclosure at top; 320 px safe.
- Motion: M-SECTION off for docs body (reading surfaces do not animate text); M-HOVER, M-OVERLAY, M-ROUTE only.
- Data: `STATIC` (MDX). `/docs/api` additionally consumes the published OpenAPI document at build time.
- Anonymous, no auth, no RT. 401/403/session expiry/masked/too-many-rows/no-capable-worker/no-metrics/expired data: N/A.

### 2.4 `/docs` — Documentation index (`DOCS-IDX`)

| Field | Contract |
|---|---|
| Purpose | Entry to documentation: concepts map, search, guided first steps. |
| Persona | DEV, ANON. |
| Sections | Search; "Start here" ordered path (quickstart, concepts, workers, api); core concepts map (job, attempt, lease, fencing, worker, state, retry, idempotency) linking `/docs/concepts` anchors; section cards for all 7 docs pages with one-line purpose. |
| Data source | `STATIC`. |
| Actions | Search, links. |
| RBAC | None. |
| Responsive | D-COMMON; cards single column <640. |
| Interaction / motion | D-COMMON. |
| Loading / Empty / Error / Success / Offline | Loading: none (SSR); search index S-LOAD. Empty: search no-results. Error: index-fail fallback list; global boundary. Success: N/A. Offline: works once loaded. |
| Accessibility | Concepts map has a list equivalent (the map is a list with grouping, not an image-only graph); search is a `role=search` landmark with combobox pattern and result count announced politely. |
| E2E IDs | `E2E-DOCS-IDX-01`, `-03`(search no-results), `-04`(index failure), `-08`, `-09`, `-10`, `-11`, `-12`; `-20` every doc page reachable from index; `-21` Ctrl/Cmd+K opens search, Esc closes, focus restored; `-22` search finds "lease" and routes to concepts anchor. |

### 2.5 `/docs/quickstart` — Quickstart (`DOCS-QS`)

| Field | Contract |
|---|---|
| Purpose | Get a developer from clean checkout to first job, 3 workers, a recovery demo. |
| Persona | DEV. |
| Sections | Prerequisites (Docker with Compose V2, supported host); `docker compose` + bootstrap commands; first job (curl and console); scale workers; recovery demo (kill a worker, watch lease expiry, stale result rejected); expected results; troubleshooting; cleanup. |
| Data source | `STATIC`. Commands imported from one tested source (examples/scripts) so docs and CI cannot diverge. |
| Actions | Copy per block; step checklist (local, `localStorage`-only convenience; never required). |
| RBAC | None. |
| Responsive | D-COMMON; numbered steps stay linear at 320 px. |
| Interaction / motion | D-COMMON. |
| Loading / Empty / Error / Success / Offline | Static. Error: verification-stamp notice (see D-COMMON) when not verified. Success: copied status; checklist item checked announced. Offline: works. Empty: N/A. |
| Accessibility | Ordered list semantics; each command block labelled with its step; expected output blocks labelled "Expected output". |
| E2E IDs | `E2E-DOCS-QS-01`, `-08`, `-09`, `-10`, `-11`, `-12`; `-20` every command block's copy equals the tested script source (hash compare); `-21` verification stamp present only when the AC-26 evidence file exists, else the not-verified notice is shown; `-22` no command references legacy `docker-compose` V1; `-23` checklist state persists across reload and degrades if storage blocked. |

### 2.6 `/docs/concepts` — Concepts (`DOCS-CON`)

| Field | Contract |
|---|---|
| Purpose | Define the vocabulary exactly as the engine uses it. |
| Persona | DEV, OPS, VIEW. |
| Sections | The **7 job states** (`scheduled, queued, running, retry_wait, succeeded, failed, canceled`) with transition table; orthogonal fields; **unschedulable as a reason, not a state**; **dead-letter as a filtered `failed` view**; attempt vs job; leases and fencing; at-least-once; due jobs and priority 0–9 (9 highest); cancellation; idempotency; "reserved for 1.1: `blocked`, `skipped`" noted as not in 1.0. |
| Data source | `STATIC`. State table content derived from the frozen state/event source once published (no hand-copied divergent table). |
| Actions | Anchors, links to console filters (deep links like `/app/jobs?status=failed` shown as examples, not live). |
| RBAC | None. |
| Responsive | D-COMMON; transition table to stacked cards <640. |
| Interaction / motion | D-COMMON. |
| Loading / Empty / Error / Success / Offline | Static; global boundary; N/A otherwise. |
| Accessibility | State names also in text in status badges (legend component reused here as canonical legend). |
| E2E IDs | `E2E-DOCS-CON-01`, `-08`, `-09`, `-10`, `-11`, `-12`; `-20` exactly 7 states listed, none named `unschedulable` or `dead_letter`; `-21` states table matches the generated state source (diff test); `-22` 9 = highest priority text present. |

### 2.7 `/docs/api` — API reference (`DOCS-API`)

| Field | Contract |
|---|---|
| Purpose | Generated, versioned reference for `/api/v1`. |
| Persona | DEV. |
| Sections | Auth (direct API tokens, scopes, never in browser), error model `{code, message, details?, request_id}`, status codes 400/401/403/404/409/413/422/429/503, pagination and filter conventions, per-operation reference with request/response examples, idempotency, request IDs, API version policy, contract version/commit shown. |
| Data source | `STATIC` generated from the published OpenAPI at the frozen contract SHA at build time. No runtime call to the API. Generated pages are never hand-edited. |
| Actions | Operation anchors, copy curl examples, jump by tag. |
| RBAC | None; documents roles per operation as stated by the contract. |
| Responsive | D-COMMON; endpoint tables stacked; schema trees collapse (`details`). |
| Interaction / motion | D-COMMON. No "try it" console in 1.0 (would be an unauthenticated control-plane path from a public page). |
| Loading / Empty / Error / Success / Offline | Static. If the OpenAPI artifact is missing at build, the build fails (not a runtime state). |
| Accessibility | Schema trees are nested lists with expand buttons, not canvas. |
| E2E IDs | `E2E-DOCS-API-01`, `-08`, `-09`, `-10`, `-11`, `-12`; `-20` every operation in the contract appears and nothing else does; `-21` displayed contract SHA equals build input; `-22` examples copy verbatim; `-23` page makes zero network calls to `/api/v1`. |

### 2.8 `/docs/workers` — Worker SDK (`DOCS-WRK`)

| Field | Contract |
|---|---|
| Purpose | Teach Go SDK use: install, register handlers, versioning, error types, timeouts, credentials, local test. |
| Persona | DEV. |
| Sections | SDK install; handler registration and schema versioning; `context.Context` cancellation (honest: deadline does not kill a handler); typed retryable vs permanent errors; timeouts, heartbeat vs lease renewal; worker credentials (operator-issued, one-time reveal); local test example; idempotent side-effect patterns. |
| Data source | `STATIC`; code examples sourced from compiled `examples/`. |
| Actions | Copy, anchors. |
| RBAC | None. |
| Responsive / Interaction | D-COMMON. |
| Loading / Empty / Error / Success / Offline | Static (as D-COMMON). |
| Accessibility | D-COMMON. |
| E2E IDs | `E2E-DOCS-WRK-01`, `-08`, `-09`, `-10`, `-11`, `-12`; `-20` page never claims forced termination (copy lint); `-21` code samples equal compiled examples (hash). |

### 2.9 `/docs/reliability` — Reliability (`DOCS-REL`)

| Field | Contract |
|---|---|
| Purpose | State guarantees and non-guarantees. |
| Persona | DEV, OPS, security reviewer. |
| Sections | Retry budgets (`max_attempts` 1–5 including first run); recovery timelines (lease expiry, drain, cancel); side-effect caveats; failure modes; guarantees (at-least-once, fenced DB writes) and non-guarantees (no exactly-once, single coordinator downtime, no FIFO, deadline not kill); measured numbers only if benchmarks exist else "not yet measured". |
| Data source | `STATIC`. |
| Actions / RBAC / Responsive / Motion / States / Accessibility | As D-COMMON. Timelines are ordered lists with an optional diagram. |
| E2E IDs | `E2E-DOCS-REL-01`, `-08`, `-09`, `-10`, `-11`, `-12`; `-20` no numeric performance claim without a linked benchmark artifact (copy lint); `-21` limitations section present. |

### 2.10 `/docs/deployment` — Deployment (`DOCS-DEP`)

| Field | Contract |
|---|---|
| Purpose | Single-VM reference deployment, operations. |
| Persona | DEV/OPS. |
| Sections | Single Linux amd64 VM reference; TLS (reverse proxy); secrets flow; compose (local vs production separate); networking/firewall/ports; backups, restore, upgrades; RPO/RTO (measured values only when drill exists, else "not yet measured"); incident runbook links. |
| Data source | `STATIC`. |
| Rest | As D-COMMON; verification stamp rule applies. |
| E2E IDs | `E2E-DOCS-DEP-01`, `-08`, `-09`, `-10`, `-11`, `-12`; `-20` no RPO/RTO number without drill artifact; `-21` local-demo config vs production config clearly separated; `-22` no claim of serverless Go hosting. |

### 2.11 `/docs/security` — Security (`DOCS-SEC`)

| Field | Contract |
|---|---|
| Purpose | Explain authentication, roles, worker trust, demo separation, threat-model summary, reporting policy. |
| Persona | Security reviewer, DEV, OPS. |
| Sections | Human auth (GitHub OAuth code + PKCE, allowlist, roles); direct API tokens; worker trust and revocation bound; demo separation; viewer field masking matrix summary; threat-model summary; reporting policy (links `SECURITY.md`). Claims limited to evidenced controls ("no blanket compliance"). |
| Data source | `STATIC`. |
| Rest | As D-COMMON. |
| E2E IDs | `E2E-DOCS-SEC-01`, `-08`, `-09`, `-10`, `-11`, `-12`; `-20` no compliance-blanket claim (copy lint for "compliant", "certified"); `-21` role matrix equals access-matrix source; `-22` states demo has zero production access. |

### 2.12 `/changelog` — Changelog (`CHANGELOG`)

| Field | Contract |
|---|---|
| Purpose | Real shipped releases and migration notes. |
| Persona | DEV, OPS. |
| Sections | Release list (version, UTC date, highlights, migration notes, breaking-change callouts, compare links). |
| Data source | `STATIC` generated from `CHANGELOG.md`. |
| Actions | Anchors by version, link to GitHub release. |
| RBAC | None. |
| Responsive | Single column; version header sticky off <768. |
| Interaction / motion | M-HOVER, M-ROUTE. |
| Loading | None (SSR). |
| Empty | **Pre-release: "No releases yet. CLUSTERX 1.0 has not shipped."** No fictitious versions. |
| Error | Global boundary. |
| Success | N/A. |
| Offline | Works after load. |
| Extra states | Auth states N/A. |
| Accessibility | Dates in `<time>` with "UTC". |
| E2E IDs | `E2E-CHANGELOG-01`, `-03`, `-08`, `-09`, `-10`, `-11`, `-12`; `-20` page versions equal git tags/`CHANGELOG.md` (no invented versions); `-21` empty state renders when no releases. |

### 2.13 `/login` — Sign in (`LOGIN`)

| Field | Contract |
|---|---|
| Purpose | Start GitHub sign-in and clearly explain every non-success outcome. No default accounts, no bypass, no demo login. |
| Persona | ANON, VIEWER, OPERATOR. |
| Sections | Wordmark; "Sign in with GitHub" button; explanatory text (who can sign in, "access is by approval"); outcome panel (one of the states below); link to `/demo` for anonymous exploration; link to `/docs/security`. |
| Data source | `BFF-AUTH` (OAuth code + PKCE/state, server-held session; Opus owns the auth contract). No Go REST call from this page. |
| Actions | Start sign-in (navigates to BFF auth start); after success redirect to validated `next` (same-origin, allowlisted path prefix `/app`) else `/app`; sign out result display. |
| RBAC | Public. If already signed in: redirect to `/app` (not shown the form). |
| Responsive | Single centered column; 320-safe; button full width. |
| Interaction / motion | M-HOVER, M-ROUTE. No other motion. |
| Loading | Redirecting-to-GitHub state: button S-PENDING, then full navigation. Callback processing page shows "Completing sign-in" status. |
| Empty | N/A. |
| Error | Distinct outcomes (each with its own copy and one recovery): **auth error** (OAuth denied / `state` mismatch / provider failure) → "Sign-in didn't complete" + Try again; **no entitlement** (GitHub user valid but not on allowlist/no role) → "This GitHub account isn't approved for this installation" + who to contact/how access works + no retry loop; **invalid `next`** silently falls back to `/app`; **BFF unavailable** → S-ERR-503. Error text never echoes provider tokens or raw `error_description`. |
| Success | Signed-in redirect; **signed-out outcome** ("You've been signed out") and **session-expired outcome** ("Your session ended. Sign in to continue") shown via `?reason=` values from a fixed allowlist, not free text. |
| Offline | Button disabled with offline reason; text explains. |
| Extra states | 401/403: N/A as route-level (this is the auth surface); 404 global; session expiry is *an outcome here*; masked/too-many/no-worker/no-metrics/expired: N/A. |
| Accessibility | Outcome panel is `role=status` (info) or `role=alert` (error); focus moves to it when present; button has visible text "Sign in with GitHub" (not icon only); no CAPTCHA; no password fields exist anywhere (3.3.8 satisfied). |
| E2E IDs | `E2E-LOGIN-01`, `-04`(BFF 503), `-08`, `-09`, `-10`, `-11`, `-12`; `-20` no password input and no default credentials exist; `-21` signed-in user is redirected from `/login` to `/app`; `-22` `next=https://evil.example` and `next=//evil.example` and `next=/docs` are ignored/clamped; `-23` OAuth `state` mismatch renders auth-error outcome, not session; `-24` non-allowlisted GitHub ID renders no-entitlement outcome, no session cookie set; `-25` `?reason=signed_out|session_expired` render allowlisted copy, arbitrary `reason` renders nothing; `-26` page makes no Go control-plane request. (`-05`,`-06`,`-07` N/A.) |

### 2.14 `/demo` — Isolated demonstration (`DEMO`)

| Field | Contract |
|---|---|
| Purpose | Let ANON evaluators tour a read-only console **without any live system**. Honest about being fixtures (FR-DEMO-01, AC-14). Real engine demonstration is local Docker Compose (linked). |
| Persona | ANON (P-EVAL). |
| Sections | Persistent banner **"Illustrative demo — seeded fixture data, read-only, not live"** (not dismissible); guided tour stepper; read-only console tour screens: overview, jobs list (with failed/dead-letter filter), job detail (attempts, events, a viewer-masked variant toggle "View as: operator / viewer"), workers list/detail, observability charts; "Run the real thing" CTA to quickstart; link `/docs/security` demo-separation. |
| Data source | `FIXTURE` only: typed JSON bundled in the web app, passed through a **separate demo data provider** that implements the same read interface as the console transport abstraction but has no network capability. **Zero control-plane calls and zero BFF API calls** (enforced by test). No timers simulating "live" arrivals: fixture playback is a labelled scripted "Replay" the user starts, with a timestamp-free clock label "Fixture time". |
| Actions | Tour next/prev; navigate fixture screens; filter/sort **client-side over fixtures** (so lists feel real); toggle operator/viewer view (shows masking behavior); replay scripted failure story (worker killed → lease expires → retry → stale result rejected). **No mutation controls exist**: no cancel, retry, drain, create, token. Where the real console has them, the demo shows them as a static annotated "Operators can do X here (not available in demo)" note, not a disabled fake button. |
| RBAC / field scope | ANON. No production data, no credentials, no write routes. Fixture IDs use a visibly synthetic prefix (e.g. `demo-…`) so they cannot be mistaken for real. |
| Responsive | Tour panel collapses to bottom sheet <768; fixture screens use the same responsive components as the console (shared UI primitives, §4) including 320 px card mode; topology replaced by list/summary <768. |
| Interaction / motion | M-ROUTE, M-HOVER, M-OVERLAY, M-ATLAS-style stepper for Replay (user-initiated only), M-TOPO **not used** (no pulses implying live activity). Replay respects reduced motion by stepping without travel. |
| Loading | Fixtures are bundled: no network skeleton. Lazy-loaded chart chunk shows S-LOAD for the chart region only. |
| Empty | Includes a deliberate fixture "empty filter" example (S-EMPTY-FILTER) so the tour shows it; first-use empty also demonstrable through a "fresh install" fixture. |
| Error | Fixture load/parse failure (bundle corruption) → S-ERR-500-style "Demo data failed to load" with reload; no `request_id` (no server). Tour also *shows* representative console error states from fixtures (clearly captioned as examples): S-NO-WORKER, S-NO-METRICS, S-EXPIRED-DATA, S-MASKED, S-409, S-403-ACTION, S-ERR-503, S-OFFLINE/Stale. |
| Success | Replay completion status. |
| Offline | Fully functional offline after load (all local). No offline banner needed; no S-OFFLINE. |
| Extra states | 401/403/session expiry: N/A (anonymous, no session). 404 for unknown fixture ID → in-demo S-404 with back to demo list. Too-many-rows: fixture with 5,000 rows demonstrates S-TOO-MANY. |
| Accessibility | Banner is `role=region` labelled; fixtures labelled in text on every screen heading ("Demo"); operator/viewer toggle is a radiogroup; chart alternatives as tables (same component as console); no "live" or "connected" badge anywhere; live-status indicator is replaced by a fixed `Demo data` pill. |
| E2E IDs | `E2E-DEMO-01`, `-02`(lazy chart only), `-03`, `-04`(fixture parse failure), `-07`, `-09`, `-10`, `-11`, `-12`; `-20` **network trap**: all requests during full tour are same-origin static assets only; zero requests to `/api/*`, BFF routes, control-plane host, WebSocket (AC-14); `-21` no cookies set, none read, no `Authorization` header sent; `-22` banner present on every demo screen and cannot be dismissed; `-23` no element with text "Live", "Connected", or `aria-live` simulated telemetry; `-24` no mutation controls in DOM (query for cancel/retry/drain/create/revoke by role/name finds none); `-25` Operator↔Viewer toggle masks the same fields as console masking spec; `-26` fixture IDs all carry synthetic prefix; `-27` build-time check: `apps/web` demo module imports nothing from the live transport/BFF client package (dependency-boundary lint); `-28` same-origin policy: `/demo` served with CSP `connect-src 'self'`-or-stricter proves no cross-origin fetch possible. (`-05`,`-06`,`-08` N/A.) |

---

## 3. Cross-route surfaces (not §8 routes, but each must be specified)

| Surface | Contract | E2E IDs |
|---|---|---|
| Global 404 (`GLOBAL-404`) | Any unknown URL, in-app and public. Names the path, suggests `/`, `/docs`, and (if signed in) `/app`. Has the full brand shell. HTTP status 404. | `E2E-GLOBAL-404-01` |
| Global error boundary (`GLOBAL-500`) | Uncaught render/server error. Does not leak stack traces; shows digest/request id, Retry and Home. HTTP 500. App shell stays usable. | `E2E-GLOBAL-500-01` |
| Console segment boundaries | Per-route error boundary inside the app shell so sidebar/nav survive a route failure. | `E2E-GLOBAL-500-02` |
| 1.1 routes | `/app/workflows*`, `/app/workflow-runs/*` return the global 404 in 1.0, are absent from nav, sitemap, command search and docs (AC-29). | `E2E-GLOBAL-GATED-01` (404), `-02` (no nav/links/sitemap reference) |
| Session expiry flow | S-SESSION-EXPIRED from any `/app*` route, triggered by 401 on any call or WS close code for auth. | `E2E-GLOBAL-SESSION-01..03` (HTTP 401, WS auth close, reconnect-after-logout rejected per FR-SEC-06 as UI-visible outcome) |
| Mobile menu | Public drawer and app drawer: focus trap, Esc, focus return, route-change closes, 320 px fits. | `E2E-GLOBAL-MENU-01..03` |
| Skip link, landmarks, focus-on-route-change | Present on every route. | `E2E-GLOBAL-A11Y-01..03` |
| Command search (app) | Topbar search/jump: navigates to routes and to a job or worker by **exact ID** (uses `GET /jobs/{id}`, `GET /workers/{id}`); no free-text server search exists in §6. Opens with `Ctrl/Cmd+K`, button always visible. States: unknown ID → inline "No job or worker with that ID", expired → S-EXPIRED-DATA link. | `E2E-GLOBAL-CMD-01..04` |
| Role banner | VIEWER sees a persistent, non-dismissible "Read-only access (viewer)" line in the topbar. | `E2E-GLOBAL-ROLE-01` |
| Theme/contrast | White-first only in 1.0 (no dark-mode requirement); forced-colors respected for badges and focus rings. | `E2E-GLOBAL-A11Y-04` |

### 3.1 Route-to-requirement quick map

`/`, `/architecture`, docs, `/changelog`, `/login`, `/demo` → FR-WEB-01, FR-WEB-03, AC-21; `/demo` → FR-DEMO-01, AC-14; `/login` → FR-WEB-09, FR-SEC-01 (consumer side), AC-13; all `/app*` → FR-WEB-02, FR-WEB-08, FR-RT-01 (where marked), AC-15, AC-16, AC-21; all → NFR-A11Y-01, NFR-RESP-01, FR-WEB-07.

---

## 4. Console routes (v1.0) — 11

Console-wide rules (apply to all `/app*` routes; not repeated):

- Auth: BFF session required. No session → S-401 (redirect to `/login?next=`). Expired mid-use → S-SESSION-EXPIRED.
- Shell: app sidebar (Overview, Jobs, Workers, Observability, Activity, Settings), topbar (breadcrumbs, command search, live indicator, role banner for viewers, user menu → preferences/sign out), `main`.
- Navigation hides operator-only items (Tokens) from VIEWER; direct URL visit shows S-403.
- Live: where a route lists `RT`, it subscribes to hints, treats them only as "refetch this snapshot" triggers, reconciles with server versions (see consumer-requirements §1.7), and shows the live indicator state. Without `RT`, the route refreshes on focus/visibility and a manual Refresh.
- Tables: native `<table>` with caption, sortable headers are buttons with `aria-sort`, row links are real anchors on the primary cell, not whole-row click traps. Cursor pagination is Previous/Next (Previous uses a client-kept cursor stack); no "jump to page N" since cursors are opaque. No fake bulk controls.
- DOM bounds: lists render at most 100 rows per page; "load more" accumulation capped at 500 DOM rows then asks to narrow (S-TOO-MANY). Virtualization only if budgets prove it necessary (keeps native table semantics otherwise).
- Mutation controls: operator only, S-PENDING then S-SUCCESS only after snapshot confirms; 409 → S-409; no optimistic status change.
- Masked fields: S-MASKED everywhere a viewer would see payload, result, logs, error detail, audit detail, credential/token hints.

### 4.1 `/app` — Overview (`APP-HOME`)

| Field | Contract |
|---|---|
| Purpose | Answer in 5 seconds: is the cluster healthy, what is stuck, what needs me. |
| Persona | OPERATOR, VIEWER (P-OPS, P-VIEW). |
| Sections | Live indicator; **KPIs** (queue depth, oldest queued age, running, succeeded, failed over a stated window, workers online/draining/offline, unschedulable count); **topical alerts** (see CCR) — e.g. unschedulable jobs, no capable worker, worker offline, repeated lease expiries, DB not ready; **recent jobs** (last N, status, type, age, links); **throughput chart** (truthful: counted from persisted events, with freshness notice + table alternative); **worker topology** (accessible, list toggle, 1/10/100 aggregation); **operational CTAs** (Submit job [OPERATOR], View failed jobs, View workers; no mutations here beyond Submit). |
| Data source | `API:GET /metrics/overview`, `API:GET /metrics/timeseries`, `API:GET /jobs` (recent, limit bounded, sort newest), `API:GET /workers`; `RT` hints (jobs, workers, metrics). Alerts: `NEED` (CCR-08). |
| Actions | Refresh; navigate; "Submit job" (OPERATOR only; absent for VIEWER); topology/list toggle; time-window selector for the chart (bounded presets). No mutations on this page. |
| RBAC / field scope | VIEWER: all KPI/aggregates visible; recent-jobs rows show no payload/result (not shown on this page for anyone). Submit CTA absent for VIEWER. |
| Responsive | KPIs 4-col → 2-col → 1-col; chart scrolls in-region with table alternative below; topology becomes summary + worker list <768; recent jobs as cards <640; CTAs stay visible. |
| Interaction / motion | M-ROUTE, M-HOVER, M-LIVE on KPI changes (numbers change instantly, no count-up), M-TOPO on active workers (suppressed ≥100 workers). |
| Loading | S-LOAD per region (KPI row, chart, recent jobs, topology load independently; one failing region does not blank the page). |
| Empty | S-EMPTY-FIRST: no jobs ever → "No jobs yet" + (OPERATOR) "Submit a job" / docs quickstart link; no workers → S-NO-WORKER-style onboarding: "No workers registered" + `/docs/workers` and `/app/settings/tokens` hint (OPERATOR). KPIs show "0" only when the API says zero, "—" with reason when unknown. |
| Error | Per-region S-ERR-500/503/TIMEOUT with retry; 503 shows DB-not-ready alert and keeps last data labelled stale. |
| Success | N/A (read-only page). |
| Offline | S-OFFLINE banner + S-STALE labelling on every region ("as of hh:mm:ss"); chart and topology dim; refresh on reconnect. |
| Extra states | S-401 / S-SESSION-EXPIRED; 403: not applicable to the page (both roles allowed) but S-403 shown if role removed; 404 N/A; **S-NO-WORKER** (alert + KPI), **S-NO-METRICS** (chart: three variants), **S-TOO-MANY** N/A (recent list bounded), **S-EXPIRED-DATA** (chart window older than metrics retention → "metrics for this window expired"), S-MASKED N/A. |
| Accessibility | KPI row is a list with each value + label + status text; live changes are *not* announced per KPI; a visually-hidden summary region (`role=status`) announces only alert additions/removals and live-state changes; chart has data table + text summary of trend; topology: SVG with `role=img` summary plus synchronized worker table, keyboard-focusable nodes in the list/graph with arrow-key movement, zoom/pan buttons (2.5.7). |
| E2E IDs | `E2E-APP-HOME-01..12` (standard); `-20` KPIs equal `GET /metrics/overview` values from fixture seed (AC-15, real-state verification in live E2E); `-21` kill a worker → worker KPI and alert update via hint+snapshot (AC-16); `-22` drop WS → indicator goes Reconnecting → Polling, stale label shows, restore → Live + reconciled; `-23` reordered/duplicated hints do not regress displayed state (version check); `-24` unschedulable job → alert + S-NO-WORKER; `-25` no-metrics three variants; `-26` VIEWER sees no Submit CTA; `-27` 1 / 10 / 100 workers topology aggregation + list parity; `-28` one region failing leaves others usable; `-29` chart table equals chart series. |

### 4.2 `/app/jobs` — Jobs list (`JOBS-LIST`)

| Field | Contract |
|---|---|
| Purpose | Find any job fast: delayed, failed, unschedulable, by type/time; drill to detail. |
| Persona | OPERATOR, VIEWER (J-05, J-12). |
| Sections | Filter bar (status multi-select over the 7 states, priority range 0–9, handler type, created/available time window bounded, **"Unschedulable"** reason filter, **"Failed (dead-letter)"** preset = `status=failed`); sort control; results table (ID short + copy, type@version, status badge+text, priority, created, available/run-after, attempts used/max, worker, unschedulable-reason chip, retry-lineage marker); pagination; result-count/capped notice; active-filter chips; "Submit job" (OPERATOR). |
| Data source | `API:GET /jobs` (server-side filter/sort/cursor). Filters and sort live in the URL query string (shareable, back-button safe; no payload data in URL). `RT` job hints → targeted refetch of the current page only. |
| Actions | Filter/sort/paginate; open detail; copy ID; "Submit job" (OPERATOR); Refresh. **No bulk actions** (no fake bulk controls; §8). |
| RBAC / field scope | VIEWER: same columns (none of these are sensitive). Payload/result never in list for any role. Submit CTA absent for VIEWER. |
| Responsive | Filters collapse into a "Filters" dialog/drawer <768 with applied-filter count and chips visible; table → stacked cards <640 (all fields present, ordered by importance: status, type, ID, age first); pagination controls ≥ 24 px targets. |
| Interaction / motion | M-ROUTE, M-HOVER, M-OVERLAY (filter drawer), M-LIVE on changed rows (not rows that merely re-sort: reorders happen only on explicit refresh to avoid under-the-cursor jumps). |
| Loading | S-LOAD table skeleton with fixed row count; filter changes keep previous rows dimmed (`aria-busy`) rather than blanking. |
| Empty | S-EMPTY-FIRST (no jobs ever, CTA by role); S-EMPTY-FILTER (chips + Clear). Dead-letter preset empty: "No failed jobs in this window" (positive framing, not an error). |
| Error | S-ERR-500/503/TIMEOUT with Retry, filters preserved; S-422 for invalid filter in URL (bad status/priority/window) → "Ignored invalid filter" notice and sanitized list; S-429 on aggressive filtering/polling; S-400 unreachable. |
| Success | N/A (read-only). |
| Offline | S-OFFLINE/S-STALE: last page retained with "as of" label; filter changes disabled while offline (cannot serve uncached results truthfully). |
| Extra states | S-401/S-SESSION-EXPIRED (filters kept in URL so return lands in the same view); S-403 N/A (both roles); 404 N/A; **S-TOO-MANY**: result exceeds cap/window → "More than N matches; narrow by status/type/time"; no load-all; **S-NO-WORKER**: unschedulable chip + filter, row tooltip/popover explains reason with link to detail; **S-NO-METRICS** N/A; **S-EXPIRED-DATA**: time window beyond history retention → notice "History older than <window> is removed by retention". |
| Accessibility | `table` caption "Jobs, N shown"; sort buttons `aria-sort`; status = icon + text; filter changes announce result count politely (debounced); focus stays on the control that was changed; pagination is `nav` with labelled Prev/Next and disabled-with-reason. |
| E2E IDs | `E2E-JOBS-LIST-01..12` standard; `-20` server-side filter by each of 7 states (ensures no `unschedulable`/`dead_letter` option exists in status filter); `-21` dead-letter preset equals `status=failed` result set; `-22` sort by each key stable under ties (insert concurrent jobs, no duplicate/skipped rows across pages — AC-15 consistency); `-23` URL round-trip (reload/back preserves filters); `-24` cursor Prev/Next and cap → S-TOO-MANY; `-25` unschedulable filter + reason chip; `-26` no bulk controls in DOM; `-27` VIEWER has no Submit CTA; `-28` row link is a real anchor, keyboard Enter opens detail; `-29` live hint refetches current page without losing scroll/focus; `-30` time window beyond limit rejected by UI and by API 422 renders S-422. |

### 4.3 `/app/jobs/new` — Submit job (`JOBS-NEW`)

| Field | Contract |
|---|---|
| Purpose | Submit one typed, bounded job safely, with clear validation and duplicate handling. |
| Persona | OPERATOR (P-DEV, P-OPS). |
| Sections | Handler select (allowlisted names only) + schema version select; **JSON payload editor** with schema-driven validation (inline errors, size meter against 64 KiB, "Format"), schema/help panel for the selected handler/version; scheduling (run-after with timezone-explicit picker, "Run now" default, bounded future max shown); priority 0–9 (labelled "9 = highest"); max attempts 1–5 (default 3, "includes first run"); timeout; idempotency (namespace + key, "Generate key"); warnings ("Do not put secrets in payloads"; at-least-once reminder); submit; result region. |
| Data source | Handler list/schemas: `NEED` (CCR-02). Submit: `API:POST /jobs`. No `RT`. |
| Actions | Choose handler/version; edit payload; format; pick time; submit; after success, link to the new job and "Submit another" (clears payload, keeps handler). |
| RBAC / field scope | **OPERATOR only.** VIEWER visiting: S-403 (route-level) with link to `/app/jobs`; nav hides it. Direct 403 on submit: S-403-ACTION. |
| Responsive | Single column <1024; editor full width with in-region horizontal scroll and wrap toggle; schema help becomes a disclosure under the editor; sticky submit bar must not obscure focus (2.4.11); date/time picker uses native input fallback on mobile. |
| Interaction / motion | M-ROUTE, M-HOVER, M-OVERLAY (help popover, confirm none). Submit button S-PENDING; no animation gates input. |
| Loading | Handler/schema catalogue S-LOAD (form frame visible, handler select skeleton); editor loads lazily with a plain `<textarea>` fallback so submit never depends on a heavy bundle. |
| Empty | **No handlers available** (catalogue empty or no worker has registered): S-NO-WORKER-style: "No handlers are registered. Jobs would be unschedulable." Submit still allowed only if the API allows queuing for an unregistered handler (contract decision, CCR-02): if not allowed, submit disabled with this reason. |
| Error | S-422 field-level (mapped via `details` JSON pointer to editor line/field), summary region + focus; S-413 (client pre-check at 64 KiB and server 413, shows size and limit); S-409 **idempotency mismatch** (same key, different fingerprint: "This key was used for a different request" + show the existing job link if API returns it + "Use a new key"); S-429 (countdown, submit disabled); S-ERR-503 (submit refused, **never claims queued**, "Nothing was submitted" is shown only when the API said so; on timeout: "Outcome unknown. Check Jobs before retrying, or retry with the same idempotency key" — the UI always sends a key so a retry is safe); S-ERR-500; S-401 → S-SESSION-EXPIRED with **draft retained** (payload kept in `sessionStorage` per tab, cleared on sign-out/success; payload must not contain secrets by policy; flagged for Opus security review). |
| Success | S-SUCCESS: "Job `<id>` accepted (`queued` or `scheduled`)" with link; if the API reports an idempotent replay: distinct message "Same job as before (idempotent replay), no new job created" (signal requested on `POST /jobs`, consumer-requirements section 3.1); status reflects server response only. |
| Offline | Submit disabled when `navigator.onLine=false` with reason; draft preserved; if connection drops mid-submit → timeout path above. |
| Extra states | 404 on handler removed between load and submit → S-422/409 equivalent "Handler no longer available" (code TBD); too-many-rows N/A; no-metrics N/A; expired data N/A; masked N/A; **no-capable-worker**: handler exists in catalogue but no live worker advertises that version → warning "Accepted jobs will be unschedulable until a worker connects" (informational, from CCR-02 data). |
| Accessibility | Every input has a visible label and described help/error; errors summary `role=alert` with links; editor: native textarea semantics preserved when a richer editor is used, Esc does not trap Tab (documented alternative to leave editor); size meter has text value; date/time input has explicit timezone label; no autofocus stealing; required fields marked in text, not only `*`. |
| E2E IDs | `E2E-JOBS-NEW-01..12` (standard; 07 = handler removed); `-20` OPERATOR happy path: submit → 2xx → job appears in `/app/jobs` with real persisted status (AC-15); `-21` VIEWER blocked (route and direct API 403 → S-403-ACTION); `-22` invalid JSON / schema violation → S-422 mapped to field; `-23` payload > 64 KiB blocked client-side and server 413 renders S-413; `-24` same key + same body → replay message, no duplicate; `-25` same key + different body → S-409 mismatch (AC-07); `-26` submit timeout does not display "failed" or "queued"; retry with same key yields same job; `-27` run-after past/too far → validation; timezone-explicit picker, DST test; `-28` priority 0–9 bounds and label; max attempts 1–5; `-29` double-click submit sends one request; `-30` draft survives session expiry and is purged on sign-out; `-31` no handler/empty catalogue state; `-32` no shell/URL/path-like free-form fields exist in the form (AC-12 UI side). |

### 4.4 `/app/jobs/[id]` — Job detail (`JOBS-DETAIL`)

| Field | Contract |
|---|---|
| Purpose | One job's complete truth: status, timing, attempts, lease/cancel/retry state, bounded logs/results, event timeline, lineage, and authorized actions. |
| Persona | OPERATOR, VIEWER (J-03, J-06, J-07, J-09, J-12). |
| Sections | Header (ID + copy, type@version, status badge+text, priority, version); **timestamps** (created, available/run-after, started, finished, hard deadline) in local+UTC; **state panel** (current attempt, `lease_generation`, lease expiry countdown only while running, `cancel_requested_at`, retry-wait availability, **unschedulable reason** if any, failure class); **payload** (RBAC); **result** (RBAC, bounded); **attempts** (card per attempt: number, worker+incarnation, start/end, outcome, error class retryable/permanent, error message RBAC, bounded log RBAC with truncation marker); **event timeline** (versioned, includes diagnostic "stale result rejected" events); **lineage** (retried-from, retried-as links); actions. |
| Data source | `API:GET /jobs/{id}`, `API:GET /jobs/{id}/attempts`, `API:GET /jobs/{id}/events` (cursor, by version); actions `API:POST /jobs/{id}/cancel`, `API:POST /jobs/{id}/retry`. `RT` hint for this job id → refetch job + events since last version. Logs: from attempts payload (retrieval shape: CCR-07). |
| Actions | Copy ID; **Cancel** (OPERATOR; when status in scheduled/queued/retry_wait: immediate; when running: "Request cancellation" with explanation that it is cooperative and a success committed first wins); **Retry** (OPERATOR; only when status is terminal `failed`; creates a **new linked job**, confirm dialog states this, uses idempotent action key; result links to the new job). Refresh timeline; load older events; expand attempt. |
| RBAC / field scope | OPERATOR: full. VIEWER: sees all non-sensitive metadata; **payload, result, attempt error detail, logs, event detail bodies = S-MASKED**; no Cancel/Retry controls (absent). Masking indicated by explicit `masked_fields` from API (consumer-requirements §1.6), never by guessing from empty values. |
| Responsive | Header actions wrap into a primary action + overflow menu <640 (Cancel/Retry remain reachable); attempts as stacked cards; timeline vertical; payload/result/log blocks scroll in own region; sticky action bar only if it does not obscure focus. At 320 px all fields readable without page scroll. |
| Interaction / motion | M-ROUTE, M-HOVER, M-OVERLAY (confirm dialogs: focus trap, restore, Esc cancels), M-LIVE (new timeline rows, status change). Status change announced once politely ("Status changed to succeeded"). Lease countdown updates text each second **without** being announced. |
| Loading | S-LOAD: header skeleton then independent sections (attempts, events) so partial data shows. |
| Empty | No attempts yet (`scheduled`/`queued`): "Not started — waiting for a worker" (or S-NO-WORKER if unschedulable); no events beyond creation never empty (creation event exists); no logs: "No log output" vs masked vs expired are three distinct renderings. |
| Error | S-ERR-500/503/TIMEOUT per section; S-409 on cancel/retry (state changed: "This job already finished as `succeeded`; cancel was not applied" with the refreshed state, no rollback implication); S-429; S-403-ACTION; cancel on terminal job = defined no-op/conflict (§5) rendered as S-409 explanatory, not an error toast; manual retry duplicate (same action key) → shows the already-created linked job; retry not eligible (policy) → S-422/409 with reason. |
| Success | Cancel accepted: for pending jobs status becomes `canceled` after snapshot; for running: "Cancellation requested" state with `cancel_requested_at` and note until final; retry: "New job `<id>` created" link. |
| Offline | S-OFFLINE/S-STALE with "as of"; actions disabled with reason; the lease countdown is replaced by "unknown while offline" (never keeps counting as if current). |
| Extra states | S-401/S-SESSION-EXPIRED; S-403: N/A at route level (both roles) but action-level; **S-404** (unknown ID); **S-EXPIRED-DATA** (job metadata past retention or logs past 7 d: distinct copy, still shows what remains; terminal history immutable while retained, so expiry is removal, not mutation); **S-NO-WORKER** (unschedulable reason block); **S-TOO-MANY** (events > page cap → "Load older"/cap notice, attempts bounded by max 5 so N/A); S-NO-METRICS N/A; S-MASKED for VIEWER. |
| Accessibility | Status text + icon; attempts as `article`s with headings; timeline is an ordered list with `<time>`; confirm dialogs labelled with consequences; masked placeholders have text "Hidden for viewer role"; copy buttons announce; truncation markers are text ("Log truncated at N KiB"). |
| E2E IDs | `E2E-JOBS-DETAIL-01..12` standard; `-20` render per each of 7 states from persisted data (real, AC-15); `-21` attempts list matches persisted attempts including 3-attempt retry exhaustion with no 4th (AC-05 UI side); `-22` cancel pending job → `canceled`; `-23` cancel running job → requested state → final `canceled` or already-`succeeded` conflict (AC-06 UI side); `-24` retry failed → new linked job, old untouched, both show lineage (J-07); `-25` retry on non-failed absent/blocked; `-26` duplicate retry click → single new job; `-27` VIEWER: payload/result/logs masked, actions absent, direct API cancel → S-403-ACTION (AC-13); `-28` stale worker late success shows diagnostic "rejected" event and unchanged final result (AC-03/J-09 UI side); `-29` events versions monotonic, WS reorder/duplicate/gap → reconcile from snapshot (AC-16); `-30` expired logs vs masked vs empty three distinct renderings; `-31` unknown ID → S-404, retention-removed ID → S-EXPIRED-DATA; `-32` lease countdown stops being asserted offline; `-33` over-limit result displays truncation marker; no raw sensitive content in DOM for VIEWER (DOM text scan canary, AC-25). |

### 4.5 `/app/workers` — Workers list (`WORKERS-LIST`)

| Field | Contract |
|---|---|
| Purpose | Fleet inventory and health: who is alive, what they can run, who is draining. |
| Persona | OPERATOR, VIEWER. |
| Sections | Summary strip (online/draining/offline/revoked counts); filters (lifecycle, liveness, handler type, name); **topology/list toggle** (default list; graph aggregates by cluster at ≥10 workers); worker table (name/ID, liveness, lifecycle (enrolled/draining/revoked/…), handlers@versions, capacity (in-flight/capacity), last heartbeat age, protocol/SDK version); degraded banner; no-worker state. |
| Data source | `API:GET /workers` (filters/sort/cursor per consumer-requirements; bounded page; topology needs the whole bounded fleet ≤ stated max). `RT` worker hints → refetch. |
| Actions | Filter/sort; toggle view; open detail; Refresh. (Drain lives on detail to force the confirm context.) |
| RBAC / field scope | Both roles. Credential metadata never shown here. |
| Responsive | Table → cards <640; topology replaced by summary + list <768; filters in drawer <768. |
| Interaction / motion | M-ROUTE, M-HOVER, M-LIVE, M-TOPO (graph view only, active attempts only; suppressed ≥100 workers). Graph supports zoom/pan with buttons + keyboard, cluster expand, detail focus panel. |
| Loading | S-LOAD table skeleton; graph skeleton. |
| Empty | S-EMPTY-FIRST: "No workers have ever registered" + operator link to docs/workers + tokens (OPERATOR); S-EMPTY-FILTER; **degraded** (some workers stale): banner with counts; **no live worker**: if all offline → critical banner + link to unschedulable jobs (`/app/jobs?...unschedulable`). |
| Error | S-ERR-500/503/TIMEOUT; S-429; S-422 invalid filter. |
| Success | N/A. |
| Offline | S-OFFLINE/S-STALE: heartbeat ages **freeze with "as of"** (never keep ticking up as if live); graph dims. |
| Extra states | S-401/S-SESSION-EXPIRED; S-403/404 N/A; **S-TOO-MANY** if fleet exceeds bounded page (topology aggregates and tells user to filter); S-NO-WORKER (the empty/degraded above); no-metrics N/A; expired data N/A (revoked workers retained per retention; disappearance after retention shows in detail as S-EXPIRED-DATA); S-MASKED N/A. |
| Accessibility | Table is the primary, graph is the enhancement (equivalent, synchronized: selecting a node selects the row and vice versa); node focus order logical (clusters then workers); status text; liveness described in words with age ("last heartbeat 12 s ago"); legend component. |
| E2E IDs | `E2E-WORKERS-LIST-01..12` standard; `-20` 1, 10, 100 workers: graph aggregates, list parity, row count = node count (AC-23); `-21` kill worker → liveness changes via hint+snapshot (AC-16); `-22` drain state appears from server, not optimistic; `-23` zoom/pan operable by buttons and keyboard; `-24` all-offline critical banner with link; `-25` revoked worker shown revoked, not "offline"; `-26` filters server-side, URL round-trip; `-27` heartbeat age freezes when offline; `-28` list/graph toggle persists as preference locally and defaults to list on <768. |

### 4.6 `/app/workers/[id]` — Worker detail (`WORKERS-DETAIL`)

| Field | Contract |
|---|---|
| Purpose | Inspect one worker and drain it safely. |
| Persona | OPERATOR, VIEWER (J-08, J-09). |
| Sections | Identity (worker_id, **current incarnation_id**, prior incarnations if exposed), liveness + heartbeat age (distinct from lease renewal wording), lifecycle (enrolled/draining/revoked/disconnected) with timestamps, handlers@versions and capabilities, capacity and in-flight, **active attempts** (job links, attempt number, lease expiry), recent events/logs (bounded), permitted metadata (host/version only, no secrets), drain action. |
| Data source | `API:GET /workers/{id}`; attempts links resolve via `API:GET /jobs/{id}` on navigation; events/logs `NEED` (CCR-05); drain `API:POST /workers/{id}/drain`; `RT`. |
| Actions | **Drain** (OPERATOR; confirmation dialog explains: stops new claims, active attempts finish or recover per documented grace, then cancellation/lease recovery policy; does not physically stop the process; typed confirmation not needed, explicit confirm button + consequence text). Copy IDs; Refresh. **No** revoke control here (credential lifecycle is in `/app/settings/tokens`; linked). |
| RBAC / field scope | OPERATOR: full + Drain. VIEWER: read-only; logs/events detail S-MASKED where sensitive; no Drain. |
| Responsive | Active attempts as cards; drain button stays reachable; metadata as definition list; 320 safe. |
| Interaction / motion | M-ROUTE, M-HOVER, M-OVERLAY (drain dialog), M-LIVE, M-TOPO-style pulse **not** used here (static badge). |
| Loading | S-LOAD per section. |
| Empty | No active attempts: "Idle"; no events: "No recent events"; worker with zero handlers: warning "advertises no handlers". |
| Error | S-ERR-500/503/TIMEOUT; drain: S-409 (already draining/revoked → refreshed state shown), S-403-ACTION, S-429; drain request timeout → "Outcome unknown, refresh before retrying" (drain is idempotent per consumer-requirements). |
| Success | "Drain requested" → state shows `draining` only after snapshot confirms; subsequent completion/state as received. |
| Offline | S-OFFLINE/S-STALE: heartbeat age frozen; Drain disabled. |
| Extra states | S-401/S-SESSION-EXPIRED; S-403 N/A route-level; **S-404** unknown ID; **S-EXPIRED-DATA** retention-removed worker record; **disconnected** state (no heartbeat beyond threshold: "No heartbeat since <time>. Its in-flight work will recover when leases expire" — truthful, no claim it stopped); **revoked** state (revoked_at shown; "Revocation blocks new claims, renewals and results; external work may still be running until it notices" per §4); no-metrics N/A; too-many N/A (events paged); S-MASKED. |
| Accessibility | Dialog labelled with consequence text; state words in text; heartbeat age text not color; definition lists. |
| E2E IDs | `E2E-WORKERS-DETAIL-01..12` standard; `-20` OPERATOR drain → claims stop, active attempts finish or recover (AC-11 UI side), state transitions shown from server; `-21` drain on already-draining → S-409 refreshed; `-22` VIEWER: no Drain, direct API drain → S-403-ACTION (AC-13); `-23` disconnected worker copy (no "stopped" claim); `-24` revoked worker disables Drain, shows revoked text and bound; `-25` active attempts list matches running attempts, links work; `-26` unknown worker → S-404, removed by retention → S-EXPIRED-DATA; `-27` incarnation change after restart displayed (new incarnation distinguishable); `-28` drain timeout outcome-unknown path. |

### 4.7 `/app/observability` — Observability (`OBS`)

| Field | Contract |
|---|---|
| Purpose | Truthful charts of throughput, queue age, attempts, failures, duration over a chosen range, with disclosed freshness. |
| Persona | OPERATOR, VIEWER (J-11). |
| Sections | Controls (time range presets bounded, aggregation/step, metric set); freshness notice ("data as of hh:mm:ss, aggregation interval N, derived from persisted events"); charts: **throughput**, **queue depth/age**, **attempts**, **failures**, **duration (p50/p95)**; **data table** under each chart (same series); optional CSV download only if the API/transform supports it (client-side CSV of the displayed series is allowed; labelled "displayed series"); legend; infrastructure-vs-job-truth note. |
| Data source | `API:GET /metrics/timeseries` (+ `API:GET /metrics/overview` for current freshness/summary). `RT` metrics hints → debounced refetch (≥ step). |
| Actions | Change range/step/series; toggle table/chart; download displayed series (client-side); Refresh. No mutation. |
| RBAC / field scope | Both; aggregates only (no job payload data in metrics). |
| Responsive | Charts full width; legend below; controls in a disclosure <768; tables stacked or scroll-in-region; touch-friendly point inspection via table rows (no hover dependency). |
| Interaction / motion | M-ROUTE, M-HOVER; chart transitions: none beyond data swap (no animated redraw looping); tooltips via focus and hover. |
| Loading | S-LOAD per chart; range change keeps prior chart dimmed. |
| Empty | **S-NO-METRICS**: (a) no data in range → "No jobs finished in this range" (zeros only if API returns zeros; gaps shown as gaps); (b) source unavailable (503) → "Metrics are unavailable right now"; (c) not yet aggregated/range too fresh → "Not enough data yet". |
| Error | S-ERR-500/503/TIMEOUT per chart; S-422 invalid range (too wide, bad step) → message with limit + auto-clamp offer; S-429. |
| Success | N/A. |
| Offline | S-OFFLINE/S-STALE: "as of" on every chart. |
| Extra states | S-401/S-SESSION-EXPIRED; S-403/S-404 N/A; **S-EXPIRED-DATA**: range older than metrics retention; **S-TOO-MANY**: too many points (window/step) → UI enforces max points and tells user to widen step; S-NO-WORKER N/A; S-MASKED N/A. |
| Accessibility | Every chart: heading, text summary (min/max/last/trend), data table, series identified by pattern + label (not color alone), keyboard focus on data points via table, legend accessible; reduced-motion redraw instant; chart is `role=img` with `aria-describedby` summary, table is the interactive alternative. |
| E2E IDs | `E2E-OBS-01..12` standard; `-20` chart series equals table equals API response (AC-15); `-21` 1,000-job fixture reconciles totals in throughput series (AC-18 UI side); `-22` three S-NO-METRICS variants distinct; `-23` freshness notice present and updates after refetch; `-24` range beyond bound → S-422/clamp; `-25` expired range → S-EXPIRED-DATA; `-26` gaps are not drawn as zeros; `-27` CSV equals displayed series; `-28` no point inspection depends on hover only; `-29` axe + table keyboard walk (chart alternative). |

### 4.8 `/app/activity` — Activity (`ACTIVITY`)

| Field | Contract |
|---|---|
| Purpose | Chronology of who/what did what: job events and privileged audit actions, with correlation. |
| Persona | OPERATOR, VIEWER (VIEWER sees masked details). |
| Sections | Filters (event kind: job event / audit; actor; action; outcome; target type; time window); chronological table (time, actor, action, target link, outcome (success/denied/rejected), correlation/`request_id`, detail); pagination. |
| Data source | `API:GET /audit` (privileged audit). Cross-job job-event chronology: `NEED` (CCR-06); until decided, the page shows audit only and per-job events stay on job detail (the combined view is omitted, not faked). `RT` optional (new-activity hint → "N new events" button, not auto-prepend, to avoid under-cursor jumps). |
| Actions | Filter, paginate, open target, copy correlation ID, "Show new" button. |
| RBAC / field scope | Whether VIEWER may read audit at all is an open access-matrix question (OQ-03). Contract assumption: VIEWER can read the chronology; actor identity and detail bodies that could expose sensitive data (token names, payload fragments, IP) are S-MASKED. OPERATOR full. |
| Responsive | Table → cards <640, all fields present, correlation ID wraps with copy; filters in drawer <768. |
| Interaction / motion | M-ROUTE, M-HOVER, M-LIVE (on "Show new"). |
| Loading | S-LOAD table skeleton. |
| Empty | S-EMPTY-FIRST ("No activity recorded yet"), S-EMPTY-FILTER. |
| Error | S-ERR-500/503/TIMEOUT; S-422 invalid filter/window; S-429; S-403 route-level if matrix denies VIEWER (then nav hides it). |
| Success | N/A. |
| Offline | S-OFFLINE/S-STALE "as of". |
| Extra states | S-401/S-SESSION-EXPIRED; S-403 (if VIEWER denied); 404 N/A; **S-TOO-MANY**: window/page cap; **S-EXPIRED-DATA**: audit older than 90 d / events older than history retention (exact windows per ADR) → "Older entries removed by retention"; S-NO-METRICS/S-NO-WORKER N/A; S-MASKED. |
| Accessibility | Table semantics; outcome text+icon; "N new events" is a button announced politely once; correlation ID monospace with copy. |
| E2E IDs | `E2E-ACTIVITY-01..12` standard; `-20` privileged actions (cancel, retry, drain, token issue/revoke) each produce a visible entry with actor/outcome (AC-13/FR-SEC-11); `-21` denied attempt recorded as outcome `denied`; `-22` VIEWER details masked; `-23` filters server-side, URL round-trip; `-24` retention-expired window → S-EXPIRED-DATA; `-25` "Show new" does not shift focus/scroll; `-26` correlation ID matches the `request_id` shown on corresponding error state. |

### 4.9 `/app/settings` — Settings (`SETTINGS`)

| Field | Contract |
|---|---|
| Purpose | Read-only operational configuration and documented limits; hub to role-appropriate settings. |
| Persona | OPERATOR, VIEWER. |
| Sections | Environment (name, version, API version/contract SHA, build); **limits** (payload 64 KiB, result 128 KiB, max attempts 1–5 incl. first, priority 0–9 (9 highest), idempotency retention, max future `run_after`, page-size cap); **retention** (logs/history/audit windows as configured); timing (heartbeat, lease, grace as configured, labelled "configured values, not guarantees"); links: Tokens (OPERATOR), Preferences, `/docs/*`. |
| Data source | Live config: `NEED` (CCR-04). Until the endpoint exists, the page shows **static documented defaults** from the build, with a visible note "Documented defaults; this installation's configured values are not yet readable" and **omits** the Environment/live-values section. |
| Actions | Navigation links only. No editing (read-only per §8). |
| RBAC / field scope | Both roles. Tokens link absent for VIEWER. No secrets/connection strings ever displayed. |
| Responsive | Definition lists stack; 320 safe. |
| Interaction / motion | M-ROUTE, M-HOVER. |
| Loading | S-LOAD for live config block. |
| Empty | N/A (always has documented defaults). Live config unavailable → degrade to static + note. |
| Error | S-ERR-500/503/TIMEOUT for live block only; static block unaffected. |
| Success | N/A. |
| Offline | Static block usable; live block S-STALE. |
| Extra states | S-401/S-SESSION-EXPIRED; S-403/404 N/A; too-many, no-worker, no-metrics, expired N/A; S-MASKED N/A. |
| Accessibility | Definition lists (`dl`), units in text, links named by destination. |
| E2E IDs | `E2E-SETTINGS-01..02, -04, -05, -08..12`; `-20` displayed limits equal the contract/API values; `-21` Tokens link absent for VIEWER; `-22` no secret-looking values in DOM (canary); `-23` static-fallback note appears when live config endpoint absent. (`-03`,`-06`,`-07` N/A.) |

### 4.10 `/app/settings/tokens` — API tokens and worker credentials (`TOKENS`)

| Field | Contract |
|---|---|
| Purpose | Operator-only issuance, rotation and revocation of direct-API tokens and worker credentials with one-time secret display. |
| Persona | OPERATOR (J-10, J-02). |
| Sections | Tabs/segments: **API tokens**, **Worker credentials**; list (name, scope, created, expiry, last used, status active/revoked/expired, non-secret identifier only); create form (name, scope selection, expiry); **one-time reveal dialog**; rotate; revoke (confirm with consequence text, incl. revocation enforcement bound for worker credentials); audit link per item (`/app/activity?...`). |
| Data source | **All operations `NEED` (CCR-01):** §6 REST has no token/credential endpoints. Route is specified but **not buildable** until CCR-01 is resolved; nav shows the route only when the capability exists. |
| Actions | Create, rotate, revoke, copy secret (once), acknowledge. |
| RBAC / field scope | **OPERATOR only.** VIEWER: S-403 route-level, nav item absent, direct API denies (AC-13). Secret material exists only in the creation response. |
| Responsive | List → cards <640; reveal dialog full-screen sheet <480 with copy button reachable; secret wraps (no truncation of the value being copied). |
| Interaction / motion | M-ROUTE, M-HOVER, M-OVERLAY. No animation on reveal. |
| Loading | S-LOAD list skeleton; create S-PENDING. |
| Empty | S-EMPTY-FIRST: "No API tokens yet" / "No worker credentials yet" + create CTA + docs link. |
| Error | S-422 (name/scope/expiry), S-409 (name conflict / already revoked), S-429, S-ERR-503; **create outcome unknown (timeout after send)**: "We couldn't confirm whether a credential was created. Check the list; if one appeared you cannot view its secret again, rotate it." Creation must be idempotent-keyed (CCR-01). S-403-ACTION if role revoked. |
| Success | One-time reveal: secret displayed once in a dialog (not in URL, not in history, not in storage, not in toast, not in logs/analytics); explicit "I have stored this secret" checkbox required to close; Copy confirms politely; after close the secret is unrecoverable: list shows only the non-secret identifier. Revoke success shows status `revoked` from snapshot; list row remains with revoked text. |
| Offline | Mutations disabled; list S-STALE. |
| Extra states | S-401/S-SESSION-EXPIRED (**if expiry happens while a secret is on screen, the dialog persists until acknowledged; no secret re-display later**); S-403; 404 for stale item ID on revoke → S-409/refresh; too-many N/A (bounded list); no-metrics/no-worker N/A; expired items listed as `expired` status; S-MASKED N/A (no secret ever returned after creation). |
| Accessibility | Secret rendered in a read-only text field with `autocomplete=off`, `spellcheck=false`, labelled; copy button; dialog `role=alertdialog`, focus trapped, title states "shown once"; confirmation text explicit about consequences; no secret in `aria-live` or document title; screen-reader users get the secret via the labelled field. |
| E2E IDs | `E2E-TOKENS-01..12` standard; `-20` create → one-time reveal → close → secret never appears again in DOM, network, storage, URL, history, console (secret canary, AC-25); `-21` rotate shows new one-time secret, old marked revoked; `-22` revoke worker credential → worker rejected for new claim/renew/report (AC-09 UI+API) and state shown; `-23` VIEWER blocked (route S-403, direct API 403, AC-13); `-24` unknown-outcome create path; `-25` close blocked until acknowledged; `-26` every issue/revoke appears in `/app/activity` (FR-SEC-11); `-27` secret not announced via live region or present in page title; `-28` session expiry while revealed keeps dialog; `-29` duplicate create click yields one credential (idempotency). |

### 4.11 `/app/settings/preferences` — Preferences (`PREFS`)

| Field | Contract |
|---|---|
| Purpose | Per-user display preferences and session actions. |
| Persona | OPERATOR, VIEWER. |
| Sections | **Timezone** (default "Browser: <zone>", or explicit IANA zone; live preview of a sample timestamp in local + UTC); **Motion** (shows system `prefers-reduced-motion` value; optional app override "Reduce motion: system / always"; override only *adds* reduction, never forces motion on); **Session** (signed in as GitHub login + role, session expiry time, Sign out). Density preference **omitted in 1.0** (§8 "only if implemented"). |
| Data source | `BFF-AUTH` for session info and sign-out. Timezone and motion override: **browser-local storage only** (per device; the page says so). No Go REST call, no new endpoint. |
| Actions | Choose timezone; choose motion override; Sign out (ends BFF session, redirects `/login?reason=signed_out`). |
| RBAC / field scope | Both roles. Shows own principal only. |
| Responsive | Single column; 320 safe. |
| Interaction / motion | M-ROUTE, M-HOVER; preference change applies immediately. |
| Loading | Session block S-LOAD; local prefs synchronous. |
| Empty | N/A. |
| Error | Storage blocked/unavailable → "Preferences can't be saved on this browser; using defaults" (functional with in-memory only); session info fetch S-ERR-503; sign-out failure → message + retry (cookie cleared client-side not assumed). |
| Success | Preference saved: polite status; sign out → redirect with outcome. |
| Offline | Local prefs work; Sign out requires network: if offline, message explains the session remains valid until sign-out completes. |
| Extra states | S-401/S-SESSION-EXPIRED; S-403/404 N/A; others N/A. |
| Accessibility | Timezone select is native with search-by-typing, labelled; sample time updates via polite status on change; motion override radio group; Sign out is a button (not a link) with clear label. |
| E2E IDs | `E2E-PREFS-01, -04, -05, -08..12`; `-20` timezone change re-renders all `<Timestamp>`s across routes (local + UTC correct); `-21` DST boundary cases in two zones (spring-forward, fall-back ambiguity labelled with offset); `-22` motion override reduces, system reduced-motion respected when override is "system"; `-23` storage-disabled fallback works; `-24` sign out ends session, back/refresh does not restore data, WS reconnect rejected (FR-SEC-06 UI side); `-25` session info shows only own identity. (`-02`,`-03`,`-06`,`-07` N/A.) |

---

## 5. 1.1 routes (gated / absent in 1.0)

| Route | 1.0 behavior | Gate to enable |
|---|---|---|
| `/app/workflows` | Not built. Global 404. Not in nav, sitemap, command search, docs or marketing. | Versioned DAG engine, real API and tests shipped (M10). |
| `/app/workflows/[id]` | Same. | Same. |
| `/app/workflow-runs/[id]` | Same. | Same. |

Tests: `E2E-GLOBAL-GATED-01/02` (AC-29). States `blocked` and `skipped` are reserved for 1.1 and must not appear in any 1.0 filter, badge legend, or docs table except the "reserved for 1.1" note in `/docs/concepts`.

## 6. Open items affecting this inventory

See `consumer-requirements.md` section 8 (Contract change requests) and section 9 (Open questions). Routes blocked or degraded until a CCR is decided: `/app/settings/tokens` (CCR-01, blocked), `/app/jobs/new` (CCR-02 handler catalogue), `/app/workers/[id]` events/logs (CCR-05), `/app/activity` cross-job events (CCR-06), `/app/settings` live config (CCR-04), `/app` alerts (CCR-08). All `RT` behavior depends on CCR-03 (polling-only fallback otherwise); `/login`, session/role gating and `/app/settings/preferences` session block depend on CCR-09 (BFF contract).
