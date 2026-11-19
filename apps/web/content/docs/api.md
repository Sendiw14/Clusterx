---
title: API reference
lead: The REST contract under /api/v1. The operation reference below is generated from api/openapi.yaml when the site is built; the sections here explain the rules that apply to every call.
description: CLUSTERX REST API reference generated from the OpenAPI document - authentication, errors, request IDs, pagination, idempotency, version policy and every operation.
blurb: Generated REST reference with authentication, errors, request IDs, idempotency and the version policy.
order: 4
---

> [!WARNING] The contract is a **draft** and is not frozen. The version shown below is read from the OpenAPI document at build time. Names, fields and status codes can still change before 1.0, and some operations are described in the document but not served yet. Each operation says which.

## Base URL and versioning

All API routes live under `/api/v1`. Health endpoints (`/health/live`, `/health/ready`) and the metrics endpoint (`/metrics`) are outside that prefix. The local Compose stack serves the API on `http://127.0.0.1:8080`.

The `version` field of the OpenAPI document is the API version, and `GET /api/v1/config` returns the `contract_version` the running control plane implements.

**Version policy.** While the version carries a `draft` suffix there is no compatibility promise. A policy for 1.0 and later (what counts as a breaking change, how long an old version is served) has not been written, and this page will not state one before it exists. The path prefix `/api/v1` is the only versioning mechanism in the contract today.

## Authentication

Direct clients send a scoped API token as a bearer credential:

```http
Authorization: Bearer cxt_<prefix>_<secret>
```

Tokens are created by an operator. The secret is shown once and only its hash is stored. Effective authority is the token's scopes intersected with the permissions of its owner's current role, evaluated on every request.

Scopes: `jobs:read`, `jobs:write`, `workers:read`, `workers:write`, `metrics:read`, `audit:read`. Tokens never reach token or credential management, or the session routes. Tokens and secrets never belong in browser code.

The web console uses a server-side session instead, through the Next.js server (built in M6; see [Security](/docs/security)). Workers do not use REST at all: they use gRPC with a worker credential.

A `401` body carries `details.reason`, for example `token_invalid`, `token_revoked`, `token_expired` or `user_disabled`. A `403` carries `details.required` and `details.role` or the missing scope.

## Roles and masking

Operators can read and write. Viewers can only read, and fields marked sensitive in the schemas (job `payload`, `result` and `error_message`, attempt `error_message` and `log_tail`, worker `hostname`) come back as `null` with their names listed in `masked_fields`.

## Request IDs

Every response carries an `X-Request-Id` header. If you send a valid `X-Request-Id` it is echoed; otherwise one is generated. The same id is in the `request_id` field of every error body and in the server's logs and audit records. **Quote it when you report a problem.**

## Errors

Every error uses one shape:

```json
{
  "code": "validation_failed",
  "message": "Request has invalid fields.",
  "details": { "fields": [ { "field": "/priority", "rule": "range", "message": "must be between 0 and 9" } ] },
  "request_id": "req_01JD3J"
}
```

`code` is a stable machine-readable string from the list in the generated section below. `message` is safe to display and never contains a secret or payload content. `details` varies by code. For `validation_failed`, each entry names the field as an RFC 6901 JSON Pointer (query parameters as `?name`, headers as `header:Name`) and a `rule`.

A `410` with code `expired` means the resource existed and was deleted by retention. It is different from a `404`, which means the id never existed. An unknown path answers `404 not_found_route` and a known path with the wrong method answers `405 method_not_allowed` with an `Allow` header.

The generated section lists every status the contract defines.

## Pagination

List endpoints use opaque cursors. Pass the `next_cursor` of one page as `cursor` in the next request, with the same filters; a cursor used with different filters is rejected with `invalid_cursor`. `limit` is 1 to 200 (default 50). Sorting is stable with the id as a tie-break. Rows created after you fetched the first page that sort before your cursor are not returned in that traversal.

## Idempotency

`POST /api/v1/jobs` accepts an `Idempotency-Key` header. The first request answers `201`; an identical replay within 24 hours answers `200` with `Idempotent-Replayed: true` and the original job; the same key with a different request answers `409 idempotency_conflict`. After an uncertain failure, such as a 503 during commit, retry with the same key. See [Concepts](/docs/concepts).

## Reconciliation

`GET /api/v1/jobs/{id}` returns the job with its `version`, and `GET /api/v1/jobs/{id}/events?after_version=v` returns what happened after that version, so a client can combine a snapshot with the events after it without gaps. Live notifications, when they exist, are hints only; the HTTP responses are authoritative.

## Limits

| Limit | Value |
| --- | --- |
| Request body | 80 KiB (413 `payload_too_large`) |
| Job payload | 64 KiB of JSON, nesting depth at most 32 |
| Result | 128 KiB |
| `priority` | 0 to 9, where 9 is the highest |
| `max_attempts` | 1 to 5, including the first run |
| `timeout_seconds` | 1 to 86,400 |
| `run_after` | At most 30 days in the future |
| Page size | 1 to 200 |

Rate limiting is implemented: the control plane returns `429 rate_limited` with `Retry-After` when a per-user mutation limit or the failed-authentication limit is exceeded.

## About the examples

Request and response examples are taken from the OpenAPI document and are **illustrative**. Some use handler names such as `thumbnail.render` that are not in the example catalogue. The handlers the current build accepts are `hash_sha256`, `sleep_ms` and `fail_n_times`; call `GET /api/v1/handlers` to list the ones your control plane accepts.
