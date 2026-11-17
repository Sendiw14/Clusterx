# CLUSTERX — Consumer Requirements (frontend → API contract, M1 input)

- Ticket: TASK-M1-FE-01 · Lane: frontend (`m1/frontend`) · Author model: `claude-sonnet-5-5`, effort high (requested)
- Source contract: `docs/product/MASTER_PROMPT.md` R2 @ `5a5ca99eb44145100044bf9517e55813cb40903c`. §5 states, §6 REST/realtime, §4 trust boundary, §8 routes.
- Protocol step: §1.3 step 2 (frontend states consumer requirements). Backend drafts OpenAPI/proto; TASK-M1-FE-02 signs the drafts against this file.
- Status: **specified**. This document lists **needs**, not endpoint designs. Names such as `code` values are placeholders for the backend to choose; the UI will key on whatever stable machine `code` the contract defines, never on `message` text.
- Companion: `docs/design/route-inventory.md` (route contracts and E2E IDs).

## 0. How to read this

Tags:

- **[§6]** the operation already exists in the master §6 REST list; the item is a field/filter/semantic the UI needs the OpenAPI to state.
- **[NEED]** an additional field, filter, header, or error code on an existing operation.
- **[CCR-nn]** the UI needs a capability for which §6 has **no operation**. Frontend is *requesting a change*; it does not propose the route, verb or schema. See section 8.
- **[BFF]** owned by the Next.js BFF contract (Opus owns auth contract, Sonnet implements UI). Not a Go REST operation.

Job states are exactly the seven in §5: `scheduled`, `queued`, `running`, `retry_wait`, `succeeded`, `failed`, `canceled`. **`unschedulable` is a reason, not a state. Dead-letter is a filtered terminal `failed` view.** `blocked`/`skipped` are 1.1-reserved and never appear in 1.0 UI.

Frontend guarantees in return: generated TS client only (no hand-written fetch wrappers for API operations), page logic behind a transport abstraction, mocks labelled and never marked integrated, `/demo` uses no transport at all.

---

## 1. Cross-cutting requirements

### 1.1 Transport, identity, and what the browser can see

- Browser calls only the Next.js BFF; BFF calls Go `/api/v1` with a short-lived audience-bound service assertion (§4). The browser never holds Go tokens, worker credentials or the OAuth tokens.
- The UI needs the **effective role** (`operator` | `viewer`) per session to decide which controls to render. This is a UX hint only; Go enforces. Source: [BFF] (CCR-09). Go responses must still return 403 for every denied call regardless of what the UI hid.
- Mutations from the browser go through BFF with CSRF/origin protections (web lane implements; contract owner Opus). Frontend needs each mutating operation to be **safe to retry** under the idempotency rules in 1.7.

### 1.2 Identifiers, time, encoding

- IDs are opaque strings to the UI (no parsing, no ordering assumptions except where a sort key says so). Need: max length stated in OpenAPI so layouts can be bounded.
- All timestamps: RFC 3339 **UTC** with `Z`. Durations: integer milliseconds (name carries the unit). No local-time strings from the API.
- Every response that returns a collection or aggregate carries `as_of` (database-clock time the snapshot was taken). The UI displays staleness from `as_of`, never from its own receipt time alone (§5 "database clock is authoritative").
- JSON payload/result are returned as JSON values (not double-encoded strings) when unmasked, and are bounded (64 KiB / 128 KiB). UTF-8 only.

### 1.3 Error model and distinct renderings

All errors `{ code, message, details?, request_id }` (§6). UI requirements:

1. `request_id` present on **every** error body and echoed in a response header, so UI can show a copyable ID even when the body cannot be parsed (e.g. proxy 502/503).
2. `details` for 422 carries **per-field errors with a JSON-pointer-style path** into the request body (so the editor can highlight the exact field/line) plus a machine reason code per field.
3. `message` is human-readable English but treated as non-contractual (UI renders its own copy from `code`; may show `message` as secondary detail, escaped).
4. Distinct renderings required. The UI cannot render two items below identically, so the contract must let it tell them apart by `code` (status alone is insufficient):

| # | UI state | HTTP | Distinction the UI needs (placeholder name for the backend to define) |
|---|---|---|---|
| E-01 | S-400 malformed | 400 | `bad_request` (should be unreachable from UI; reported as bug) |
| E-02 | S-401 / session expired | 401 | `unauthenticated` vs `session_expired` — the BFF may produce these; Go-originated auth failures must not be conflated with "BFF session ended" (OQ-02) |
| E-03 | S-403 forbidden route/action | 403 | `forbidden_role` (role lacks operation); field-level restriction is expressed by masking (1.6), not by errors |
| E-04 | S-404 not found | 404 | `not_found` (never existed or not visible) |
| E-05 | S-EXPIRED-DATA | 404 (or 410 — backend decides, OQ-10) | `expired_by_retention` **distinct** from `not_found`, ideally with the retention window and `expired_at` in `details` |
| E-06 | S-409 job/worker state conflict | 409 | `state_conflict` with current `status`/`version` in `details` (e.g. cancel on `succeeded`, retry on non-`failed`, drain on revoked/already draining) |
| E-07 | S-409 idempotency mismatch | 409 | `idempotency_key_mismatch` **distinct from** E-06, with the existing job's ID in `details` when the caller may see it |
| E-08 | S-413 too large | 413 | `payload_too_large` with `limit_bytes` and (if known) `received_bytes` |
| E-09 | S-422 validation | 422 | `validation_failed` with per-field `details` (see 2) |
| E-10 | S-422 domain rejections | 422 | Distinct codes the UI shows with specific copy: `handler_unknown`, `handler_version_unsupported`, `run_after_out_of_bounds`, `retry_not_eligible`, `time_window_too_wide`, `page_size_too_large` |
| E-11 | S-429 rate limited | 429 | `rate_limited` + `Retry-After` header (seconds) |
| E-12 | S-ERR-503 unavailable | 503 | `unavailable` (e.g. DB not ready; coordinator draining) + `Retry-After` when known; **must not be used for 500-class bugs** |
| E-13 | S-ERR-500 | 500 | `internal` with `request_id`; no stack traces |

