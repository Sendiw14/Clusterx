# CLUSTERX — Permissions matrix (route × role × field)

Status: **specified** (TASK-M1-BE-01). Source: MASTER_PROMPT R2 §4, §7; ADR-0005. Requirements FR-SEC-03, FR-SEC-04, FR-SEC-05, FR-SEC-11; AC-09, AC-13, AC-14.
Enforced in Go on **every** request (direct API and BFF alike). The BFF/UI may hide controls but is never the enforcement point.

## Principals

| Principal | How authenticated | Role source |
|---|---|---|
| Human via BFF | Service assertion (ADR-0005) + DB re-check of user/session | `users.role` (DB wins over assertion) |
| Direct API token | `cxt_` bearer, SHA-256 lookup | `api_tokens.role` ∩ `api_tokens.scopes` |
| Worker | `cxw_` bearer on gRPC only | worker credential; **no REST access** |
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
| `GET /api/v1/jobs/{id}/events` | ✓ | ✓ | `jobs:read` | ✗ 401 | no |
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
| Token issue/revoke/rotate *(open contract item)* | ✓ | ✗ 403 | none (session only, never via token) | ✗ | **yes** |
| Worker credential issue/revoke/rotate *(open contract item)* | ✓ | ✗ 403 | none (session only) | ✗ | **yes** |
| WebSocket `/api/v1/stream` *(open contract item)* | ✓ | ✓ (hints carry no sensitive fields) | `jobs:read` | ✗ | no |

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
