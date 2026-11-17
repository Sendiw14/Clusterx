# CLUSTERX — Permissions matrix (route × role × field)

Status: **specified** (TASK-M1-BE-01). Source: MASTER_PROMPT R2 §4, §7; ADR-0005. Requirements FR-SEC-03, FR-SEC-04, FR-SEC-05, FR-SEC-11; AC-09, AC-13, AC-14.
Enforced in Go on **every** request (direct API and BFF alike). The BFF/UI may hide controls but is never the enforcement point.

## Principals

| Principal | How authenticated | Role source |
|---|---|---|
| Human via BFF | Service assertion (ADR-0005) + DB re-check of user/session | `users.role` (DB wins over assertion) |
| Direct API token | `cxt_` bearer, SHA-256 lookup | `api_tokens.scopes` ∩ owner's **current** `users.role` (view `api_token_authority`); owner disabled ⇒ 401 `user_disabled` (RV01-02) |
| Worker | `cxw_` bearer on gRPC only | worker credential; **no REST access** |
| BFF at login | login assertion (`purpose=login`, `sub=github:<id>`, no `sid`) | valid only for `POST /api/v1/sessions` (ADR-0006 §5) |
| Metrics scraper | static scrape token, internal listener | `/metrics` only |
| Anonymous | none | `/health/live`, `/health/ready` only |
| Demo visitor | none | **no control-plane access at all** (fixtures in web app) |

Token scopes: `jobs:read`, `jobs:write`, `workers:read`, `workers:write`, `metrics:read`, `audit:read`.
A viewer token can only hold `*:read` scopes (issuance rejects otherwise).

## Route matrix

| Operation | operator | viewer | token scope | Anonymous | Audit (privileged) |
|---|---|---|---|---|---|
| `POST /api/v1/jobs` | ✓ | ✗ 403 | `jobs:write` | ✗ 401 | no (job event `admitted` carries actor) |
| `GET /api/v1/jobs` | ✓ | ✓ (masked) | `jobs:read` | ✗ 401 | no |
| `GET /api/v1/jobs/{id}` | ✓ | ✓ (masked) | `jobs:read` | ✗ 401 | no |
| `GET /api/v1/jobs/{id}/attempts` | ✓ | ✓ (masked) | `jobs:read` | ✗ 401 | no |
| `GET /api/v1/jobs/{id}/events` | ✓ | ✓ (`data` is the closed `JobEventData` schema, so it has no maskable content; RV01-14) | `jobs:read` | ✗ 401 | no |
| `POST /api/v1/jobs/{id}/cancel` | ✓ | ✗ 403 | `jobs:write` | ✗ 401 | **yes** `job.cancel` |
| `POST /api/v1/jobs/{id}/retry` | ✓ | ✗ 403 | `jobs:write` | ✗ 401 | **yes** `job.manual_retry` |
| `GET /api/v1/workers` | ✓ | ✓ (masked) | `workers:read` | ✗ 401 | no |
| `GET /api/v1/workers/{id}` | ✓ | ✓ (masked) | `workers:read` | ✗ 401 | no |
| `POST /api/v1/workers/{id}/drain` | ✓ | ✗ 403 | `workers:write` | ✗ 401 | **yes** `worker.drain` |
| `GET /api/v1/metrics/overview` | ✓ | ✓ | `metrics:read` | ✗ 401 | no |
| `GET /api/v1/metrics/timeseries` | ✓ | ✓ | `metrics:read` | ✗ 401 | no |
| `GET /api/v1/audit` | ✓ (all categories) | ✓ `privileged` only, `details` masked | `audit:read` | ✗ 401 | no |
| `GET /health/live`, `/health/ready` | ✓ | ✓ | none | ✓ (status only) | no |
| `GET /metrics` | scrape token only | ✗ | — | ✗ 401 | no |
| `POST /api/v1/sessions` | BFF login assertion only (allowlisted GitHub id; role from DB) | same | **none** (API token ⇒ 403 `credential_not_allowed`) | ✗ 401 | security `session.create` |
| `DELETE /api/v1/sessions/current` | ✓ (own session) | ✓ (own session) | **none** (⇒ 403) | ✗ 401 | security `session.revoke` |
| `GET /api/v1/me` | ✓ | ✓ | **none** (⇒ 403) | ✗ 401 | no |
| `GET /api/v1/tokens` | ✓ | ✗ 403 | **none** (BFF session only; any API token ⇒ 403) | ✗ 401 | no |
| `POST /api/v1/tokens` | ✓ | ✗ 403 | **none** (session only) | ✗ 401 | **yes** `token.create` |
| `POST /api/v1/tokens/{id}/revoke` | ✓ | ✗ 403 | **none** (session only) | ✗ 401 | **yes** `token.revoke` |
| `GET /api/v1/worker-credentials` | ✓ | ✗ 403 | **none** (session only) | ✗ 401 | no |
| `POST /api/v1/worker-credentials` | ✓ | ✗ 403 | **none** (session only) | ✗ 401 | **yes** `worker_credential.create` |
| `POST /api/v1/worker-credentials/{id}/revoke` | ✓ | ✗ 403 | **none** (session only) | ✗ 401 | **yes** `worker_credential.revoke` |
| `GET /api/v1/handlers` | ✓ | ✓ | `jobs:read` | ✗ 401 | no |
| `GET /api/v1/config` | ✓ | ✓ | any read scope | ✗ 401 | no |
| `GET /api/v1/stream` (WebSocket, via BFF) | ✓ | ✓ (hints carry ids/versions only) | `jobs:read` — token streams re-validated ≤ 30 s, close 4401/4403 (RV01-13) | ✗ 401 | no |

