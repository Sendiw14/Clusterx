# CLUSTERX — Consumer Requirements (frontend → API contract, M1 input)

- Ticket: TASK-M1-FE-01 · Lane: frontend (`m1/frontend`) · Author model: `claude-sonnet-5-5`, effort high (requested)
- Source contract: `docs/product/MASTER_PROMPT.md` R2 @ `5a5ca99eb44145100044bf9517e55813cb40903c`. §5 states, §6 REST/realtime, §4 trust boundary, §8 routes.
- Protocol step: §1.3 step 2 (frontend states consumer requirements). Backend drafts OpenAPI/proto; TASK-M1-FE-02 signs the drafts against this file.
- Status: **specified; signed off with gaps against `api/openapi.yaml` 1.0.0-draft.3 @ `f82325b` (TASK-M1-FE-02, section 11)**. Sections 1 to 10 are the original FE-01 needs; **section 11 is authoritative where it differs** (real error codes, real shapes, dispositions, gaps). This document lists **needs**, not endpoint designs. Names such as `code` values are placeholders for the backend to choose; the UI will key on whatever stable machine `code` the contract defines, never on `message` text.
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

| ID | Need (capability absent from §6) | Why / routes | Blocking? | Disposition (draft.3) |
|---|---|---|---|---|
| **CCR-01** | Management of **direct-API tokens and worker credentials**: list (non-secret metadata only: name, scope, created/expiry/last-used, status), create with **one-time secret in the creation response only**, rotate, revoke, with audit entries (FR-SEC-03/05/11, J-10, J-02). Creation must be safely retryable (idempotency key) and the secret never retrievable again. | `/app/settings/tokens`; `/docs/workers` credential steps; revocation visible on worker detail. §6 lists no token/credential operations at all. | **Yes**: route cannot be built; omitted from nav until resolved | **SATISFIED** (`/tokens`, `/worker-credentials`, rotate composed by UI); GAP-CRED-01 |
| **CCR-02** | **Handler catalogue** for the new-job form: allowlisted handler names, schema versions, the JSON Schema (or equivalent validator input) per version, defaults/limits per handler, and whether a live worker currently advertises each version. | `/app/jobs/new` (select, in-browser validation, "no capable worker" warning) | **Yes** for the intended UX; fallback = free-text type/version + server-only validation (degraded, not recommended) | **SATISFIED** (`GET /handlers`) |
| **CCR-03** | **Realtime channel definition**: connect path, authentication/ticket mechanism (session-bound, short-lived), origin checks, subscription scopes, message schemas (hint, hello, resync, auth_expiring per 1.8), close codes, heartbeat interval, reconnect/backoff guidance, per-user connection limit. §6 requires WebSockets but specifies none. | All `RT` routes; FR-RT-01, FR-SEC-06, AC-16 | **Yes** for live indicator; fallback = polling-only (UI still functions, shows `Polling`) | **SATISFIED with gaps** (`GET /stream`); GAP-RT-01..03 |
| **CCR-04** | **Read-only environment/config/limits**: environment name, version/contract SHA, effective limits (payload, result, attempts, priority, max future `run_after`, idempotency retention, page/window caps), retention windows, timing parameters (heartbeat, lease, grace) as configured. | `/app/settings`; client pre-checks everywhere; drain grace display | No: fallback shows labelled static documented defaults and omits live values | **SATISFIED with gap** (`GET /config`); GAP-CFG-01 |
| **CCR-05** | **Worker event/log feed**: recent bounded events/logs for a worker (registration, incarnation changes, drain, revoke, lease expiries attributable to it). May be satisfied by an existing op gaining a `worker_id` filter, provided it is bounded, paginated and maskable. | `/app/workers/[id]` | No: section omitted until resolved | **DEFERRED to M6**; interim `GET /audit?subject_type=worker&subject_id=`; GAP-AUDIT-02 |
| **CCR-06** | **Cross-job event chronology** for the activity page (job events across jobs, not only per-job and not only privileged audit). May be satisfied by `GET /audit` including job events with a `kind` filter. | `/app/activity` | No: page shows audit only | **DEFERRED to M6**; interim `/audit` + per-job events; GAP-AUDIT-01 |
| **CCR-07** | **Attempt log retrieval semantics**: confirm logs are inline in `GET /jobs/{id}/attempts` (bounded, with truncation marker and 7-day expiry marker, maskable) or add a way to fetch them. Includes size cap per attempt and over-limit behavior (§5 "explicit"). | `/app/jobs/[id]` | Partial: section degrades to "logs unavailable" | **SATISFIED** (`log_tail`, `log_truncated`, `log_expired` on attempts) |
| **CCR-08** | **Operational alerts** on overview: backend-defined, structured alerts (code, severity, target, since) for unschedulable jobs/no capable worker, worker offline, repeated lease expiries, queue age, DB not ready — so thresholds live in one place (backend) and match the alert/runbook set in §12. Frontend will not invent thresholds. | `/app` "topical alerts"; NFR-OBS-01 | No: fallback shows only raw KPIs and `unschedulable_count`, no alert list | **DEFERRED to M8**; raw signals only; GAP-OVR-01 |
| **CCR-09** | **BFF contract** items in section 4 (session info, role, CSRF, expiry signalling, auth outcome codes). Owner: Opus/integrator, not a Go change. | `/login`, all `/app*`, `/app/settings/preferences` | **Yes** (needed before M6) | **SATISFIED with gaps** (ADR-0006, status Proposed); GAP-BFF-01/02, GAP-SESS-01 |

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

