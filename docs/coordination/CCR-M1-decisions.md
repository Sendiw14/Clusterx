# M1 contract change requests — integrator decisions

Inputs: `HANDOFF-M1-FE-01` (CCR-01..09, OQ-01..13), `HANDOFF-M1-BE-01` (4 contract gaps).
Rule (§1.3): §6 lists the REST baseline, but §7/§8 require token management, handler selection and a live channel. Endpoints below are refinements needed to satisfy existing master requirements — **not new scope**. Backend implements them in `api/openapi.yaml` under TASK-M1-BE-02; integrator freezes at M1 exit.

| CCR | Decision | Contract |
|---|---|---|
| CCR-01 tokens + worker credentials | **ACCEPT** (§7, §8 `/app/settings/tokens`) | `GET/POST /api/v1/tokens`, `POST /api/v1/tokens/{id}/revoke`; `GET/POST /api/v1/worker-credentials`, `POST /api/v1/worker-credentials/{id}/revoke`. Rotate = create new + revoke old (UI composes; no rotate endpoint). Secret returned once in 201 body only. Operator only; viewer 403. Audited. |
| CCR-02 handler catalogue | **ACCEPT** (§8 `/app/jobs/new` allowlisted select) | `GET /api/v1/handlers`: name, versions[], JSON Schema per version, `capable_workers` count. |
| CCR-03 realtime channel | **ACCEPT** (§6, FR-RT-01) | `GET /api/v1/stream` WebSocket upgrade. Messages are hints `{type, entity, id, version}`; `hello`, `resync`, `auth_expiring`. Auth/origin checked on upgrade; close codes documented. Snapshot stays authoritative. |
| CCR-04 config/limits | **ACCEPT** (§8 `/app/settings`) | `GET /api/v1/config`: read-only limits (payload/result bytes, attempts range, page caps, retention windows). No secrets. |
| CCR-05 worker events | **DEFER to M6**; interim `GET /audit?subject_type=worker&subject_id=` | — |
| CCR-06 cross-job chronology | **DEFER to M6**; interim `/audit` + per-job events | — |
| CCR-07 attempt logs | **ACCEPT as fields** | `GET /jobs/{id}/attempts` items carry `log_tail`, `log_truncated`, `log_expired`. |
| CCR-08 overview alerts | **DEFER to M8** (thresholds need pilot, OD-06). UI shows raw signals only, no invented alerts. | — |
| CCR-09 BFF contract | **ACCEPT** as ADR-0006 (backend-owned; Sonnet implements BFF) | Assertion claims, audience, TTL, CSRF/origin, auth outcome codes, session-expiry surfacing. |

Field additions — **ACCEPT**: `masked_fields`; `retried_as_job_id`; `unschedulable_reason` field + filter; replay signal = HTTP 200 + `Idempotent-Replayed: true` (first create = 201); expired-by-retention = **410 Gone** with `code=expired` (distinct from 404); mutation responses and 409 `details` carry post-state `{status, version}`; stable worker ordering `(last_seen_at DESC, id)`.

Open questions answered by integrator:

- OQ-04: `POST /jobs` accepts a registered handler with no live worker; job is visible with `unschedulable_reason`.
- OQ-07: `available_at` is a stable sort key with `id` tie-break.
- OQ-08: yes, `failure_class` ∈ `retryable_exhausted | permanent | timeout | lease_expired | canceled_by_operator` on failed/canceled jobs.
- OQ-09: lease renewals do **not** create job events (bounded log); visible as `lease_expires_at` on the attempt.
- OQ-10: retention windows proposed in ADR-0004; expired = 410.
- OQ-12: see replay signal above.

Owner / later milestone: OQ-01, OQ-02 (→ ADR-0006), OQ-03 (viewer reads `/audit` with actor/detail masking — proposed, needs owner nod), OQ-05/06 (budgets freeze after baseline, NFR-PERF-02), OQ-11 (local preferences for 1.0 — proposed yes), OQ-13 (typeface licensing — owner, M2).
