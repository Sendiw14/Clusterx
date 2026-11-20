---
title: Security
lead: Who can do what, how each boundary is protected, how workers are trusted and why the public demo cannot reach real data. No compliance is claimed.
description: Authentication and roles, API tokens, worker trust, demo separation, threat-model summary and reporting policy for CLUSTERX.
blurb: Authentication, roles, API tokens, worker trust, the demo boundary, a threat-model summary and the reporting policy.
order: 7
---

This page describes the design and says how much of it is built. It makes **no compliance or certification claims**, and a formal threat model has not been completed.

> [!NOTE] Built and tested: API tokens with scopes, worker credentials, role checks and viewer masking on the REST API, fencing and database privilege separation, and (milestones M5 and M6) GitHub sign-in, server-held browser sessions, CSRF protection and the Next.js server that signs service assertions. Run so far only against a mock OAuth server on loopback HTTP: **real GitHub, TLS and production mode (`__Host-` secure cookies) have not been run**, and the fixes for the first independent review of the web layer are pending re-review.

## Roles

There are two roles:

- **Operator**: can submit jobs, cancel and retry them, drain workers and manage credentials.
- **Viewer**: read-only. Sensitive fields are returned as `null` and listed in `masked_fields`. They are never partly redacted.

Authorization is decided by the control plane on **every** request. The web interface may hide a control, but it is never the enforcement point. A request with no established identity gets 401; an established identity that is not permitted gets 403.

| Field | Operator | Viewer |
| --- | --- | --- |
| Job `payload`, `result`, `error_message` | Visible | Masked |
| Attempt `error_message`, `log_tail` | Visible | Masked |
| Worker `hostname` | Visible | Masked |
| Credential secrets and hashes | Never returned after creation | Never |

## Human sign-in (built, M6)

Sign-in is GitHub OAuth (authorization code with PKCE and a `state` bound to the browser that started it) plus an explicit allowlist. A GitHub account that is not on the allowlist, or has been disabled, gets no session. A person is either an operator or a viewer. No default accounts exist and none will.

- **The browser never holds an API token or worker credential.** It holds one sealed (AES-256-GCM), `HttpOnly`, `SameSite=Lax` cookie, which carries the session id, user id, login, a role hint and the expiry. Over HTTPS the cookie is `Secure` and has the `__Host-` prefix. The role shown in the console always comes from the control plane, never from the cookie.
- **The session itself lives in the control plane**, so it can be revoked. Signing out revokes it there. Signing in again revokes the earlier session of the same browser. A session has an idle and an absolute expiry, and disabling the account ends it on the next call.
- **Mutations need two checks:** the request `Origin` must equal the console's own origin, and the `X-CSRF-Token` header must match the session. The same Origin rule guards the live-update WebSocket.
- **The Next.js server signs a short-lived assertion** (EdDSA, 60 seconds at most, a fixed set of claims, a fresh id per request) for every call to the control plane. The control plane verifies the signature and then re-reads the user, role and session from the database. The Next.js server decides nothing about authorization.
- **The server-side proxy is an allowlist** of the API routes the console uses. Session operations, the stream and anything else answer 404. Request bodies are capped at 1 MiB while they are read.
- **The console and sign-in pages use a per-request nonce Content-Security-Policy** without `'unsafe-inline'` for scripts. The public pages keep a static policy.
- **Links from public pages into the console or sign-in page load a new document**, so those pages never run inside a public page's document and its looser policy. Every path under `/app`, `/login` and `/auth` gets the nonce policy, in any letter case.
- **The role in the session cookie is a hint.** The control plane authorizes from the database role on every request. After a role change the Next.js server reads `GET /me` and re-seals the cookie, so the control plane stops recording a role-mismatch security event for every request after the first few that follow the change.
- **The stream relay caps browser sockets** at 5 per session, 15 per user and 256 per process, and frees a socket as soon as the client closes or half-closes it.
- **Rotating the web session key** is supported with an optional previous key during a window. The signing key for assertions is read once at start, so rotating it needs a restart of the web server.

> [!UNVERIFIED] Not run: real GitHub, TLS between browser, web server and control plane, production mode with Secure `__Host-` cookies in a browser, idle and absolute expiry by elapsed time, Firefox and WebKit.