---

## 11. Consumer sign-off against contract 1.0.0-draft.3 (TASK-M1-FE-02)

Inputs (read-only, from the shared repository): `main:docs/coordination/CCR-M1-decisions.md` @ `12c004e`; backend `f82325b`: `api/openapi.yaml` (1.0.0-draft.3), `docs/architecture/state-machine.md`, `docs/architecture/api-contracts.md`, `docs/security/permissions.md`, `docs/architecture/adrs/0006-bff-session-contract.md`, `HANDOFF-M1-BE-01..03`. Nothing was run: this is a document review, not an integration test (**NOT RUN**: TS client generation, any live call).

Legend: **SATISFIED** (cites openapi path/schema), **DEFERRED** (per decision doc, interim noted), **GAP** (exact missing field/code; fallback stated). Operation names are openapi `operationId`s.

### 11.1 Disposition of CCR-01..CCR-10

| CCR | Disposition | Citation / detail |
|---|---|---|
| CCR-01 tokens + worker credentials | **SATISFIED**, one gap | `listApiTokens`, `createApiToken` (201 `ApiTokenCreated.secret`, `Cache-Control: no-store`), `revokeApiToken` (200, idempotent), and the same trio for `/worker-credentials` (`WorkerCredential.bound_worker_ids`). Session-only: viewers and API tokens get 403. Rotation = create + revoke, composed by the UI. **GAP-CRED-01:** no `Idempotency-Key` on either create POST. |
| CCR-02 handler catalogue | **SATISFIED** | `listHandlers` / schema `Handler`: `name`, `versions[].version`, `payload_schema` (JSON Schema 2020-12), `result_schema`, `default_timeout_seconds`, `capable_workers`, `live_capable_workers`. UI needs a 2020-12 validator in the client. |
| CCR-03 realtime | **SATISFIED with gaps** | `openStream`, schema `StreamMessage` (`hello`, `hint`, `resync`, `auth_expiring`), close codes in the operation description. See 11.5. GAP-RT-01, GAP-RT-02, GAP-RT-03. |
| CCR-04 config | **SATISFIED with gap** | `getConfig` / `Config` (`limits`, `retention`, `timing`, `api_version`). GAP-CFG-01. |
| CCR-05 worker events | **DEFERRED to M6** | Interim `listAuditEvents` with `subject_type=worker`, `subject_id`. Viewers only get `privileged` rows (permissions matrix). GAP-AUDIT-02. |
| CCR-06 cross-job chronology | **DEFERRED to M6** | Interim `/audit` plus per-job `listJobEvents`. Activity page shows audit only. GAP-AUDIT-01 for filters. |
| CCR-07 attempt logs | **SATISFIED** | Schema `Attempt`: `log_tail` (≤ 16 KiB, masked or expired = null), `log_truncated`, `log_expired`, `masked_fields`, `lease_expires_at`. Masked / empty / expired are separable (masked via `masked_fields`, expired via `log_expired`). |
| CCR-08 alerts | **DEFERRED to M8** | UI renders raw signals from `MetricsOverview` only and invents no thresholds. GAP-OVR-01 (window labelling). |
| CCR-09 BFF contract | **SATISFIED with gaps** | ADR-0006 (status Proposed pending RV-01). See 11.4. GAP-BFF-01, GAP-BFF-02, GAP-SESS-01. |
| CCR-10 BFF session lifecycle | **SATISFIED** | `createSession` (`POST /api/v1/sessions`, 201 `SessionCreated`; 403 `details.reason` `not_allowlisted` / `user_disabled` / `credential_not_allowed`), `deleteCurrentSession` (204), `getMe` (`Me`). BFF-only. |

