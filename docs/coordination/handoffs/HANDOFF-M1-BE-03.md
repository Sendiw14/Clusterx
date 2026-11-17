# HANDOFF — TASK-M1-BE-03 (CCR-10 BFF session lifecycle)

- **Author, model/effort:** backend lane subagent, model `claude-opus-5-5`. Requested effort: medium. Effective effort and Ultracode: UNVERIFIED (MW-04).
- **Branch:** `m1/backend`. Not pushed, not merged. Source decision: `main:docs/coordination/CCR-M1-decisions.md` Round 2 (CCR-10 ACCEPT).
- **Scope respected:** DDL, PoC and ADR-0001..0005 are untouched. The reviewer snapshot `ee1fa3d` is still valid for those files.

## Commits
| SHA | Content |
|---|---|
| `04cb7d1` | OpenAPI `1.0.0-draft.3`: `POST /api/v1/sessions`, `DELETE /api/v1/sessions/current`, `GET /api/v1/me`; `bffLoginAssertion` security scheme; schemas `SessionCreate`, `SessionUser`, `SessionCreated`, `Me`; tag `sessions` |
| `23d998b` | permissions.md (3 rows + login principal), api-contracts.md, ADR-0006 status/§5 |
| (this) | this handoff |

## Contract summary
- `POST /api/v1/sessions` accepts only the BFF login assertion (`sub=github:<id>`, `purpose=login`, no `sid`). The GitHub id comes from the signed `sub`, never from the body.
  - 201 returns `{session_id, secret (once, no-store), user{id, github_login}, role, expires_at, idle_timeout_seconds, idle_expires_at}`.
  - 403 returns `details.reason` ∈ `not_allowlisted` | `user_disabled` | `credential_not_allowed`.
  - 401 only for `assertion_invalid` | `assertion_expired`.
- `DELETE /api/v1/sessions/current` returns 204. Revocation is immediate, and streams close 4401 within ≤ 30 s. An already revoked or expired session gets 401 with that reason.
- `GET /api/v1/me` returns user, the role from the DB, and the session's id/created/expires/idle timeout.
- An API token on any of the three gets 403 `credential_not_allowed`. Session create/revoke are audited (`security` category).

## Commands, exit codes, results
| Check | Command | Exit | Result |
|---|---|---|---|
| YAML parse + `$ref` | python3/PyYAML script | 0 | `version 1.0.0-draft.3 paths: 25 ops: 28 refs: 50 unresolved: []` |
| Lint | `npx --yes @redocly/cli@2.79.0 lint api/openapi.yaml` | 0 | `Woohoo! Your API description is valid. 🎉` / `You have 5 warnings.` |

Lint warnings, verbatim rule names (all expected, unchanged since draft.2):
- `[1] api/openapi.yaml:4:1 at #/info` — info-license (OD-02 pending)
- `[2] api/openapi.yaml:28:10 at #/servers/0/url` — no-server-example.com (localhost dev server)
- `[3] api/openapi.yaml:783:7 at #/paths/~1api~1v1~1stream/get/responses` — operation-2xx-response (WebSocket returns 101)
- `[4] …:800:7 /health/live` — operation-4xx-response (by design)
- `[5] …:812:7 /health/ready` — operation-4xx-response (by design)

## NOT RUN / limits
- No implementation or tests for these endpoints exist yet (M6). TS client generation: NOT RUN.
- **Idle timeout:** Go returns `idle_expires_at`, but cannot enforce it without a `sessions.last_activity_at` column. Until then the BFF enforces it by revoking the session. The DDL change is deferred to the reviewer-findings ticket.
- Login `jti` single-use relies on the same in-memory cache as ADR-0006 §1 (single coordinator).

## Open questions
- **Owner:**
  - OQ-03: may viewers read `/audit`?
  - Are session lifetimes of idle 2 h and absolute 12 h acceptable?
- **Integrator:** may the `sessions.last_activity_at` DDL be added in the reviewer-findings ticket?
- **Frontend (FE-02):** confirm that the 403 `not_allowlisted` and `user_disabled` reasons render as the `/login` no-entitlement state (§8).