5. Requirement for AC-01/AC-08 consumer side: on any 5xx or timeout for `POST /jobs`, the response (or its absence) must never be interpreted as "accepted". The UI will only show "accepted" on a 2xx body containing the job. Combined with 1.7 idempotency, a retry is safe.

### 1.4 Pagination, filter, sort

For `GET /jobs`, `GET /workers`, `GET /jobs/{id}/events`, `GET /audit` (§6: cursor-paginated, stable sort/tie-break, bounded page and window):

- Request: `limit` (UI sends 25/50/100; **max stated in OpenAPI**, OQ-06), opaque `cursor`, filter params, `sort` + direction from a documented closed list.
- Response: items, `next_cursor` (absent when exhausted), `has_more`, `as_of`.
- **No total count required.** If cheap, an optional `approx_total` with a `total_capped: true` marker is useful for "more than N matches" (S-TOO-MANY). The UI never shows an exact total it cannot trust under concurrent inserts.
- **Stability under concurrent inserts:** walking pages with a fixed sort must not duplicate or skip existing rows; new rows may be missed. State this guarantee in the contract; UI E2E `E2E-JOBS-LIST-22` verifies it. Sort keys must be **immutable for a row's lifetime** or the contract must say how a row whose key changes between pages is handled (OQ-07; `available_at` changes on retry).
- Cursors are opaque and may expire: an expired/invalid cursor needs a distinct `code` (`cursor_invalid`) so the UI restarts from page one with a notice rather than showing an error wall.
- Previous page: UI keeps its own cursor stack; no backward cursor needed.
- Windows: any time filter has a **documented maximum span** and a retention floor; violations are E-10 `time_window_too_wide`, not silent clamping, so the UI can message precisely.
- Unknown filter values → 422 (E-09) with the offending param path, so the UI can drop that filter and tell the user (`E2E-JOBS-LIST-30`).

### 1.5 Bounds the UI mirrors (must be machine-readable or in OpenAPI)

Payload 64 KiB; result 128 KiB; priority 0–9; `max_attempts` 1–5; max future `run_after`; idempotency key max length and charset; idempotency retention; log bytes per attempt; events page cap; max workers returnable for topology; metrics max points. UI pre-checks what it can for UX only; Go is authoritative. The values reach the UI from the OpenAPI schema or CCR-04 (never hard-coded silently: where static documented defaults are shown they are labelled as such).

### 1.6 Viewer field masking (S-MASKED)

Need a single, uniform representation so the UI never guesses:

- Every object that can contain a viewer-restricted field carries `masked_fields`: an array of field names (e.g. `["payload","result"]`) that **are restricted for this caller and were omitted/blanked**. Absence of the array means nothing masked.
- A masked field's value is **omitted or null, and the UI keys only on `masked_fields`**, so "masked" is distinguishable from "empty", "expired" and "not produced yet". Three states must be separable per field: *masked*, *empty/absent*, *expired* (the last via `expired_fields` or an equivalent marker, relevant to logs 7 d).
- Fields the UI expects to be maskable for VIEWER (access-matrix owner confirms; UI renders whatever `masked_fields` says):

| Object | Maskable fields |
|---|---|
| Job | `payload`, `result`, `idempotency_key`/namespace (sensitive correlation), `requested_by` (actor identity, OQ-03) |
| Attempt | `error.message`/`error.detail`, `logs` |
| Job event | `detail` bodies carrying payload/error fragments |
| Worker | metadata beyond non-sensitive host/version/liveness; any addressing information |
| Audit entry | `actor` identity detail, `detail`, source address |
| Token / credential | n/a for VIEWER (route denied); for OPERATOR secret is returned once, ever (CCR-01) |

- Raw sensitive content must not be sent to the browser and then hidden by CSS: masking is server-side (AC-25 DOM canary test depends on it).
- The same `masked_fields` mechanism is used by `/demo` fixtures to show the viewer rendering, without any API.

### 1.7 Versions, idempotency and reconciliation

Goal: the UI can safely merge responses that arrive out of order, duplicated, or after a WebSocket gap, and never regress displayed state (AC-16).