### 11.2 Disposition of NEED / field-addition items

| Need (this document) | Disposition | Citation / what to render |
|---|---|---|
| `masked_fields` on restricted objects (1.6) | **SATISFIED** | `Job.masked_fields` (`payload`, `result`, `error_message`), `Attempt.masked_fields` (`error_message`, `log_tail`), `Worker.masked_fields` (`hostname`), `AuditEvent.masked_fields` (`details`). Masked values are `null`. **Job events are not masked** (`JobEvent.data` carries no payload, permissions matrix), narrower than 1.6 assumed. No `idempotency_key`/`requested_by` fields exist (`created_by` is a principal ref, visible to viewers): those rows of 1.6 are dropped. |
| `retried_as_job_id` (3.2/3.3) | **SATISFIED (detail only)** | `Job.retried_as_job_id`; list items carry only `retried_from_job_id` (`JobSummary`). Also in 409 `retry_already_exists` `details.retried_as_job_id`. |
| `unschedulable_reason` field + filter (1.10) | **SATISFIED** | `JobSummary.unschedulable_reason` (`no_capable_worker`, `capable_workers_offline`, `capable_workers_draining`), `unschedulable_since`; filter `unschedulable_reason[]` on `listJobs`. No boolean "any" filter: pass all three. Required handler/version and worker diagnostics come from the job's `handler_name/version` plus `/handlers` counts and `/workers?handler_name=`. |
| Replay signal (OQ-12) | **SATISFIED** | `createJob`: 201 first create, 200 + header `Idempotent-Replayed: true`. `retryJob`: 201 new child, 200 replay (status only; header not declared on retry). BFF must pass status and header through (GAP-BFF-02). |
| Expired distinct from not found (E-05) | **SATISFIED for jobs** | HTTP 410 `code=expired`, `details {terminal_at, expired_at, retention_days}` on `getJob`, `listJobAttempts`, `listJobEvents`, `cancelJob`, `retryJob`. Workers have none (404 `worker_not_found`). Audit/metrics windows: GAP-RET-01. |
| Mutation responses carry post-state; 409 details carry `{status, version}` (1.7) | **SATISFIED** | `cancelJob` 200/202 `Job`; `retryJob` `Job`; `drainWorker` 202 `Worker`; 409 examples carry `details {status, version}` (`job_terminal`, `retry_not_allowed`, `retry_already_exists`, `worker_deregistered`). `idempotency_conflict` carries `details.job_id` instead. |
| Worker ordering token (1.7) | **SATISFIED** | `Worker.version` (bumped on status/incarnation change, **not on heartbeat**). Sort `(last_seen_at DESC, id DESC)` is mutable; clients de-duplicate by id (stated in the operation). |
| `as_of` everywhere (1.2) | **SATISFIED for collections**, gap for single resources | Present on `listJobs`, `listWorkers`, `listAuditEvents`, token/credential lists, `listHandlers`, overview, timeseries. `Job`, `Worker`, `WorkerDetail` have no `as_of` (they have `version`, `updated_at`/`last_seen_at`). GAP-TIME-01. |
| `failure_class` (OQ-08, 1.10) | **SATISFIED** | `JobSummary.failure_class` ∈ `retryable_exhausted`, `permanent`, `timeout`, `lease_expired`, `canceled_by_operator`; non-null exactly on failed/canceled. Display only: **no filter parameter** (GAP-SORT-01). |
| Version on every state-relevant job change (1.7) | **SATISFIED** | `Job.version` equals the latest event version (`getJob` 200 description; `ETag W/"<version>"`). Lease renewals bump nothing (state-machine E4). Reconciliation rule from 1.7 stands. |
| Events: contiguous `version`, resume by version (1.7, 3.5) | **SATISFIED** | `listJobEvents?after_version&limit≤200`, `job_version`, `has_more`, ascending only. Event kinds: `admitted, promoted, claimed, succeeded, failed_retryable, failed_permanent, attempts_exhausted, lease_expired, deadline_exceeded, cancel_requested, canceled, retry_created`. There is **no** `stale_result_rejected`, `unschedulable_*` or `manual_retry_created` job event: stale-result diagnostics are `/audit` rows (`diagnostic`, operator only); `retry_created` is on the **child**. |
| Attempts bounded, ≤ 5, unpaginated (3.4) | **SATISFIED** | `listJobAttempts` `maxItems: 5`, `job_version`. `Attempt.state` ∈ `active, succeeded, failed, lease_expired, deadline_exceeded, canceled`; `error_class` retryable/permanent. |
| Pagination envelope (1.4) | **SATISFIED** | `{items, next_cursor (null at end), as_of}`; `limit` 1-200, default 50; 400 `invalid_cursor` (also when filters changed). No total count: the UI shows "more results exist" when `next_cursor` is non-null after its own DOM cap (500 rows). Keyset snapshot-less: rows inserted mid-traversal may be missed (stated). |
| Filters: jobs (3.2) | **SATISFIED partly** | Present: `status[]`, `handler_name`, `priority_min/max`, `created_after/before` (≤ 31 d, default 7 d), `unschedulable_reason[]`, `retried_from_job_id`. **Missing:** `available_after/before`, `failure_class`, handler version, exact `id` (not needed: command search uses `getJob`). GAP-SORT-01. |
| Sort keys (3.2) | **GAP** | Only fixed `(created_at DESC, id DESC)`; no `sort` parameter on jobs or workers, although OQ-07 was answered "available_at is a stable sort key". GAP-SORT-01. |
| Workers (3.8/3.9) | **SATISFIED mapped** | `status` (active/draining/deregistered) is lifecycle; `live` boolean is liveness; `credential_revoked` boolean overlay; `capabilities[{name,version}]`; `active_attempts` count; `drain_requested_at`/`drain_deadline_at`; `WorkerDetail.active[]`. No `revoked_at` (GAP-WRK-01), no name filter, no "offline" tier beyond `live=false`. Filters: `status[]`, `liveness` (live/stale), `handler_name`. |
| Overview (3.11) | **SATISFIED, thinner than asked** | `jobs_by_status` (all 7), `workers {live, stale, draining}`, `oldest_queued_age_seconds`, `unschedulable_jobs`, `as_of`. Terminal counts are "within retention window" (no named window: GAP-OVR-01). No `metrics_available` flag: source down = 503 `unavailable`. No oldest-job ID, no top reasons. |
| Timeseries (3.12) | **SATISFIED partly** | `metric` ∈ `jobs_admitted, jobs_succeeded, jobs_failed, jobs_canceled, attempts_started, lease_expirations, queue_age_p95_seconds, run_duration_p95_seconds`; one metric per request; `step` 1m/5m/1h; `to - from ≤ 7 d` and within retention; ≤ 2016 points; points `{t, v|null}`; `as_of`; "buckets after as_of are absent, not zero". **Missing:** queue-depth series, duration p50, workers-online series (GAP-OBS-01); null-vs-0 semantics (GAP-OBS-02); no `unit`/`retention_floor` (UI uses a static unit map and `/config`). |
| Audit (3.13) | **SATISFIED partly** | Fields `id, category, action, outcome (allowed/denied/rejected/error), actor, subject_type, subject_id, request_id, occurred_at, details (masked), masked_fields`. Filters only `category`, `subject_type`, `subject_id`, `from`, `to`. GAP-AUDIT-01/02. |
| Idempotency keys on mutations (1.7) | **SATISFIED for jobs**, gap for credentials | `Idempotency-Key` header (1-128 chars `^[A-Za-z0-9_.:-]+$`, 24 h) on `createJob` and `retryJob`. Cancel and drain idempotent (200/202 repeat). Credential creates: GAP-CRED-01. |
| 429 / 503 headers (1.9) | **SATISFIED** | `Retry-After` header and `details.retry_after_seconds` (429); `Retry-After` on 503; `X-Request-Id` on all responses (`headers.RequestId`). |
| `Cache-Control: no-store` for reads (1.9) | **NOT STATED** | Only declared on secret-bearing 201s. Frontend BFF will set `no-store` on all authenticated proxy responses itself (BFF-owned, no contract change). |

