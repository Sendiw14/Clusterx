# ADR-0006 — BFF session contract (assertions, CSRF, auth outcomes, expiry)

- Status: **Proposed** (TASK-M1-BE-02, 2026-11-17). Implements CCR-09 from `docs/coordination/CCR-M1-decisions.md`.
  Answers frontend OQ-02 and gives a security opinion on OQ-01. Review: TASK-M1-RV-01 (auth).
- Requirements: FR-SEC-01, FR-SEC-02, FR-SEC-06, FR-SEC-07, FR-WEB-09, FR-RT-01; AC-13, AC-16. Master §4, §7.
- Builds on ADR-0005. Owner: backend (contract); implementer: frontend lane (Next.js BFF).

## 1. Service assertion (BFF → Go)

JWS compact, header `{alg: "EdDSA", typ: "clusterx-assertion+jwt", kid}`. Claims:

| Claim | Value | Go check |
|---|---|---|
| `iss` | `clusterx-bff` | exact |
| `aud` | `clusterx-control-plane` | exact |
| `sub` | `user:<uuid>` | user exists, `disabled_at IS NULL` |
| `sid` | session UUID | session exists, not revoked, not expired (DB) |
| `role` | `operator` \| `viewer` | **hint only**. Go uses `users.role`; a mismatch is logged as security event |
| `iat`, `nbf` | now | ±5 s skew |
| `exp` | ≤ `iat + 60 s` | reject if past (±5 s) |
| `jti` | random 128-bit | mutations: single-use within 120 s (in-memory replay cache; single coordinator, ADR-0001) |
| `rid` | request id | echoed as `X-Request-Id` |

A new assertion is minted for every upstream request (or WebSocket upgrade); the BFF never forwards the
browser cookie to Go. Keys: Ed25519, public keys pinned in Go config by `kid`; rotation = add new `kid`,
switch signing, remove old after 2× max TTL.

## 2. Go auth outcomes (OQ-02 answer)

Go answers 401 `unauthenticated` with a machine-readable `details.reason`. The BFF maps it:

| `details.reason` | Meaning | BFF action | Browser sees |
|---|---|---|---|
| `session_expired`, `session_revoked`, `user_disabled` | The human session ended in the authoritative store | destroy BFF session, clear cookie | 401 `session_expired` → `/login?next=` (clamped same-origin path) |
| `assertion_expired` | Clock skew or slow request | retry **once** with a fresh assertion | transparent; if it repeats → 502 `upstream_auth` |
| `assertion_invalid` | Bad signature / `kid` / `aud` / `iss` / replayed `jti` — a BFF or config fault, not the user's | **do not** end the session; log with request id | 502 `upstream_auth` (S-ERR with request_id); never a login redirect |
| `token_*` | Direct API token problems | n/a (BFF never uses API tokens) | — |

A 403 `forbidden` from Go is passed through as 403 (role changed or insufficient). The BFF then refreshes
the session's effective role (see §5). **No login loop:** login is offered only for the first row group.
If a fresh login immediately produces `assertion_invalid` again, the UI still shows the 502 infrastructure
error.

## 3. Browser ↔ BFF

- Cookie `__Host-clusterx_session`: `Secure; HttpOnly; SameSite=Lax; Path=/`; value = opaque 256-bit session
  secret (Go stores only its SHA-256 in `sessions.secret_hash`). Regenerated at login (fixation defense).
- Session lifetime (proposed, owner may tune): idle 2 h, absolute 12 h. `expires_at` is absolute; idle
  timeout is enforced by the BFF and reflected by revoking the session row.
- **CSRF:** every mutating BFF route (POST/PUT/PATCH/DELETE) requires both
  (a) `Origin` header exactly equal to the configured app origin (missing Origin ⇒ reject), and
  (b) header `X-CSRF-Token` equal to `HMAC-SHA256(csrf_key, sid)` (base64url), obtained from the session-info route.
  Failure ⇒ 403 `csrf_failed` (BFF-owned code). GET routes are side-effect free.
- OAuth: GitHub authorization code + PKCE (S256) + `state` (≥128-bit, single use, 10 min); allowlist by
  numeric GitHub user id; `next` redirect clamped to same-origin relative paths starting with `/app`.
- BFF-owned routes (Next.js, not Go): `GET /bff/session` → `{user:{id, login}, role, expires_at,
  idle_expires_at, csrf_token}`; `POST /bff/logout`; `/login`, `/auth/callback`. 401 from `/bff/session` =
  signed out.
- Session-expiry surfacing: `/bff/session` gives both expiries; the stream sends `auth_expiring` 60 s before
  expiry and closes 4401 at expiry; any BFF route after expiry returns 401 `session_expired`.

## 4. WebSocket

Browser → BFF `/bff/stream` (Origin allowlist + session cookie; BFF max 5 sockets per session).
BFF → Go `/api/v1/stream` with `Authorization: Bearer <assertion>`. Go re-validates `sid` every ≤ 30 s and
on logout closes with 4401; the BFF relays close codes unchanged. Messages are hints only (OpenAPI
`StreamMessage`); after reconnect or `resync` the UI refetches snapshots (FR-RT-01, AC-16).

## 5. Open item — CCR-10 (proposal, NOT added to OpenAPI)

Go owns `users` and `sessions`, so the BFF needs three server-to-server operations that §6 and CCR-M1 do
not list. Proposed, pending integrator decision:

| Proposed op | Auth | Purpose |
|---|---|---|
| `POST /api/v1/sessions` | assertion with `sub=github:<numeric id>`, `purpose=login`, no `sid` | allowlist check, create session row, return `{user_id, role, session_id, expires_at, secret}` |
| `DELETE /api/v1/sessions/current` | normal assertion | logout → revoke `sid` |
| `GET /api/v1/me` | normal assertion | effective role + session expiry (used by `/bff/session` and after a 403) |

Until CCR-10 is decided, the BFF cannot complete a real login against Go (M6 dependency). Alternative
if rejected: BFF-owned session store with Go trusting `sid` only for audit. That loses Go-side immediate
revocation, so this ADR recommends against it.

## 6. OQ-01 security opinion (new-job draft in `sessionStorage`)

Acceptable: per-tab, cleared on submit success and on logout. Payloads must not contain secrets by policy
(§4), and `sessionStorage` is never sent to the server. Residual risk: XSS can read it, but XSS could also
submit as the user. Do **not** use `localStorage`.

## Consequences
- The BFF signing key is the main impersonation risk. Go's DB re-check of `sid`/user/role bounds what a
  stolen key can do to allowlisted users with live sessions.
- Per-request DB session checks cost about one indexed lookup per API call. That is acceptable at 1.0 scale
  and should be re-measured in the AC-19 benchmark.
