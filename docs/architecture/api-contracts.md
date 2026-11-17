# CLUSTERX — API contracts index (M1 draft)

Status: **specified**, not frozen. Integrator freezes at M1 exit (§1.3 step 3).

| Contract | Path | Version | Validation run in M1 |
|---|---|---|---|
| REST | `api/openapi.yaml` | OpenAPI 3.1.1, API `1.0.0-draft.1` | YAML parse + `$ref` resolution (python3 PyYAML); **full OpenAPI schema validation NOT RUN** (no validator installed) |
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

## Open contract items (need integrator/owner decision; NOT invented as endpoints)
1. Token issuance/revocation and worker-credential issuance endpoints — required by §8 `/app/settings/tokens`
   and §7, but not in the §6 operation list.
2. WebSocket endpoint path/handshake — §6 requires the channel but names no path.
3. BFF login/session endpoints live in Next.js (frontend lane under Opus auth contract).
4. Handler registry listing for `/app/jobs/new` (allowlisted handler select + schema) — not in §6 list.