### 11.3 Error model as defined by the contract (replaces the placeholder names in 1.3)

Envelope `Error {code, message, details?, request_id}` with `code` enum: `bad_request, invalid_cursor, unauthenticated, forbidden, job_not_found, worker_not_found, job_terminal, idempotency_conflict, retry_not_allowed, retry_already_exists, worker_deregistered, expired, token_not_found, worker_credential_not_found, payload_too_large, validation_failed, rate_limited, unavailable, internal`. The UI keys on `code` (and `details.reason` for 401/403), never on `message`.

| 1.3 row | Real contract | UI state | Notes / gap |
|---|---|---|---|
| E-01 | 400 `bad_request` | S-400 | as planned |
| E-01b | 400 `invalid_cursor` | restart list at page one + notice | new |
| E-02 | 401 `unauthenticated` + `details.reason` | see 11.4 | reason set below |
| E-03 | 403 `forbidden`, `details {required, role}` | S-403 / S-403-ACTION | login 403 reasons in 11.4 |
| E-04 | 404 `job_not_found` / `worker_not_found` / `token_not_found` / `worker_credential_not_found` | S-404 | "same response for nonexistent and inaccessible IDs" |
| E-05 | **410** `expired`, `details {terminal_at, expired_at, retention_days}` | S-EXPIRED-DATA | jobs only |
| E-06 | 409 `job_terminal`, `retry_not_allowed`, `retry_already_exists`, `worker_deregistered` | S-409 | all carry `details {status, version}`; `retry_already_exists` adds `retried_as_job_id` |
| E-07 | 409 `idempotency_conflict`, `details.job_id` | S-409 (mismatch variant) | as planned |
| E-08 | 413 `payload_too_large`, `details {limit_bytes, field}` | S-413 | no `received_bytes` (UI computes its own size) |
| E-09 | 422 `validation_failed`, `details.fields[{field, rule, message}]` | S-422 | **GAP-ERR-01** (rule vocabulary), **GAP-ERR-02** (payload path format) |
| E-10 | folded into `validation_failed` (`handler_name` rule `registered`; window and `run_after` bounds as fields) | S-422 with field | no distinct `time_window_too_wide` / `run_after_out_of_bounds` codes; UI keys on `field` |
| E-11 | 429 `rate_limited`, `Retry-After`, `details.retry_after_seconds` | S-429 | as planned |
| E-12 | 503 `unavailable`, `Retry-After`; after a failed COMMIT the outcome is unknown, retry with the same key | S-ERR-503 | matches the planned UX |
| E-13 | 500 `internal` | S-ERR-500 | `internal` is in the enum but no response is declared for 500 in the operations (minor) |
| BFF-owned | 502 `upstream_auth`, 403 `csrf_failed`, 401 `session_expired` | S-ERR-502, S-CSRF, S-SESSION-EXPIRED | not in the Go enum (BFF-owned); GAP-BFF-02 asks that BFF errors reuse the same envelope |

