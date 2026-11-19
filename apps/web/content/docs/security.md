---
title: Security
lead: Who can do what, how each boundary is protected, how workers are trusted and why the public demo cannot reach real data. No compliance is claimed.
description: Authentication and roles, API tokens, worker trust, demo separation, threat-model summary and reporting policy for CLUSTERX.
blurb: Authentication, roles, API tokens, worker trust, the demo boundary, a threat-model summary and the reporting policy.
order: 7
---

This page describes the design and says how much of it is built. It makes **no compliance or certification claims**, and a formal threat model has not been completed.

> [!NOTE] Built and tested today: API tokens with scopes, worker credentials, role checks and viewer masking on the REST API, fencing and database privilege separation. Specified but **not built**: GitHub sign-in, browser sessions, CSRF protection and the Next.js server that signs service assertions. Until those ship, the web console cannot sign anyone in.

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

## Human sign-in (specified, not built)

The design is GitHub OAuth with a server-held session and an explicit allowlist. A person is either an operator or a viewer. The browser holds a session cookie only and never an API token or worker credential. The Next.js server proxies to the control plane with a short-lived signed assertion (60 seconds at most) and re-checks nothing itself; the control plane verifies the signature and then re-reads the user, role and session from the database.

No default accounts exist and none will.

## API tokens (built)

Direct clients send `Authorization: Bearer cxt_<prefix>_<secret>`. Tokens are scoped (`jobs:read`, `jobs:write`, `workers:read`, `workers:write`, `metrics:read`, `audit:read`), revocable and expiring. Only a SHA-256 hash of a 256-bit random secret is stored; the secret is shown once, at creation.

A token **never outranks its owner**: its effective authority is its scopes intersected with what the owner's *current* role allows, evaluated on every request. Demote the owner to viewer and the token loses every write scope at once. A disabled owner makes the token answer 401. Tokens cannot create or revoke tokens or worker credentials.

## Worker trust (built)

- Workers authenticate to the control plane over gRPC with a `cxw_` credential, checked on **every** call. Credentials for claims, renewals and reports are never cached.
- A credential is bound to one worker. Each call also names the worker and its current incarnation, so a superseded process is refused.
- Workers have no database credentials and no REST access.
- Handlers are precompiled and allowlisted. There is no arbitrary code upload, shell, file path or URL-fetching job type in 1.0.
- Revoking a credential takes effect for the next claim, renewal or report. A revoked worker may still be running an external side effect: only database writes are fenced.
- Use TLS for any non-loopback worker connection. Plaintext is for a private local network only.

## The public demo

`/demo` runs entirely on fixture data bundled in the web application. It has no connection to a control plane, holds no credentials and has no write routes. The boundary is enforced three ways: a lint rule that forbids the demo's code from importing the live data provider, session code or network client; a test that walks the real import graph; and a browser test that fails if the demo makes any cross-origin request, any `/api/` or `/bff/` request, any non-GET request, any request with an `Authorization` header, or opens a WebSocket or sets a cookie. The viewer view in the demo removes sensitive fields from the data, not just from the screen.

## Threat-model summary

Boundaries and their main residual risks, from `docs/security/trust-boundaries.md`:

| Boundary | Protected by | Main residual risk |
| --- | --- | --- |
| Browser to web server | Session cookie, CSRF check (specified) | Script injection in the app could act as the user within a session |
| Web server to control plane | Signed assertion, audience and expiry, then a database re-check | Theft of the signing key lets an attacker impersonate any allowed user until it is rotated |
| API client to control plane | Hashed scoped token, owner's current role | A leaked token works until it is revoked |
| Worker to control plane | Per-call credential, worker identity, attempt fence | A revoked worker can still cause external side effects |
| Control plane to PostgreSQL | Least-privilege role, parameterized queries | A compromised database host is out of scope |
| Anonymous to demo | No route to the control plane | Fixture content must be labelled illustrative |

Other limits you should know:

- Secrets must not be placed in payloads, results or logs. They are stored and shown to operators.
- Rate limiting is specified (HTTP 429) but is not implemented in the current build.
- A formal STRIDE threat model, an independent security review of the browser-facing layer and a dependency and supply-chain audit are **not done**.

## Reporting a vulnerability

> [!UNVERIFIED] There is no published security contact or disclosure policy yet. Choosing one is an owner decision that has not been made, and this page will not invent an address. Until a policy is published, do not post vulnerability details in a public issue.