Token/credential management is deliberately unavailable to API tokens, so a leaked token cannot mint
or revoke credentials. Rotation = create + revoke (CCR-01).

Denied mutations append `audit_events` (`category='security'`, `outcome='denied'`). Rule: 401 when the
principal is not established; 403 when established but not permitted. Object-level: single
organization, so any authenticated reader may read any job/worker ID (no BOLA across tenants);
nonexistent IDs return 404 identically for all roles.

## Field matrix (viewer masking)

Masked fields are returned as `null` and named in `masked_fields`. Never partially redacted.

| Resource | Field | operator | viewer |
|---|---|---|---|
| Job | `payload` | ✓ | masked |
| Job | `result` | ✓ | masked |
| Job | `error_message` | ✓ | masked (`error_code` visible) |
| Job | `created_by` | ✓ | ✓ (principal ref only, no login/email) |
| Attempt | `error_message`, `log_tail` | ✓ | masked |
| Worker | `hostname` | ✓ | masked |
| Worker | credential id/prefix/hash | never via these routes | never |
| Audit | `details` | ✓ | masked |
| Audit | `security`/`diagnostic` rows | ✓ | not returned |
| Audit | `actor` | ✓ | ✓ principal ref only (OQ-03 proposal — **needs owner nod**) |
| API token / worker credential | `secret` | **once**, in the 201 create response only (`Cache-Control: no-store`) | never |
| API token / worker credential | hash | never | never |
| Attempt | `log_truncated`, `log_expired` | ✓ | ✓ (flags only) |
| Any | secret hashes, session ids, tokens | **never returned to anyone** | never |

## Worker (gRPC) permissions

| RPC | Requirement |
|---|---|
| Register | valid, unrevoked, unexpired credential; credential not bound to another live worker unless `worker_id` matches |
| Heartbeat, Drain, Deregister | credential bound to `worker_id`; incarnation current |
| Claim | above + worker `active` + capacity |
| RenewLease, ReportResult, ReportFailure, PollCancellation | full fence predicate (state-machine.md) |

## Tests required (AC-13, AC-09)
Table-driven: every row above × {operator, viewer, operator-token-wrong-scope, viewer-token, revoked token,
expired assertion, wrong audience, forged forwarded headers, anonymous} asserting status + masking;
worker RPC × {revoked credential, wrong incarnation, other worker's credential}.