### 11.4 Authentication outcomes, `/bff/session`, login states (ADR-0006)

**401 `details.reason` to UI/BFF behavior.** Confirmed as specified (ADR-0006 section 2; openapi `Unauthenticated`), with one refinement:

| `details.reason` | Who sees it | Frontend/BFF behavior |
|---|---|---|
| `session_expired`, `session_revoked` | BFF proxy | Destroy BFF session, clear cookie; browser receives 401 `session_expired`; UI shows S-SESSION-EXPIRED (mid-use) or `/login?reason=session_expired`. |
| `user_disabled` (on an existing session) | BFF proxy | Destroy session. **Refinement:** send the user to `/login?error=user_disabled` (the no-entitlement panel) rather than a plain re-login prompt, because signing in again would return 403 `user_disabled` (a loop in effect). |
| `assertion_expired` | BFF fault | Retry **once** with a fresh assertion (new `jti`); if it repeats, 502 `upstream_auth`. |
| `assertion_invalid` | BFF fault | No retry, **session kept**, no login redirect; 502 `upstream_auth` with `request_id` (S-ERR-502). Never a login loop. |
| `token_invalid`, `token_revoked`, `token_expired` | direct API clients | Cannot occur on BFF traffic; if seen, treated as S-ERR-502. |
| (only `assertion_*` on `POST /sessions`) | BFF login | Same handling; no session exists yet. |

