# CLUSTERX — API contracts index (M1 draft)

Status: **specified**, not frozen. Integrator freezes at M1 exit (§1.3 step 3).

| Contract | Path | Version | Validation run in M1 |
|---|---|---|---|
| REST | `api/openapi.yaml` | OpenAPI 3.1.1, API `1.0.0-draft.4` | PyYAML parse + `$ref` check + stray-key scan; `npx @redocly/cli@2.79.0 lint` → valid, 0 errors, 5 intended warnings (TASK-M1-BE-04) |
| gRPC | `packages/proto/clusterx/worker/v1/worker.proto` | package `v1`, protocol_version 1 | `protoc --descriptor_set_out=/dev/null` |
| SQL | `apps/control-plane/migrations/0001_init.sql` | 0001 | `psql -v ON_ERROR_STOP=1 --single-transaction` on PG 18 |
| State table | `docs/architecture/state-machine.md` | M1 | PoC tests for AC-02/03/06 |

## Cross-cutting rules
- Error body: `{code, message, details?, request_id}`; codes enumerated in `Error.code` and state-machine.md.
- Request id: `X-Request-Id` in/out; included in logs, audit rows and error bodies.
- Pagination: opaque base64url cursor encoding the last row's sort key **and** a hash of the filters;
  stable keyset sort with `id` tie-break; `limit` 1–200; jobs list time window ≤ 31 days.
- Reconciliation (FR-RT-01): `GET /jobs/{id}` returns `version`; `GET /jobs/{id}/events?after_version=v`
  returns the gap. WebSocket hints `{type:"hint", entity, id, version}` (OpenAPI `StreamMessage`) are advisory only.
- Masking: viewer sees `null` + `masked_fields` (see docs/security/permissions.md).
- Idempotency: `Idempotency-Key` on `POST /jobs` and `POST /jobs/{id}/retry` (ADR-0004).

## Contract items resolved by integrator (CCR-M1-decisions.md, implemented in draft.2)
CCR-01 tokens + worker credentials · CCR-02 `GET /handlers` · CCR-03 `GET /stream` · CCR-04 `GET /config` ·
CCR-07 attempt log flags · CCR-09 → ADR-0006. Deferred: CCR-05/06 (M6; interim `/audit?subject_type=&subject_id=`), CCR-08 (M8).

CCR-10 (Round 2, ACCEPTED) → draft.3: `POST /api/v1/sessions`, `DELETE /api/v1/sessions/current`, `GET /api/v1/me`.

draft.4 (TASK-M1-BE-04): RV-01 fixes and FE-02 gaps:
- SESS-01: Go enforces idle timeout via `sessions.last_activity_at`.
- SORT-01: `sort`, `available_after/before`, `failure_class`, `handler_version`.
- ERR-01/02: `ValidationDetails` with a `rule` enum and JSON-Pointer `field`.
- CRED-01: `Idempotency-Key` on credential creates (a replay returns no secret).
- WRK-01: `credential_revoked_at`.
- TIME-01: `as_of` on Job and WorkerDetail.
- OVR-01: `terminal_counts_window`.
- CFG-01: `environment`, `build`, `contract_version`, `timeseries_window_max_days`.
- DOC-01: a 500 response is declared on every `/api/v1` op.
- RV01-14: closed `JobEventData`.

## Validation conventions (ERR-01/02)
- `details.fields[].field` is an RFC 6901 JSON Pointer from the request body root (`/payload/items/0/width`).
  Query parameters use `?name`; headers use `header:Name`.
- `rule` ∈ `required, type, format, range, length, pattern, enum, unknown_field, registered, schema,
  window_too_wide, window_inverted, too_far_in_future, retention_exceeded, scope_not_allowed, role_not_allowed`.

## Still open
- Owner: OQ-03 (viewer `/audit`) and the session lifetimes (idle 2 h, absolute 12 h).
- Deferred by the integrator: RT-01..03 and AUDIT-01/02 to M6; OBS-01/02 and RET-01 to M8; RV01-15 (claim top-N) to M4.
