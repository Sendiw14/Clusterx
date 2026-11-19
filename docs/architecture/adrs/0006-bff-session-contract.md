# ADR-0006 — BFF session contract (assertions, CSRF, auth outcomes, expiry)

- Status: **Proposed** (TASK-M1-BE-02, 2026-11-17); §5 **accepted by integrator** as CCR-10 (Round 2) and specified in
  OpenAPI `1.0.0-draft.3` (TASK-M1-BE-03). Amended TASK-M1-BE-04 for RV01-11/12/13 and FE gaps SESS-01,
  BFF-01, BFF-02 (OpenAPI draft.4). Fixes pending RV-01 re-review; owner nod on lifetimes still pending.
  Implements CCR-09 from `docs/coordination/CCR-M1-decisions.md`.
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
| `sid` | session UUID | session exists, not revoked, not past `expires_at`, not idle-expired, and **`sessions.user_id` = `sub`** (RV01-11) |
| `role` | `operator` \| `viewer` | **hint only**. Go uses `users.role`; a mismatch is logged as security event |
| `iat`, `nbf` | now | ±5 s skew |
| `exp` | ≤ `iat + 60 s` | reject if past (±5 s) |
| `jti` | random 128-bit | mutations: single-use within 120 s (in-memory replay cache; single coordinator, ADR-0001). **Residual (RV01-12):** the cache is lost on coordinator restart, so a captured mutation assertion can be replayed once more until its `exp` (≤ 65 s incl. skew). Reads and WebSocket upgrades are never single-use and can be replayed until `exp`. Accepted for 1.0 because assertions only travel BFF→Go over TLS; recorded in trust-boundaries B2 |
| `rid` | request id | echoed as `X-Request-Id` |

A new assertion is minted for every upstream request (or WebSocket upgrade); the BFF never forwards the
browser cookie to Go. Keys: Ed25519, public keys pinned in Go config by `kid`; rotation = add new `kid`,
switch signing, remove old after 2× max TTL.

## 2. Go auth outcomes (OQ-02 answer)

Go answers 401 `unauthenticated` with a machine-readable `details.reason`. The BFF maps it:

| `details.reason` | Meaning | BFF action | Browser sees |
|---|---|---|---|
| `session_expired` (`details.expiry` = `idle` \| `absolute`), `session_revoked` | The human session ended in the authoritative store | destroy BFF session, clear cookie | 401 `session_expired` → `/login?reason=session_expired&next=` (clamped same-origin path) |
| `user_disabled` | The account was disabled | destroy BFF session, clear cookie | `/login?error=user_disabled` (no-entitlement panel, not a plain re-login prompt — re-login would 403) |
| `assertion_expired` | Clock skew or slow request | retry **once** with a fresh assertion | transparent; if it repeats → 502 `upstream_auth` |
| `assertion_invalid` | Bad signature / `kid` / `aud` / `iss` / replayed `jti` — a BFF or config fault, not the user's | **do not** end the session; log with request id | 502 `upstream_auth` (S-ERR with request_id); never a login redirect |
| `token_*` | Direct API token problems | n/a (BFF never uses API tokens) | — |

A 403 `forbidden` from Go is passed through as 403 (role changed or insufficient). The BFF then refreshes
the session's effective role (see §5). **No login loop:** login is offered only for the first row group.
If a fresh login immediately produces `assertion_invalid` again, the UI still shows the 502 infrastructure
error.

## 3. Browser ↔ BFF

