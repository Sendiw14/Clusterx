# ADR-0005 — Trust boundaries and authentication

- Status: **Proposed** (TASK-M1-BE-01). Review: TASK-M1-RV-01 (security). Owner decision OD-03 pending.
- Requirements: FR-SEC-01..07, FR-SEC-11, FR-DEMO-01, FR-RT-01; AC-09, AC-13, AC-14, AC-16. Master §4, §7.
- Detail: `docs/security/trust-boundaries.md`, `docs/security/permissions.md`.

## Decision
1. **Browser → Next.js BFF**: GitHub OAuth authorization code + PKCE + state via a maintained library;
   server-held session (`sessions` table, hash of session secret), cookie `__Host-clusterx_session`
   (Secure, HttpOnly, SameSite=Lax, Path=/). Mutating BFF routes require same-origin `Origin` check +
   CSRF token. The browser never holds Go or worker credentials.
2. **BFF → Go**: every call carries `Authorization: Bearer <service assertion>` — a JWS (EdDSA/Ed25519)
   with `iss=clusterx-bff`, `aud=clusterx-control-plane`, `sub=user:<uuid>`, `role`, `sid` (session id),
   `iat`, `exp ≤ iat+60 s`, `jti`. Go verifies signature with a pinned public key, audience, expiry
   (±5 s skew, DB-independent), then **re-checks** `users.disabled_at IS NULL`, the user's current role
   and that the session `sid` is not revoked/expired (authoritative DB check for mutations; ≤ 5 s cache
   for reads). Role in the assertion is never trusted above the DB role. Forwarded identity headers
   (`X-User`, `X-Forwarded-User`, …) are ignored.
3. **Direct API clients**: `Authorization: Bearer cxt_<prefix>_<secret>`; Go hashes (SHA-256 of a
   256-bit random secret) and looks up `api_tokens`; checks revoked/expired, role and scopes
   (`jobs:read`, `jobs:write`, `workers:read`, `workers:write`, `metrics:read`, `audit:read`).
   Effective permission = role ∩ scopes. `last_used_at` updated asynchronously. Token issuance/revocation
   is an operator console action via the BFF (endpoints are an open contract item — not in §6 list).
4. **Workers → Go (gRPC)**: TLS when non-loopback; metadata `authorization: Bearer cxw_<prefix>_<secret>`
   checked against `worker_credentials` on **every** RPC (no cache for Claim/Renew/Report). Credential
   bound to the worker row at Register; `worker_id`+`incarnation_id` in each request must match the
   bound worker. Workers have no DB access and no REST access.
5. **Demo isolation**: `/demo` uses bundled fixtures only; the demo build has no control-plane base URL
   or credentials configured; enforced by a network-isolation test (AC-14). No demo login bypass.
6. **WebSocket** `/api/v1/stream` (proposed, not in §6 list — open contract item): upgrade requires a
   fresh service assertion (via BFF) and allowed `Origin`; server closes the socket at assertion/session
   expiry and on logout (session revoked check every ≤ 30 s). Messages are hints `{type, job_id, version}`
   only; clients reconcile via HTTP.
7. Every REST operation is authorized in Go (role, scope, object, field); see permissions matrix.
   Privileged actions append `audit_events` (`category='privileged'`).

## Consequences
- The BFF signing key is the highest-value secret on the web host; rotation via a key id (`kid`) list.
- Revocation of a session takes effect on the next mutation immediately and on reads within 5 s.