## API tokens (built)

Direct clients send `Authorization: Bearer cxt_<prefix>_<secret>`. Tokens are scoped (`jobs:read`, `jobs:write`, `workers:read`, `workers:write`, `metrics:read`, `audit:read`), revocable and expiring. Only a SHA-256 hash of a 256-bit random secret is stored; the secret is shown once, at creation.

A token **never outranks its owner**: its effective authority is its scopes intersected with what the owner's *current* role allows, evaluated on every request. Demote the owner to viewer and the token loses every write scope at once. A disabled owner makes the token answer 401. Tokens cannot create or revoke tokens or worker credentials.

## Worker trust (built)

- Workers authenticate to the control plane over gRPC with a `cxw_` credential, checked on **every** call. Credentials for claims, renewals and reports are never cached.
- A credential is bound to one worker. Each call also names the worker and its current incarnation, so a superseded process is refused.
- Workers have no database credentials and no REST access.
- Handlers are precompiled and allowlisted. There is no arbitrary code upload, shell, file path or URL-fetching job type in 1.0.
- Revoking a credential takes effect for the next claim, renewal or report. A revoked worker may still be running an external side effect: only database writes are fenced.
- **Draining is not exclusion.** A drain stops new claims for that worker row. A drained worker that a supervisor (`restart: always`, Kubernetes) restarts comes back as a new worker. To keep a host out, **revoke its worker credential**.
- Use TLS for any non-loopback worker connection. Plaintext is for a private local network only.

## The public demo

`/demo` runs entirely on fixture data bundled in the web application. It has no connection to a control plane, holds no credentials and has no write routes. The boundary is enforced three ways: a lint rule that forbids the demo's code from importing the live data provider, session code or network client; a test that walks the real import graph; and a browser test that fails if the demo makes any cross-origin request, any `/api/` or `/bff/` request, any non-GET request, any request with an `Authorization` header, or opens a WebSocket or sets a cookie. The viewer view in the demo removes sensitive fields from the data, not just from the screen.

## Threat-model summary

Boundaries and their main residual risks, from `docs/security/trust-boundaries.md`:

| Boundary | Protected by | Main residual risk |
| --- | --- | --- |
| Browser to web server | Sealed session cookie, Origin and CSRF checks, nonce Content-Security-Policy on the console | Script injection in the app could act as the user within a session (the first review found no injection sink) |
| Web server to control plane | Signed assertion, audience and expiry, then a database re-check; TLS required to a non-loopback control plane in production | Theft of the signing key, or of the web session key (it can forge a cookie for any live session id), lets an attacker impersonate an allowed user until it is rotated |
| API client to control plane | Hashed scoped token, owner's current role | A leaked token works until it is revoked |
| Worker to control plane | Per-call credential, worker identity, attempt fence | A revoked worker can still cause external side effects |
| Control plane to PostgreSQL | Least-privilege role, parameterized queries | A compromised database host is out of scope |
| Anonymous to demo | No route to the control plane | Fixture content must be labelled illustrative |

Other limits you should know:

- Secrets must not be placed in payloads, results or logs. They are stored and shown to operators.
- Rate limiting is implemented in the control plane: HTTP 429 `rate_limited` with `Retry-After`, per-user mutation limits and a bucket for failed authentications.
- An independent review of the web layer (REVIEW-M6-RV-01) found no critical or high issue and three medium ones (proxy path handling, body size cap, WebSocket relay sockets). A second round confirmed their fixes on the live stack and passed the M6 exit. It left release items: a public page could hand over to the console or sign-in page inside its own, looser policy; letter-case variants of `/app`, `/login` and `/auth` carried no policy; a role change left a stale role hint in the cookie; a half-closed stream client held a relay slot. All four are fixed in the M9 web hardening change and have **not had an independent re-review**. A formal STRIDE threat model, the production security and load gate (M8) and a dependency and supply-chain audit are **not done**.

## Reporting a vulnerability

> [!UNVERIFIED] There is no published security contact or disclosure policy yet. Choosing one is an owner decision that has not been made, and this page will not invent an address. Until a policy is published, do not post vulnerability details in a public issue.