**403 on login (`POST /sessions`)** — **confirmed:** `details.reason` `not_allowlisted` and `user_disabled` both render the `/login` **no-entitlement** panel (not a generic error), with distinct copy ("not approved" vs "disabled"), no retry loop, no cookie set; `credential_not_allowed` is a BFF bug and renders as 502 `upstream_auth`. A 403 `forbidden` on any other call is passed through; UI refreshes `/bff/session` once (role may have changed, ADR-0006 section 2).

**BFF-owned codes (confirmed):** `upstream_auth` (HTTP 502) and `csrf_failed` (HTTP 403). `csrf_failed` is rejected before reaching Go, so one automatic retry after refreshing `csrf_token` is safe even for token creation. Distinguish from Go 403 by `code`.

**`/bff/session` shape (confirmed):** `GET /bff/session` returns `{ user: { id, login }, role: "operator" | "viewer", expires_at, idle_expires_at, csrf_token }`; 401 means signed out. (Go `GET /me` returns `{user {id, github_login}, role, session {id, created_at, expires_at, idle_timeout_seconds}}`; the BFF maps `github_login` to `login`.) Mutating BFF calls send `X-CSRF-Token` and rely on the exact-`Origin` check. `POST /bff/logout` ends the session. Cookie `__Host-clusterx_session` is never readable by page JS.

**Login outcome query values.** ADR-0006 does not define them (GAP-BFF-01). Frontend implements the BFF, so it proposes: `/login?error=` one of `auth_error`, `no_entitlement`, `user_disabled`, `provider_unavailable`, `upstream_auth`; `/login?reason=` one of `signed_out`, `session_expired`; anything else renders nothing.

### 11.5 Realtime (`GET /api/v1/stream`, via `/bff/stream`)

Actual messages: `hello {server_time, session_expires_at}`; `hint {entity: job|worker|metrics, id|null, version}` (version 0 for metrics); `resync {reason: buffer_overflow|server_restart|gap_detected}`; `auth_expiring {expires_at}` 60 s before the absolute expiry; client may send `{"type":"ping"}`.

Differences from 1.8 and the UI consequence:

- No `seq`, `emitted_at`, `audit` entity, or heartbeat interval. UI relies on `version` for job/worker and on snapshots; gap detection is by `resync`/reconnect, not a local `seq` check.
- No pong frame is declared, so the UI cannot detect a half-open socket except by close 4408 (**GAP-RT-01**). Fallback: treat silence beyond a fixed UI timeout as `Reconnecting` and refetch.
- Worker hints fire on `workers.version` changes only (status/incarnation), not on heartbeat; **liveness decay emits nothing** (**GAP-RT-02**). Fallback: timed refetch of `/workers`.
- Close codes mapped: 4401 S-SESSION-EXPIRED (no reconnect loop), 4403 S-403, 4408 reconnect with backoff, 4429 (max 5 streams per session) stop reconnecting, fall back to `Polling`, 1008 origin rejected (configuration error), 1001/1012 reconnect with backoff, 1000 normal. The console opens **one** socket per tab; with more than 5 tabs the extras fall back to polling.
- Hints contain only ids and versions, as required (no masked data over the socket).
- Test hooks to force drop/reorder/duplicate and session expiry are not part of the contract (**GAP-RT-03**): backend/integrator must provide test-build-only seams (section 7).

### 11.6 Open questions OQ-01..OQ-13