- **Job:** `version` (monotonic, §5) on every job representation (detail and list items). Rule frozen on the UI side: *an incoming job with `version` lower than the held one is discarded; equal is a no-op; higher replaces.* Needs: version increments on **every** state-relevant change (status, cancel_requested_at, current_attempt, lease generation, unschedulable reason).
- **Job events:** per-job `version` contiguous and strictly increasing (§6). The UI treats a hole (e.g. 7 then 9) as a gap and refetches from the last contiguous version. Need: events listing supports "after version N" (cursor semantics or a version filter) to resume cheaply.
- **Workers:** §6 only mandates `jobs.version`. The UI needs an equivalent monotonic ordering token for worker representations (`version` or `updated_at` with strict ordering), so reordered snapshots/hints cannot show an older liveness/drain state as current (NEED on `GET /workers`, `GET /workers/{id}`).
- **Collections/aggregates:** `as_of` (see 1.2). The UI keeps the newest `as_of` per resource and discards older responses.
- **Mutation idempotency (client side keys):**
  - `POST /jobs`: idempotency scope key/namespace (§5) — UI always sends one so timeouts are retry-safe. Response must **signal replay** (e.g. distinct status code or a boolean) so the UI can say "same job as before" (OQ-12).
  - `POST /jobs/{id}/retry`: idempotent action key (§5). Repeat with same key returns the same new linked job, not a second one.
  - `POST /jobs/{id}/cancel`: naturally idempotent; repeated call on an already canceled/cancel-requested job returns a non-error current snapshot or a defined 409 (UI needs which).
  - `POST /workers/{id}/drain`: idempotent; repeat returns current worker snapshot.
  - Where the key is carried (header vs body) is the backend's decision; frontend just needs it documented.
- **Mutation responses carry the post-state:** cancel, retry, drain return the resulting snapshot (with version). On 409 `state_conflict`, `details` carries the current `status` and `version` so the UI can render "already finished as succeeded" immediately without a second request.

### 1.8 Realtime hint shape (FR-RT-01, hints only)

`GET`/WebSocket upgrade path, auth mechanism, subscription scopes: **CCR-03** (§6 names WebSockets but defines nothing). Consumer requirements on the message shape, independent of transport details:

```jsonc
// server -> client, JSON text frame. A HINT: no authoritative data.
{
  "type": "hint",
  "v": 1,                          // schema version, additive-only
  "seq": 1042,                     // per-connection monotonic; UI detects gaps/reorder locally
  "resource": "job",               // job | worker | metrics | audit
  "id": "…",                       // resource id; omitted for metrics/audit
  "version": 17,                   // OPTIONAL resource version at emit time (job/worker); never a substitute for a snapshot
  "emitted_at": "2026-11-17T10:00:00Z"
}
```

Control frames the UI needs distinguishable from hints: `hello` (carries `server_time` and heartbeat interval so the UI can detect silence), `resync` (server asks all clients to snapshot because it dropped/coalesced hints), `auth_expiring` (so the UI can show S-SESSION-EXPIRED gracefully before the close). Requirements:

1. Hints contain **no payload, result, log or status values**. The UI will refetch; it never renders from a hint. (Prevents masking bypass over WS and prevents "fake live" rendering.)
2. Hint volume is server-coalesced (at most one hint per resource per interval) so the client is not driven by a hot loop (§10).
3. Subscription scoping is authorized by Go per principal/role; a viewer must not receive hints for resources it may not read (even hints leak existence).
4. **Close codes** distinguish: normal, server restart, auth failure/expired session (UI → S-SESSION-EXPIRED, no reconnect loop), origin rejected, over-limit. Reconnect after logout must be rejected with the auth close code (FR-SEC-06).
5. Gap handling expected by the UI: on connect, reconnect, `seq` gap/reorder, `resync`, or `hello` silence beyond 2× heartbeat → UI sets **Reconnecting/Stale**, fetches snapshots (with versions per 1.7), then returns to **Live**. If WS unavailable → **Polling** at a bounded interval (interval and per-user request budget to be fixed with backend, OQ-05), pauses when tab hidden.
6. Test needs: the contract provides a way to run deterministic drop/reorder/duplicate in E2E (backend test hook or recorded frames) for `E2E-APP-HOME-22/-23`, `E2E-JOBS-DETAIL-29`.

### 1.9 Headers and caching

- `Retry-After` on 429 and 503. `X-Request-Id` (or documented name) on all responses; the UI sends none and uses the server's.
- Authenticated API responses `Cache-Control: no-store` (job payloads, masked data). Server-state in the UI cache is memory only; never `localStorage`.
- Rate limits: the UI needs the documented per-principal budget to size polling and live refetch; optionally `RateLimit-*` headers for display (not required).

### 1.10 Job vocabulary the UI renders (so the OpenAPI enums match)

- `status`: the 7 states. UI badge legend and filters use exactly these.
- `unschedulable_reason`: nullable structured field on the job (`code` + human `detail` + the required handler type/version + count of workers that advertise the type but not the version or that are draining/revoked). **A field, not a status.** Filterable on `GET /jobs` (`unschedulable=true` or by reason code).
- `failure_class` on terminal `failed` jobs and failed attempts: distinguishes `permanent`, `retries_exhausted`, `timeout`, `lease_expired`, (others backend defines) so the dead-letter view (`status=failed`) can be understood without opening each job (OQ-08). Retryable vs permanent classification per attempt.
- `cancel_requested_at`, `hard_deadline_at`, `current_attempt_id`, `lease_generation`, `available_at`, `retried_from_job_id` (and the reverse lineage: ID(s) of jobs created by manual retry — **NEED** `retried_as_job_id`/list, since the UI shows "retried as" on the old failed job).

