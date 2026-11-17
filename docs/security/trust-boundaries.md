# CLUSTERX — Trust boundaries

Status: **specified** (TASK-M1-BE-01). Decision record: ADR-0005. Master §4, §7. Feeds the STRIDE threat model (FR-SEC-09, later).

```
 [Browser] --HTTPS, session cookie--> [Next.js BFF] --HTTPS, signed assertion--> [Go coordinator] --TLS/local--> [PostgreSQL]
                                                                                       ^
 [Direct API client] ------------------HTTPS, cxt_ token-------------------------------|
 [Worker process(es)] -----------------gRPC+TLS, cxw_ credential-----------------------|
 [Prometheus] ------------------------internal listener, scrape token------------------|
 [Anonymous /demo visitor] --HTTPS--> [Next.js static/fixture pages]  (no edge to Go)
```

| # | Boundary | Crossing credential | Verified by | Threats addressed | Residual risk |
|---|---|---|---|---|---|
| B1 | Browser → BFF | `__Host-` session cookie (HttpOnly, Secure, SameSite=Lax) + CSRF token/Origin on mutations | BFF | CSRF, session fixation (regenerate on login), XSS token theft (no tokens in JS) | XSS in app can still act as user within session |
| B2 | BFF → Go | EdDSA JWS, `aud`, `exp ≤ 60 s`, `sid` | Go: signature, aud, exp, then DB user/role/session | forged identity headers, replay beyond 60 s, role escalation via assertion | BFF key compromise = impersonation of any allowlisted user until key rotation |
| B3 | API client → Go | `cxt_` token (hash lookup) | Go: hash, revoked, expired, role ∩ scope | token reuse after revoke, over-scoped tokens | leaked unrevoked token until revoked |
| B4 | Worker → Go | `cxw_` credential + worker_id + incarnation_id + attempt fence | Go per RPC, uncached for mutations | forged results, stale results, replay, revoked worker, superseded process | revoked worker may still run external side effects; only DB writes are fenced |
| B5 | Go → PostgreSQL | DB role (least privilege, no DDL at runtime) | PostgreSQL | SQL injection (parameterized pgx only), privilege misuse | DB superuser/host compromise out of scope |
| B6 | Anonymous → `/demo` | none | network isolation test (AC-14) | data exposure, write paths | fixture content must be labeled illustrative |
| B7 | Browser ↔ Go WebSocket (via BFF-issued assertion) | assertion + Origin allowlist | Go; closes at expiry/logout | cross-site WebSocket hijacking, post-logout reconnect | hints may arrive late/out of order; HTTP is authoritative |
| B8 | Prometheus → `/metrics` | scrape token on internal listener | Go | metric scraping by outsiders, label-cardinality DoS | none expected |

Rules:
- Go never trusts `X-Forwarded-User`, `X-User-Role` or similar; only B2/B3 credentials establish identity.
- No secrets in job payloads/results/logs; payloads are untrusted input validated by handler JSON schema.
- Handlers are precompiled and allowlisted; no shell, filesystem path or URL-fetch task types (§5, §7).
- Workers have no DB credentials and no REST credentials.
- Credentials are stored only as SHA-256 hashes of ≥ 256-bit random secrets (fast hash acceptable
  because secrets are high-entropy; not passwords). Shown once at issuance.