| OQ | Status | Answer |
|---|---|---|
| OQ-01 | **Closed** | ADR-0006 section 6: per-tab `sessionStorage` draft acceptable; never `localStorage`; cleared on submit success and logout. |
| OQ-02 | **Closed** | ADR-0006 section 2 (mapping in 11.4). |
| OQ-03 | **Open (owner)** | Viewer reads `/audit` privileged rows with `details` masked, actor as principal ref: proposal, needs owner nod. UI supports both outcomes. |
| OQ-04 | **Closed** | Registered handler with no live worker accepted (visible with `unschedulable_reason`); unknown handler = 422. |
| OQ-05 | **Deferred** | Budgets freeze after baseline (NFR-PERF-02); UI honors `Retry-After`; stream max 5 per session. |
| OQ-06 | **Closed by contract** | Page ≤ 200; jobs window ≤ 31 d (default 7 d); timeseries ≤ 7 d, 2016 points; values also in `/config`. Max workers = unpaginated by cursor following (UI cap). |
| OQ-07 | **Answered, not reflected** | "available_at stable sort key" but no `sort` parameter exists: GAP-SORT-01. |
| OQ-08 | **Closed (display only)** | `failure_class` present; no filter. |
| OQ-09 | **Closed** | Renewals are not events; `lease_expires_at` on attempt/job. |
| OQ-10 | **Closed** | Retention in `/config.retention` (logs 7 d, terminal jobs 30 d, audit 90 d, tombstones 90 d); 410 for jobs. |
| OQ-11 | **Proposed** | Preferences browser-local; no endpoint. Needs owner nod (low risk). |
| OQ-12 | **Closed** | 200 + `Idempotent-Replayed: true`. |
| OQ-13 | **Open (owner)** | Typeface licensing, M2. |

### 11.7 Confirmations the backend asked for (HANDOFF-M1-BE-01/02/03)

| Question | Frontend answer |
|---|---|
| Masking shape `null` + `masked_fields[]` | **Accepted.** |
| Jobs list window 31 d max / 7 d default | **Accepted**, provided the UI always states the active window (it does) and a sort/`available_at` filter is added for delayed jobs (GAP-SORT-01). |
| Reconcile via `version` + `events?after_version` | **Accepted** (1.7 rule). |
| Lineage field `retry_child_job_id` | The contract names it **`retried_as_job_id`** (draft.3): accepted; update state-machine/handoff text (GAP-DOC-01). |
| Three unschedulable reasons enough for copy | **Accepted**; copy exists for each. |
| 401 `details.reason` mapping; `upstream_auth` 502, `csrf_failed` 403; `/bff/session` shape; `/bff/stream` | **Confirmed** (11.4, 11.5). |
| 403 `not_allowlisted` / `user_disabled` render the no-entitlement state | **Confirmed**, with distinct copy. |

### 11.8 GAP list (ranked by the route it blocks; none blocks a route outright)

"Blocks" means the route works only in the degraded way stated under Fallback. Ranked highest impact first.

