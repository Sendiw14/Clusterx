# CLUSTERX — API contracts index (M1 draft)

Status: **specified**, not frozen. Integrator freezes at M1 exit (§1.3 step 3).

| Contract | Path | Version | Validation run in M1 |
|---|---|---|---|
| REST | `api/openapi.yaml` | OpenAPI 3.1.1, API `1.0.0-draft.2` | PyYAML parse + `$ref` check; `npx @redocly/cli@2.79.0 lint` → valid, 0 errors, 5 intended warnings (TASK-M1-BE-02) |
| gRPC | `packages/proto/clusterx/worker/v1/worker.proto` | package `v1`, protocol_version 1 | `protoc --descriptor_set_out=/dev/null` |
| SQL | `apps/control-plane/migrations/0001_init.sql` | 0001 | `psql -v ON_ERROR_STOP=1 --single-transaction` on PG 18 |
| State table | `docs/architecture/state-machine.md` | M1 | PoC tests for AC-02/03/06 |

## Cross-cutting rules
- Error body: `{code, message, details?, request_id}`; codes enumerated in `Error.code` and state-machine.md.
- Request id: `X-Request-Id` in/out; included in logs, audit rows and error bodies.
- Pagination: opaque base64url cursor encoding the last row's sort key **and** a hash of the filters;
  stable keyset sort with `id` tie-break; `limit` 1–200; jobs list time window ≤ 31 days.
- Reconciliation (FR-RT-01): `GET /jobs/{id}` returns `version`; `GET /jobs/{id}/events?after_version=v`
  returns the gap. WebSocket hints `{type:"job.changed", job_id, version}` are advisory only.
- Masking: viewer sees `null` + `masked_fields` (see docs/security/permissions.md).
- Idempotency: `Idempotency-Key` on `POST /jobs` and `POST /jobs/{id}/retry` (ADR-0004).

## Contract items resolved by integrator (CCR-M1-decisions.md, implemented in draft.2)
CCR-01 tokens + worker credentials · CCR-02 `GET /handlers` · CCR-03 `GET /stream` · CCR-04 `GET /config` ·
CCR-07 attempt log flags · CCR-09 → ADR-0006. Deferred: CCR-05/06 (M6; interim `/audit?subject_type=&subject_id=`), CCR-08 (M8).

## Still open
- **CCR-10 (proposed in ADR-0006, not in OpenAPI):** BFF↔Go session lifecycle (login exchange, logout, `me`).
  Go owns `users`/`sessions`, so the BFF needs a way to create/revoke sessions and read the effective role.