---

## 2. Route to operation matrix

Legend: R = reads, W = mutates, RT = hint subscription, — none. Operation tags `NEED` mean the section 3 entry lists additions.

| Route | Operations | RT |
|---|---|---|
| `/app` | `GET /metrics/overview`, `GET /metrics/timeseries`, `GET /jobs` (recent), `GET /workers`; alerts CCR-08 | jobs, workers, metrics |
| `/app/jobs` | `GET /jobs` | jobs (list) |
| `/app/jobs/new` | `POST /jobs` (W); handler catalogue CCR-02 | — |
| `/app/jobs/[id]` | `GET /jobs/{id}`, `GET /jobs/{id}/attempts`, `GET /jobs/{id}/events`, `POST /jobs/{id}/cancel` (W), `POST /jobs/{id}/retry` (W); logs CCR-07 | job:{id} |
| `/app/workers` | `GET /workers` | workers |
| `/app/workers/[id]` | `GET /workers/{id}`, `POST /workers/{id}/drain` (W); events/logs CCR-05 | worker:{id} |
| `/app/observability` | `GET /metrics/timeseries`, `GET /metrics/overview` | metrics |
| `/app/activity` | `GET /audit`; cross-job events CCR-06 | audit |
| `/app/settings` | config CCR-04 | — |
| `/app/settings/tokens` | CCR-01 (all) | — |
| `/app/settings/preferences` | [BFF] session info/sign-out; local storage | — |
| `/login` | [BFF] OAuth start/callback/session | — |
| Global (all `/app*`) | `GET /jobs/{id}`, `GET /workers/{id}` (command search by exact ID); [BFF] session; CCR-03 WS | — |
| `/demo` | **none** (`FIXTURE`, section 5) | **none** |
| Public static routes | **none** (build-time inputs, section 6) | none |

Frontend never calls `/health/live`, `/health/ready` or `/metrics` (Prometheus) from the browser. Readiness is surfaced to the UI only as 503 `unavailable` on the real operations.

---

## 3. Per-operation needs

For each §6 operation: the fields, filters, sorts, versions, errors and masking the UI needs. Anything not listed is not needed by the UI.

### 3.1 `POST /jobs`  [§6]

- **Request the UI sends:** handler `type` + `schema_version`, `payload` (JSON), `priority` (0–9), `run_after` (UTC; omitted = now), `max_attempts` (1–5), `timeout`, idempotency key + namespace. No shell/path/URL-style free fields exist in the form (AC-12 UI side).
- **Response (2xx):** the full job snapshot (see 3.3 fields) incl. `status` (`queued` or `scheduled`), `version`, `as_of`; **replay indicator** distinct from first creation (NEED, OQ-12).
- **2xx only after commit** (AC-01): UI shows "accepted" only on 2xx.
- **Errors:** E-07 (409 key mismatch, with the existing job ID in `details` where permitted), E-08 (413 payload), E-09 (422 per-field paths into `payload` too, so the editor can mark schema violations), E-10 (`handler_unknown`, `handler_version_unsupported`, `run_after_out_of_bounds`), E-11, E-12, E-13, 401/403.
- **Needs outside this op:** the list of allowed handlers/versions and their JSON Schemas to build the form and validate in-browser (CCR-02). Whether a job for a handler with no live worker is accepted (it becomes `unschedulable`-reasoned) or rejected (OQ-04).

### 3.2 `GET /jobs`  [§6]

- **Item fields (list):** `id`, `type`, `schema_version`, `status`, `priority`, `created_at`, `available_at`, `started_at?`, `finished_at?`, `attempt_count`, `max_attempts`, `current_worker_id?`, `unschedulable_reason?` (summary: `code`), `failure_class?`, `retried_from_job_id?`, `retried_as_job_id?`, `cancel_requested_at?`, `version`. **No payload/result in list items for any role** (keeps list cheap and masking trivial).
- **Filters (server-side):** `status` (multi-value, the 7 states only), `priority` (min/max or set), `type`, `created_at` window (`from`/`to`), `available_at` window (for delayed jobs), `unschedulable` (bool or reason code), `retried_from_job_id` / has-lineage (optional), exact `id` (for deep link). Dead-letter = `status=failed` (no extra filter state). Failure-class filter optional (OQ-08).
- **Sort keys:** `created_at` (default, desc), `available_at`, `priority`; direction both; deterministic tie-break `id` (§6). Document consistency under concurrent inserts (1.4).
- **Pagination/bounds:** 1.4; max window; `approx_total`+`total_capped` optional.
- **Envelope:** `as_of`.
- **Errors:** E-09/E-10 for bad filters/windows/page size, `cursor_invalid`, E-11, E-12, E-13, 401/403.
- **Masking:** none required in list (`masked_fields` absent), except `requested_by` if shown (OQ-03; UI does not show it in the list in 1.0).
- **RT:** `resource=job` hints (no id needed for list-level "something changed"; optional id).