- Cookie `__Host-clusterx_session`: `Secure; HttpOnly; SameSite=Lax; Path=/`; value = an **authenticated-encrypted
  (sealed) `{sid, uid, login, role, exp}` envelope** with a 1-byte key-version prefix, sealed by the BFF with keys derived from
  `CLUSTERX_WEB_SESSION_KEY` (label-separated from the CSRF key). The `cxs_` session secret returned by
  `POST /sessions` is **not** stored in the cookie (Go authenticates BFF requests by assertion `sid`, not by the
  secret; Go keeps only its SHA-256). Regenerated at login (fixation defense); a still-valid previous session is
  revoked best-effort at re-login. *(Amended 2026-11-20 per REVIEW-M6-RV-01 RV6-07; supersedes the "value =
  opaque secret" text.)*
- **Key-compromise residual (B1/B2):** `CLUSTERX_WEB_SESSION_KEY` lets its holder forge a cookie for any live
  `sid`; together with the BFF signing key it is equivalent to impersonating any signed-in user. Both keys are
  BFF-host secrets of the same sensitivity, loaded from files, never bundled or logged. Rotation: add a new key
  version, accept the previous version for one absolute-session lifetime (12 h), then remove it.
- Session lifetime (proposed, owner may tune): idle 2 h, absolute 12 h. **Go enforces both (GAP-SESS-01):**
  a session is valid iff `revoked_at IS NULL AND expires_at > now() AND last_activity_at +
  idle_timeout_seconds > now()`. Go bumps `last_activity_at` at most once per 60 s per session on
  authenticated requests (a hint-only stream does not count as activity). The BFF keeps no timers.
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
- **Login outcomes (closed set, GAP-BFF-01).** `/login?error=` ∈ {`auth_error` (OAuth denied/state mismatch),
  `no_entitlement` (Go 403 `not_allowlisted`), `user_disabled` (Go 403/401 `user_disabled`),
  `provider_unavailable` (GitHub unreachable), `upstream_auth` (Go `assertion_*` or `credential_not_allowed`)};
  `/login?reason=` ∈ {`signed_out`, `session_expired`}. Any other value renders nothing.
- **Proxy envelope (GAP-BFF-02).** For `/api/v1` calls the BFF passes Go's status code, body and these headers
  through unchanged: `Content-Type`, `X-Request-Id`, `Retry-After`, `Idempotent-Replayed`, `Location`, `ETag`,
  `Cache-Control`, `Date`. It never rewrites Go error bodies. BFF-owned failures use the same
  `{code, message, details?, request_id}` envelope: `session_expired` (401), `csrf_failed` (403),
  `upstream_auth` (502), `upstream_unavailable` (503, Go unreachable).

## 4. WebSocket

Browser → BFF `/bff/stream` (Origin allowlist + session cookie; BFF max 5 sockets per session).
BFF → Go `/api/v1/stream` with `Authorization: Bearer <assertion>`. Go re-validates `sid` (and the
`sid`↔`sub` binding) every ≤ 30 s and on logout closes with 4401; the BFF relays close codes unchanged.
**Streams opened with an API token** (direct clients) are re-validated on the same ≤ 30 s cadence against
`api_token_authority` (RV01-13). They close 4401 when the token is revoked or expired or its owner is
disabled, and 4403 when the effective scopes no longer include `jobs:read`. Messages are hints only (OpenAPI
`StreamMessage`); after reconnect or `resync` the UI refetches snapshots (FR-RT-01, AC-16).

## 5. Session lifecycle — CCR-10 (accepted, OpenAPI draft.3)

Go owns `users` and `sessions`, so the BFF uses three server-to-server operations. API tokens get 403
`credential_not_allowed` on all three. Login failures for an authenticated GitHub identity return 403 with
`not_allowlisted` or `user_disabled`. Idle timeout is enforced by Go (§3).

**Key-compromise bound (RV01-11, corrected).** For normal requests Go requires `sessions.user_id = sub`, so a
stolen BFF key cannot use one user's live session as another user. **However, `POST /api/v1/sessions`
trusts the login assertion's `sub=github:<id>`. Whoever holds the BFF signing key can mint a session for
any allowlisted user, which is full impersonation.** Go does not re-verify the GitHub OAuth exchange in 1.0.
Mitigations:
- The key lives only on the BFF host and is rotated by `kid`.
- Every `session.create` is a `security` audit event carrying the BFF `kid` and request id.
- Bursts of logins are alertable (M8).

Verifying the OAuth code exchange in Go is a possible post-1.0 hardening.

| Proposed op | Auth | Purpose |
|---|---|---|
| `POST /api/v1/sessions` | assertion with `sub=github:<numeric id>`, `purpose=login`, no `sid` | allowlist check, create session row, return `{user_id, role, session_id, expires_at, secret}` |
| `DELETE /api/v1/sessions/current` | normal assertion | logout → revoke `sid` |
| `GET /api/v1/me` | normal assertion | effective role + session expiry (used by `/bff/session` and after a 403) |

Rejected alternative: a BFF-owned session store with Go trusting `sid` only for audit. It would lose Go-side
immediate revocation.

## 6. OQ-01 security opinion (new-job draft in `sessionStorage`)

Acceptable: per-tab, cleared on submit success and on logout. Payloads must not contain secrets by policy
(§4), and `sessionStorage` is never sent to the server. Residual risk: XSS can read it, but XSS could also
submit as the user. Do **not** use `localStorage`.

## Consequences
- The BFF signing key is the main impersonation risk. With it an attacker can impersonate any allowlisted
  user (via `POST /api/v1/sessions`). The `sid`↔`sub` binding only stops cross-user reuse of existing sessions.
- Per-request DB session checks cost about one indexed lookup per API call. That is acceptable at 1.0 scale
  and should be re-measured in the AC-19 benchmark.

## 7. Implementation notes (TASK-M5-BE-01, OpenAPI draft.6)

- **Keys.** Go reads `CLUSTERX_BFF_JWKS_FILE`: a JWKS of Ed25519 OKP public keys (`{"keys":[{"kty":"OKP","crv":"Ed25519","kid":"…","x":"…"}]}`,
  1..16 keys). A JWK with a private member `d` is refused at start-up. Without the variable every assertion is rejected
  (`assertion_invalid`) and only API tokens work. Rotation = add the new `kid` to the JWKS, restart, switch signing, drop the old `kid`.
- **Header/claims are strict.** `alg` must be `EdDSA`, `typ` `clusterx-assertion+jwt`; the header members are exactly a subset
  of `alg, typ, kid` (so no `crit`, `jku`, `x5u` …). The claim set is closed:
  `iss, aud, sub, sid, role, purpose, iat, nbf, exp, jti, rid` (unknown claims ⇒ `assertion_invalid`). In both objects member
  names must match exactly (lower case) and appear once; duplicates or case variants ⇒ `assertion_invalid` (RV5-03). `jti` is 16..128
  base64url characters (128 random bits recommended). `exp - iat <= 60 s`; `iat`/`nbf` may lead the Go clock by at most 5 s;
  `exp` may lag it by at most 5 s (`assertion_expired`).
- **Single use.** Mutations (non-GET/HEAD) and every login assertion consume the `jti` in an in-memory cache (120 s,
  at most 100 000 live entries; when full, new mutations fail closed with `assertion_invalid`). GET and WebSocket upgrades
  do not consume it. The RV01-12 restart residual stands.
- **Sessions.** `secret` = `cxs_` + 43 base64url characters (256 bits); only its SHA-256 is stored. Unknown `sid` ⇒
  `session_revoked`; `sid` of another user ⇒ `assertion_invalid` + security audit `assertion.sid_mismatch`; role hint ≠ DB role ⇒
  security audit `assertion.role_mismatch` (DB role used). Every request re-checks the session in PostgreSQL (no cache, stricter
  than the ≤ 5 s read cache allowed above). Lifetimes: `CLUSTERX_SESSION_IDLE_TIMEOUT` (default 2 h), `CLUSTERX_SESSION_ABSOLUTE_TTL`
  (default 12 h).
- **Development.** `clusterx-control-plane dev-keygen --out-dir D` writes `bff-dev-private.jwk.json` (0600) and
  `bff-dev.jwks.json`; `dev-assertion --key-file F (--login <github id> | --sub user:<uuid> --sid <uuid> --role r)` prints one
  assertion. Both refuse `CLUSTERX_ENV=production`. They are local test helpers, not a login bypass: users must still be
  allowlisted (`clusterx-control-plane user upsert --github-id N --login L --role operator|viewer`).
- **CSRF** stays a BFF concern (§3); Go never sees browser cookies, so there is no CSRF surface in Go.