| Rank | GAP | Blocks | Exact missing field / code | Fallback now in the routes |
|---|---|---|---|---|
| 1 | **GAP-SESS-01** idle timeout enforcement | all `/app*` (security) | Go has no `sessions.last_activity_at`; ADR-0006 and draft.3 leave idle expiry to the BFF, but the BFF has no stated store for last-activity (a restart would reset or drop timers) | UI shows `idle_expires_at` from `/bff/session` and `expires_at`; idle logout is best-effort until the DDL lands |
| 2 | **GAP-BFF-01** login outcome set | `/login`, `/app/settings/preferences` | No closed set of `?error=` / `?reason=` values or BFF auth-failure codes in ADR-0006 | Frontend proposal in 11.4; needs ADR-0006 amendment and RV-01 review |
| 2 | **GAP-BFF-02** BFF proxy envelope | every route using the API | ADR does not say the BFF returns Go error bodies unchanged, passes status codes (201/200/202/410), `Idempotent-Replayed`, `Retry-After`, `X-Request-Id`, and uses the same `{code,message,details?,request_id}` for BFF-owned errors (`upstream_auth`, `csrf_failed`, `session_expired`) | Frontend implements the BFF that way; record it in ADR-0006 |
| 3 | **GAP-ERR-01** validation rule vocabulary | `/app/jobs/new`, `/app/jobs`, `/app/observability` | `details.fields[].rule` values are not enumerated (only `range`, `registered` appear in examples); no codes for window-too-wide, `run_after` bounds, `step` | UI keys on `field` and shows server `message` |
| 3 | **GAP-ERR-02** payload violation path | `/app/jobs/new` | No statement of the `field` format for schema violations inside `payload` (JSON Pointer vs dotted) | Violations listed in the summary; editor flagged as a whole; client-side schema validation shows precise errors |
| 4 | **GAP-SORT-01** list sort and filters | `/app/jobs` (J-05 "find delayed"), `/app/workers` | `sort` parameter on `listJobs`/`listWorkers`; jobs filters `available_after/before`, `failure_class`, handler version | One fixed sort; no sort controls; workers sorted client-side after fetching all pages; delayed jobs found via `status=scheduled`/`retry_wait` filter only |
| 5 | **GAP-RT-01** liveness of the socket | all `RT` routes | No pong or server heartbeat frame/interval (`hello` has no interval) | UI-side silence timeout then refetch |
| 5 | **GAP-RT-02** missing hint classes | `/app/workers`, `/app/activity`, `/app` | No hint for worker liveness decay, no `audit` entity, no `seq` | Timed refetch of workers and audit page one |
| 5 | **GAP-RT-03** E2E fault seams | AC-16 tests on all `RT` routes | No contract-level (or test-build) way to drop/reorder/duplicate frames or expire a session on demand | Integrator/backend provide test-build seams (section 7) |
| 6 | **GAP-AUDIT-01** audit filters | `/app/activity` | `actor`, `action`, `outcome`, `request_id` filters | Only category/subject/window server-side; optional page-local text find |
| 6 | **GAP-AUDIT-02** audit coverage and vocabulary | `/app/activity`, `/app/workers/[id]`, `/app/jobs/[id]` | `action` values are free strings (no enum); not stated which worker lifecycle events are audited; not stated that `AuditEvent.request_id` equals the original API call's `request_id` | Render action strings verbatim; do not promise "find the failed call here" until confirmed |
| 7 | **GAP-OBS-01** missing series | `/app/observability`, `/app` | Queue-depth time series, duration p50, workers-online series | Chart only the 8 real metrics; show current queue depth and oldest age from overview |
| 7 | **GAP-OBS-02** null vs 0 | `/app/observability` | Whether empty buckets are `0` (counts) or `null`, and `null` for percentiles | Render `null` as a gap, `0` as zero; wording per S-NO-METRICS |
| 7 | **GAP-RET-01** pre-retention windows | `/app/observability`, `/app/activity` | Behavior when `from`/`to` precede retention: 422 vs empty | UI clamps using `/config.retention` and tells the user |
| 8 | **GAP-CRED-01** idempotent credential create | `/app/settings/tokens` | `Idempotency-Key` (or equivalent) on `createApiToken` / `createWorkerCredential` | Client-side double-submit block; "outcome unknown" copy; revoke and recreate if unsure |
| 9 | **GAP-WRK-01** revocation time | `/app/workers/[id]` | `revoked_at` (only boolean `credential_revoked`) | Badge without a time |
| 10 | **GAP-CFG-01** environment info | `/app/settings` | `environment`/name, build version or commit, contract SHA; `timeseries` window max (7 d is only in the parameter description) | Show `api_version` only; use documented 7 d |
| 11 | **GAP-TIME-01** single-resource `as_of` | `/app/jobs/[id]`, `/app/workers/[id]` | `as_of` (or reliance on HTTP `Date`) on `Job`, `Worker` | UI uses the passed-through `Date` header for lease countdown/stale labelling and `hello.server_time`; BFF must pass `Date` through |
| 12 | **GAP-OVR-01** overview windows | `/app` | Terminal counts are "within retention window" without a named window; no per-window counts | Label as "last N days" from `/config.retention.terminal_jobs_days` |
| 13 | **GAP-DOC-01** contract text mismatches | none (clarity) | `state-machine.md` shows `unschedulable: {reason, since}` but openapi uses flat `unschedulable_reason` + `unschedulable_since`; `api-contracts.md` shows hint `{type:"job.changed", job_id, version}` but openapi is `{type:"hint", entity, id, version}`; BE-01 says `retry_child_job_id` but openapi says `retried_as_job_id`; `JobNotFound` description says "or deleted by retention" while 410 now covers that; `Config.api_version` example is `draft.2`; `Error` enum includes `internal` without a declared 500 response | UI follows openapi; backend to reconcile text |

### 11.9 Parts of sections 1-10 superseded by section 11

1.3 error table (names replaced by 11.3); 1.4 `approx_total`, `cursor_invalid`, `time_window_too_wide` (now `invalid_cursor`, no total, 422 field errors); 1.6 maskable-field table (narrower, 11.2); 1.8 realtime shape (11.5); 1.10 `unschedulable_reason` shape (flat fields, enum of three); 3.2 filters/sort; 3.8 worker liveness/lifecycle names; 3.11/3.12 field lists; 3.13 audit filters; section 8 CCR table (dispositions added); OQ table (11.6).