### 3.3 `GET /jobs/{id}`  [§6]

- **Fields:** everything in 3.2 plus `payload`, `result` (with `result_truncated`/`size_bytes` if bounded), `priority`, `timeout`, `hard_deadline_at`, `lease_generation`, `lease_expires_at` (only while running), `current_attempt_id`, `cancel_requested_at`, `retry_available_at` (for `retry_wait`), full `unschedulable_reason` (code, detail, required handler/version, matching-worker diagnostics), `failure_class`, `created_by`/`requested_by` (maskable), `idempotency_namespace`/`key` (maskable), lineage fields, `version`, `as_of`, `masked_fields`.
- **Masking:** 1.6 (`payload`, `result`, `idempotency_*`, `requested_by`).
- **Errors:** E-04 (404), **E-05 (expired by retention, with `expired_at`)**, 401/403/503/500.
- **Behavioral need:** terminal jobs immutable while retained (§5) — the UI treats any `version` change on a terminal job as a bug and surfaces it in dev builds only.

### 3.4 `GET /jobs/{id}/attempts`  [§6]

- **Per attempt:** `attempt_number` (unique per job), `id`, `worker_id`, `incarnation_id`, `lease_generation`, `started_at`, `finished_at?`, `outcome` (running / succeeded / failed_retryable / failed_permanent / lease_expired / timed_out / canceled — backend defines closed set), `error` {`class` retryable|permanent, `message`, `detail`} (maskable), `lease_expires_at?` (running only), `logs` (bounded; truncation and expiry markers; maskable) — retrieval shape is **CCR-07**.
- **Bounded:** at most `max_attempts` (≤ 5) → no pagination needed; state this so UI does not build one.
- **Masking/expiry:** per-field `masked_fields` and an expiry marker for `logs` (7-day window).
- **Errors:** E-04, E-05, 401/403/503/500.

### 3.5 `GET /jobs/{id}/events`  [§6]

- **Per event:** `version` (per-job contiguous), `occurred_at`, `kind` (closed set; UI must render distinct, icon+text, at least: created, promoted_to_queued, claimed, retry_scheduled, succeeded, failed, cancel_requested, canceled, lease_expired, hard_deadline_reached, **stale_result_rejected** (bounded diagnostic, AC-03), manual_retry_created (with new job ID), unschedulable_set/cleared), `from_status`/`to_status` where applicable, `attempt_number?`, `actor?` (system/worker/principal kind; identity maskable), `detail` (bounded, maskable).
- **Pagination:** cursor; direction ascending by `version` (default) and a way to fetch "after version N" and to load older pages (OQ-09: whether lease renewals are events; UI needs them **not** to flood the stream — renewals should not be individual events).
- **Errors:** E-04, E-05, 401/403/503/500.
- **RT:** `resource=job,id=X` hint → refetch events after last contiguous version plus job snapshot.

### 3.6 `POST /jobs/{id}/cancel`  [§6]

- **Allowed:** per §5 table. UI offers Cancel only for `scheduled`, `queued`, `retry_wait`, `running`; for `running` the semantic is "request cancellation".
- **Response 2xx:** post-state snapshot (status may be `canceled` immediately for pending jobs, or still `running` with `cancel_requested_at` set).
- **Errors:** E-06 `state_conflict` for terminal jobs (the §5 "defined no-op/conflict response") with current status/version in `details`; E-04; E-05; E-11; E-12; 403 (viewer). The UI must be able to distinguish "cancel won" from "success committed first" from the response alone.

### 3.7 `POST /jobs/{id}/retry`  [§6]

- **Allowed:** terminal `failed` jobs only (§5). UI offers it only for `failed`.
- **Request:** idempotent action key (1.7).
- **Response 2xx:** the **new** linked job snapshot (`retried_from_job_id` = source) and the unchanged source ID; replay with same key returns the same new job (signal as in 3.1).
- **Errors:** E-06 (not `failed` / already retried policy), E-10 `retry_not_eligible` with reason, E-04/E-05, E-11, E-12, 403.
- **Side-need:** source job shows `retried_as_job_id`(s) without a second query (3.2/3.3 NEED).

### 3.8 `GET /workers`  [§6]

- **Item fields:** `id`, display `name`, `incarnation_id`, `liveness` (derived from heartbeat: e.g. live/stale/offline — backend defines closed set and thresholds), `lifecycle` (enrollment: pending/enrolled/draining/revoked/disconnected — closed set, **kept separate from liveness**), `draining_since?`, `revoked_at?`, `capabilities` [{`type`, `schema_versions[]`}], `capacity`, `in_flight`, `last_heartbeat_at`, `registered_at`, `protocol_version`, `sdk_version`, `version` (ordering token, 1.7), `masked_fields`.
- **Filters:** `lifecycle`, `liveness`, handler `type` (and version), name prefix.
- **Sort:** `name`, `last_heartbeat_at`, `registered_at`; tie-break `id`.
- **Bounds:** fleet sizes of 1, 10 and 100 must be returnable for topology (AC-23): state max page size ≥ 100 or a bounded "all workers" mode with cap and `has_more`; UI aggregates clusters client-side (OQ-06). If the fleet exceeds the cap, S-TOO-MANY.
- **Envelope:** `as_of`; summary counts by lifecycle/liveness (so the summary strip does not need a second query; may come from overview).
- **Errors:** E-09 (bad filter), `cursor_invalid`, E-11, E-12, E-13, 401/403.

### 3.9 `GET /workers/{id}`  [§6]

- **Fields:** all of 3.8 plus `incarnation_history?` (prior incarnations, bounded, optional), `active_attempts[]` (job_id, attempt_number, started_at, lease_expires_at), `permitted_metadata` (host, build/SDK version, no secrets, maskable), `drain` (requested_at, grace_deadline_at, state), `revocation` (revoked_at, reason class — no credential material), `version`, `as_of`.
- **Events/logs for a worker:** not in §6 → CCR-05.
- **Errors:** E-04, E-05 (worker record removed by retention), 401/403/503/500.

### 3.10 `POST /workers/{id}/drain`  [§6]

- **Request:** none required by UI beyond identity; grace period is the documented policy (UI displays it from config/response, CCR-04/3.9 `drain.grace_deadline_at`).
- **Response 2xx:** post-state worker snapshot with `lifecycle=draining` and `grace_deadline_at`.
- **Errors:** E-06 (already draining / revoked / deregistered) with current lifecycle in `details`, E-04, E-05, E-11, E-12, 403.
- **Truthfulness need:** response/docs must not imply the process was stopped (§4/§5): drain stops new claims only.

### 3.11 `GET /metrics/overview`  [§6]

- **Fields (all with explicit `unknown` vs `0` distinction):** jobs by status (the 7 states); queue depth (queued, eligible); **oldest eligible queued age** (ms) and its job ID; running count; counts of succeeded/failed/canceled in a stated trailing window (window length echoed); `unschedulable_count` with top reasons; workers by lifecycle and liveness; recent lease-expiry count (window echoed); DB/coordinator readiness hint; `as_of`; `freshness` (aggregation lag in ms, so "data as of" is honest); `metrics_available` flag set false rather than returning zeros when the source is down (S-NO-METRICS variant b).
- **Alerts:** CCR-08.
- **Errors:** E-11, E-12, E-13, 401/403.

### 3.12 `GET /metrics/timeseries`  [§6]

- **Request the UI needs to express:** metric name(s) from a closed set (throughput = jobs finalized per step by outcome; queue depth; oldest queued age; attempts started; failures; duration p50/p95; workers online), `from`/`to` (bounded span), `step` (closed set; server enforces max points), aggregation where meaningful.
- **Response:** per series: unit, `step_ms`, ordered `points` [{`t`, `v`}] with **explicit gaps** (`v: null`) distinct from zero; `as_of`; `aggregation_lag_ms`; `retention_floor` (earliest available); `truncated`/`max_points` markers.
- **Provenance:** states whether derived from job events or aggregates (§6 "disclosed freshness") so the freshness notice is honest.
- **Errors:** E-10 `time_window_too_wide`/`step_invalid` (with allowed values in `details`), E-05-like `expired_by_retention` for windows before the floor, E-11, E-12, 401/403.
- **Cardinality:** no per-job or per-worker label explosion; per-handler-type series only if bounded.

### 3.13 `GET /audit`  [§6]

- **Entry fields:** `id`, `occurred_at`, `actor` {kind: user/api_token/worker/system, id/display — maskable}, `action` (closed set incl. job.cancel, job.retry, worker.drain, token.issue/revoke/rotate, worker_credential.issue/revoke, auth failures that are audit-worthy, stale_result_rejected as security/diagnostic), `target` {type, id}, `outcome` (success/denied/rejected), `request_id`/correlation ID (**must equal the `request_id` returned on the original API call**, enabling "find this error in activity"), `detail` (bounded, maskable), `masked_fields`.
- **Filters:** `actor`, `action`, `outcome`, `target_type`, `target_id`, time window (bounded).
- **Sort:** `occurred_at` desc default; tie-break `id`.
- **Access:** whether VIEWER may call it at all, and which fields mask: OQ-03 (route-level 403 for VIEWER is the fallback and the UI handles both).
- **Errors:** E-09, `cursor_invalid`, E-05 for windows older than 90 d (final value per retention ADR), E-11, E-12, 401/403.

### 3.14 `/health/live`, `/health/ready`, `/metrics`

Not consumed by the browser. Needed by UI only indirectly: `unavailable` (E-12) must be emitted by every API operation when `ready` would fail, so the UI shows S-ERR-503 rather than S-ERR-500 (AC-08 console side).

---

## 4. BFF-owned surfaces (not Go REST) — CCR-09

Specified here only so the BFF contract owner has the consumer needs. No Go endpoint is requested for these.

| Need | Detail |
|---|---|
| Session info | Effective `role`, GitHub login/display, `session_expires_at`, CSRF token for mutations, current contract/API version. Returned without any OAuth token. |
| Auth start/callback | PKCE/state flow; failure outcomes mapped to a **closed set**: `auth_error`, `no_entitlement`, `provider_unavailable` (so `/login` renders distinct states). Callback `next` validated same-origin and `/app` prefix. |
| Sign out | Ends session; subsequent WS reconnect rejected (FR-SEC-06). |
| Expiry signalling | Distinguishable `session_expired` response (E-02) for any BFF-proxied API call, and a WS auth close code. |
| Preferences | **None server-side**: timezone/motion override are browser-local in 1.0. If the owner wants cross-device preferences, that is a new CCR. |

---

## 5. `/demo` fixture requirements (no API)

- Source: bundled typed JSON in the web app, **no network**. Same TypeScript types as the generated client where shape-compatible (so shared UI primitives work), delivered through a **separate demo data provider** with no transport. Static dependency-boundary lint: demo module must not import the live transport/BFF client (`E2E-DEMO-27`).
- Required fixture sets: (a) healthy 3-worker cluster with 1,000-job aggregate summary (labelled seeded); (b) scripted failure story: worker killed → lease expiry → retry → stale late result rejected; (c) job in each of the 7 states; (d) an `unschedulable` job with reason (S-NO-WORKER); (e) viewer-masked variants of the same job (uses `masked_fields`, 1.6); (f) metrics series with a deliberate gap and an empty window (S-NO-METRICS); (g) an expired-by-retention example (S-EXPIRED-DATA); (h) 5,000-row list (S-TOO-MANY); (i) example error states for captions only (409, 403, 503).
- All IDs carry a synthetic prefix (`demo-`). All timestamps are fixed (fixture time), never `Date.now()`-derived "live".
- Labelling: persistent non-dismissible "Illustrative demo — seeded fixture data, read-only, not live". No "Live/Connected" badge, no WS, no cookie, no `Authorization`.

## 6. Static/build-time inputs for public routes

| Route | Input | Requirement on others |
|---|---|---|
| `/docs/api` | Published OpenAPI document at the frozen contract SHA | Integrator exposes it as a build artifact path (not an endpoint); page prints the SHA; build fails if missing |
| `/docs/concepts` | Generated state/event table source (§5, FR-ENG-03) | Backend/integrator publishes one machine-readable source |
| `/docs/security` | Access matrix source (`docs/security/permissions.md` per §13) | Integrator/backend |
| `/docs/quickstart`, `/docs/deployment`, `/docs/workers` | Tested scripts/examples under `examples/`, evidence file for the "verified" stamp (AC-26) | Integrator |
| `/changelog` | `CHANGELOG.md` / git tags | Integrator |

## 7. Seed/test data the UI needs for E2E (not product endpoints)

For `E2E-*` to prove real persisted state (AC-15/16/18/23), the integration environment needs deterministic seeds: one job in each of the 7 states; failed job with 3 attempts (retries exhausted); unschedulable job; a viewer principal and an operator principal; 1, 10 and 100 workers; a worker that can be killed/drained; 1,000-job run; a retention-expired job and worker; metrics windows with gaps; a way to force WS drop/reorder/duplicate (1.8.6) and to expire a session. Responsibility: backend provides seed/fault hooks in test builds only; integrator wires CI. These hooks must not exist in production builds.

---

## 8. Contract change requests

Frontend requests; it does **not** design the endpoint, verb, path or schema. Each lists the need and the affected route. Blocking = the route cannot ship real behavior without it.

| ID | Need (capability absent from §6) | Why / routes | Blocking? |
|---|---|---|---|
| **CCR-01** | Management of **direct-API tokens and worker credentials**: list (non-secret metadata only: name, scope, created/expiry/last-used, status), create with **one-time secret in the creation response only**, rotate, revoke, with audit entries (FR-SEC-03/05/11, J-10, J-02). Creation must be safely retryable (idempotency key) and the secret never retrievable again. | `/app/settings/tokens`; `/docs/workers` credential steps; revocation visible on worker detail. §6 lists no token/credential operations at all. | **Yes**: route cannot be built; omitted from nav until resolved |
| **CCR-02** | **Handler catalogue** for the new-job form: allowlisted handler names, schema versions, the JSON Schema (or equivalent validator input) per version, defaults/limits per handler, and whether a live worker currently advertises each version. | `/app/jobs/new` (select, in-browser validation, "no capable worker" warning) | **Yes** for the intended UX; fallback = free-text type/version + server-only validation (degraded, not recommended) |
| **CCR-03** | **Realtime channel definition**: connect path, authentication/ticket mechanism (session-bound, short-lived), origin checks, subscription scopes, message schemas (hint, hello, resync, auth_expiring per 1.8), close codes, heartbeat interval, reconnect/backoff guidance, per-user connection limit. §6 requires WebSockets but specifies none. | All `RT` routes; FR-RT-01, FR-SEC-06, AC-16 | **Yes** for live indicator; fallback = polling-only (UI still functions, shows `Polling`) |
| **CCR-04** | **Read-only environment/config/limits**: environment name, version/contract SHA, effective limits (payload, result, attempts, priority, max future `run_after`, idempotency retention, page/window caps), retention windows, timing parameters (heartbeat, lease, grace) as configured. | `/app/settings`; client pre-checks everywhere; drain grace display | No: fallback shows labelled static documented defaults and omits live values |
| **CCR-05** | **Worker event/log feed**: recent bounded events/logs for a worker (registration, incarnation changes, drain, revoke, lease expiries attributable to it). May be satisfied by an existing op gaining a `worker_id` filter, provided it is bounded, paginated and maskable. | `/app/workers/[id]` | No: section omitted until resolved |
| **CCR-06** | **Cross-job event chronology** for the activity page (job events across jobs, not only per-job and not only privileged audit). May be satisfied by `GET /audit` including job events with a `kind` filter. | `/app/activity` | No: page shows audit only |
| **CCR-07** | **Attempt log retrieval semantics**: confirm logs are inline in `GET /jobs/{id}/attempts` (bounded, with truncation marker and 7-day expiry marker, maskable) or add a way to fetch them. Includes size cap per attempt and over-limit behavior (§5 "explicit"). | `/app/jobs/[id]` | Partial: section degrades to "logs unavailable" |
| **CCR-08** | **Operational alerts** on overview: backend-defined, structured alerts (code, severity, target, since) for unschedulable jobs/no capable worker, worker offline, repeated lease expiries, queue age, DB not ready — so thresholds live in one place (backend) and match the alert/runbook set in §12. Frontend will not invent thresholds. | `/app` "topical alerts"; NFR-OBS-01 | No: fallback shows only raw KPIs and `unschedulable_count`, no alert list |
| **CCR-09** | **BFF contract** items in section 4 (session info, role, CSRF, expiry signalling, auth outcome codes). Owner: Opus/integrator, not a Go change. | `/login`, all `/app*`, `/app/settings/preferences` | **Yes** (needed before M6) |

Field/filter/semantic additions on existing operations (NEED items in section 3) are listed there, not as CCRs. Highest-impact ones for review: worker ordering token (1.7), `retried_as_job_id`, `unschedulable_reason` structure and filter (1.10), `masked_fields` (1.6), replay indicator on `POST /jobs`, `expired_by_retention` distinct from `not_found` (E-05), mutation responses carrying post-state and conflict details (1.7), `as_of` everywhere (1.2).

## 9. Open questions (need an owner answer; frontend defaults noted)

| ID | Question | Frontend default meanwhile | Asks |
|---|---|---|---|
| OQ-01 | May a **new-job draft** (payload JSON, no secrets by policy) persist in `sessionStorage` across session expiry (cleared on sign-out/success)? | Yes, per-tab only; if refused: in-memory only, lost on expiry | Opus security |
| OQ-02 | When Go rejects a BFF service assertion (401 from Go), how does the BFF surface it — as `session_expired` (user re-auth) or as 503/500 (infrastructure)? Must not trap users in login loops. | Treat only BFF-originated 401 as session expiry; Go-originated auth failure as S-ERR-500 with request_id | Opus |
| OQ-03 | May VIEWER read `/audit`? Which fields mask (actor identity, detail, source IP)? Is `idempotency_key` / `requested_by` maskable? | Activity route allowed for VIEWER with detail masked; if denied → route-level S-403, nav hidden | Opus + owner |
| OQ-04 | Does `POST /jobs` accept a job whose handler/version has no live worker (becoming `unschedulable`-reasoned), or reject it? Does it accept an unknown handler name at all? | Accept known-catalogue handlers only; warn when none live | Backend |
| OQ-05 | Fallback polling interval, hidden-tab behavior, per-user request budget, max concurrent WS connections. | 5 s focused, suspended hidden, back off on 429 | Backend |
| OQ-06 | Max page size, max time-window span per endpoint, max workers returnable for topology, max timeseries points. | 100 rows; 24 h default window; 200 workers; 500 points | Backend |
| OQ-07 | Sort-key mutability: does `available_at` (changes on retry/promotion) break cursor stability? Which sort keys are immutable? | Default sort `created_at desc`; `available_at` sort offered only if guaranteed stable | Backend |
| OQ-08 | Is there a `failure_class` (permanent / retries exhausted / timeout / lease expired) on failed jobs, filterable for the dead-letter view? | Show it if present; filter only by `status=failed` otherwise | Backend |
| OQ-09 | Do lease renewals create job events? (Volume.) | Assume not; UI renders only the kinds in 3.5 | Backend |
| OQ-10 | Retention windows final (logs 7 d, history 30 d, audit 90 d are proposals) and status code for expired resources (404 vs 410) — UI needs only the distinct `code`. | Distinct code assumed | Backend ADR |
| OQ-11 | Preferences local-only acceptable for 1.0? | Local-only per device, labelled | Owner |
| OQ-12 | How is an idempotent replay signalled on `POST /jobs` and `POST /jobs/{id}/retry` (distinct status vs body flag)? | Either; UI keys on a boolean field if present, else on a distinct status | Backend |
| OQ-13 | Typeface/asset licensing for the brand font is outside this ticket; flagged because §9 requires usable licenses before design artifacts are final. | System font fallback specified; no font committed | Owner |

## 10. Sign-off checklist for TASK-M1-FE-02 (frontend reviews backend drafts)

Frontend will confirm, per OpenAPI/proto draft, that: every error in 1.3 is a distinct `code`; masked/empty/expired fields are separable (1.6); versions and `as_of` exist where 1.7 needs them; idempotency keys and replay signals exist on all four mutations; pagination guarantees are stated (1.4); dead-letter needs nothing beyond `status=failed` (+ optional `failure_class`); `unschedulable` is a field not a status enum member; the realtime messages carry no data (1.8); every CCR has a disposition (accepted / alternative / rejected with fallback acknowledged).
