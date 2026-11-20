# CLUSTERX — Threat model (STRIDE)

Status: **M8 draft for review** (TASK-M8-BE-01; FR-SEC-08..10, AC-25). Scope: MASTER_PROMPT §4, §7; boundaries B1–B8 in
`trust-boundaries.md`; permissions in `permissions.md`; ADR-0003/0004/0005/0006. "Test" names are Go tests under
`apps/control-plane/**` or Playwright specs under `apps/web/e2e/**`; `verification.md` maps them to ASVS 5.0. Nothing
here claims a deployed host was assessed: there is none (OD-04).

Severity: **H** exploitable with significant impact if the mitigation failed; **M** limited impact or needs a prior
compromise; **L** defence in depth. Status: **M** mitigated and tested · **P** partly (gap stated) · **A** accepted residual
(owner sign-off needed for H) · **O** open.

## Assets

Job payloads/results/logs (may hold customer data); acknowledged-job durability; operator authority (submit/cancel/retry/
drain/credentials); API tokens (`cxt_`), worker credentials (`cxw_`), BFF signing key, session cookies, DB passwords,
metrics token, TLS keys; audit trail; availability of the coordinator and database.

## Components and boundaries

```
Browser ──B1 HTTPS cookie──▶ Caddy ─▶ Next.js BFF ──B2 TLS + EdDSA assertion──▶ Go control plane ──B5──▶ PostgreSQL
API client ──B3 HTTPS cxt_──▶ Caddy ───────────────────────────────────────────▶ Go (REST)
Worker ──B4 gRPC+TLS cxw_─────────────────────────────────────────────────────▶ Go (gRPC)
Prometheus ──B8 TLS scrape token──▶ Go internal :9091      Anonymous ──B6──▶ /demo (no edge to Go)
GitHub OAuth ◀── BFF (authorization code + PKCE)            CI/CD: GitHub Actions → images/SBOM (not published)
```

## STRIDE by component

### Browser ↔ BFF (B1, B7)

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-B1 | **S** Session theft via XSS | H | HttpOnly `__Host-` cookie, no OAuth/API tokens in browser JS, CSP (`script-src 'self' 'unsafe-inline'`), React escaping | `console-security.spec.ts`; ADR-0006 | **P** — CSP still allows inline scripts (Next.js bootstrap); nonce CSP is a web-lane follow-up |
| T-B2 | **S** Session fixation | M | session regenerated at login, sealed cookie, server-side session row | `TestM5_SessionsLifecycle`, `console-auth.spec.ts` | M |
| T-B3 | **T** CSRF on mutating BFF routes | H | CSRF token + exact Origin check on every mutation, SameSite=Lax | `console-security.spec.ts` | M |
| T-B4 | **S** Cross-site WebSocket hijacking | M | Origin allowlist (`CLUSTERX_STREAM_ALLOWED_ORIGINS`), short-lived assertion, closed at logout/expiry | `TestM5_StreamAuthHintsAndRevalidation`, `TestM5_StreamSessionExpiry`, `console-stream.spec.ts` | M |
| T-B5 | **I** Open redirect after login | M | login `next` restricted to same-origin `/app` paths | `bff-core.test.ts` (vitest), `console-auth.spec.ts` | M |
| T-B6 | **R** User denies an action | L | privileged actions audited with actor/session (`audit_events`) | `TestM5_AuditAndMetrics` | M |

### BFF → Go (B2)

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-F1 | **S** Forged identity headers | H | Go ignores forwarded identity headers; only EdDSA assertions with `aud`, `exp ≤ 60 s`, `sid`, strict JOSE (alg/typ/kid only, closed claim set) | `TestVerify`, `TestVerifyStrictJOSE`, `TestV1AuthMatrix` | M |
| T-F2 | **S** Assertion replay | M | `jti` replay cache for mutations, 60 s lifetime | `TestReplayCacheBoundedAndExpiring` | **A** — cache is in-memory: after a restart a captured assertion is replayable ≤ 65 s; reads never single-use (RV01-12) |
| T-F3 | **E** Role escalation via assertion | H | DB role wins over assertion claims; session re-checked per request | `TestAC13_RouteMatrix` | M |
| T-F4 | **I** Assertion/API traffic sniffed in transit | H | TLS BFF→Go in the production example (internal CA, `NODE_EXTRA_CA_CERTS`) | production proof (`deployment.md`) | **P** — verified locally with a self-signed CA only |
| T-F5 | **S** BFF signing-key compromise | H | key only in BFF container secret file; JWKS rotation by `kid` | ADR-0006 | **A** — compromise = impersonation until rotation; no HSM |

### Direct API (B3)

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-A1 | **E** BOLA/IDOR, viewer mutations, sensitive field reads | H | Go checks scopes ∩ owner's current role per operation and field; viewer masking | `TestAC13_RouteMatrix`, `TestV1ReadsMaskingPaginationExpiry`, `TestRV5_LogTailMaskAndAnyReadScope` | M |
| T-A2 | **S** Token guessing / credential stuffing | M | 256-bit secrets, SHA-256 lookup, per-peer auth-failure bucket (429) | `TestM5_RateLimit`, `TestRV5_AuthFailureBucketIsNotCrossUser` | **P** — behind Caddy all clients share one peer address for the failure bucket (see T-D3) |
| T-A3 | **T** Duplicate/conflicting submissions | M | scoped idempotency (principal+namespace+key, fingerprint), 409 on different body | `TestV1CreateValidationAndIdempotency` (AC-07) | M |
| T-A4 | **D** Oversized input, request floods | M | 64 KiB payload, 1 MB Caddy body cap, bounded pages/windows, per-principal mutation limit (20/s default) | `TestV1CreateValidationAndIdempotency`, `TestM5_RateLimit` | M |
| T-A5 | **I** Error/log injection, internal error leakage | M | typed `{code,message,request_id}` errors, structured logs, transport errors mapped to 503 | `TestPanicIsContractError`, `TestClassifyDBErrorConnectionLoss` | M |
| T-A6 | **R** Unaudited credential operations | M | token/credential create/revoke audited; denials audited with per-credential budget | `TestM5_TokensAndWorkerCredentials`, `TestRV5_DenialAuditRateLimited` | M |

### Worker gRPC (B4)

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-W1 | **S/T** Forged or stale results | H | per-RPC credential + worker id + incarnation + attempt + lease generation fence | `TestAC03_StaleSuccessAfterExpiryAndReclaim`, `TestAC09_GRPCIdentityMatrix` | M |
| T-W2 | **E** Revoked credential keeps working | H | credential re-checked on claim/renew/report and on Register | `TestRV5_RegisterRechecksRevokedCredential`, AC-09 matrix | M — handler side effects already started cannot be undone (§16) |
| T-W3 | **S** Credential sniffed on the network | H | production config refuses plaintext gRPC off loopback; worker refuses insecure in production | `config` tests; production proof | **P** — local self-signed CA proof only |
| T-W4 | **D** Credential guessing over gRPC | M | per-peer failure bucket → `RESOURCE_EXHAUSTED` | `TestWorkerAuthFailureLimit` | M |
| T-W5 | **E** Malicious payload escapes handler | H | precompiled allowlisted handlers, strict JSON schema, nonroot RO containers, pids/mem limits, no shell/URL task types | `catalog` tests, AC-12 tests, compose hardening | **P** — no egress filtering inside the workers network beyond `internal: true`; no sandbox (non-goal 1.0) |
| T-W6 | **D** Lease starvation by a stuck worker | M | lease TTL + recovery, hard deadline, drain grace | `TestAC04_*`, benchmarks w3-kill | M |

### PostgreSQL (B5)

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-D1 | **T** SQL injection | H | pgx parameterised queries only; dynamic SQL limited to fixed fragments (`sqlb`) | code review, tests | M |
| T-D2 | **E** App role misuse (DDL, TRUNCATE, history rewrite) | H | runtime role without DDL/TRUNCATE, delete/truncate guard triggers, production start-up privilege check | `TestNF04_PrivilegesAndDeleteGuards` | M |
| T-D3 | **I** Database exposed on the network | H | production example: no published port, `internal: true` network | `infra/production/compose.yaml`, proof | M (example); TLS to PostgreSQL inside the host: **O** (sslmode=disable on the private network) |
| T-D4 | **D** Disk exhaustion / table growth | M | retention job, bounded logs, `ClusterxDiskSpaceLow` alert | retention tests, alert drill | M |
| T-D5 | **I** Backup archive disclosure | H | archives 0600, checksum; contain hashes not secrets | `backup-restore.md` | **P** — encryption and off-host storage are operator responsibilities |

### Public demo (B6)

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-X1 | **I/T** Anonymous read/write of production data | H | demo uses bundled fixtures; no API URL or credentials in demo code paths | `demo-isolation.spec.ts` (AC-14) | M |
| T-X2 | **S** Demo content mistaken for live data | L | labelled illustrative | `demo-isolation.spec.ts` | M |

### OAuth provider (GitHub)

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-O1 | **S** Authorization-code interception / CSRF login | H | authorization code + PKCE + `state`, exact callback | `console-auth.spec.ts` (mock provider) | **P** — real GitHub app NOT exercised (owner approval) |
| T-O2 | **E** Any GitHub user becomes operator | H | allowlisted identities; first operator via CLI | ADR-0006 | M |
| T-O3 | **S** OAuth endpoint override in production | M | overrides refused when `CLUSTERX_ENV=production` | web config (web lane) | M |

### Coordinator availability

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-C1 | **D** Single coordinator crash | M | durable state in PostgreSQL, restart recovery, alerts | `TestAC01_AckOnlyAfterCommitAndRestart`, `TestAC08_DatabaseOutageMidRun` | **A** — no HA in 1.0 (§16) |
| T-C2 | **I** Metrics label cardinality / data leak | L | fixed-enum labels only, scrape token, internal listener | `TestMetricsRequiresBearer`, `TestOpsCollector` | M |

### CI/CD and supply chain

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-S1 | **T** Compromised third-party action | H | actions pinned to full SHAs, `contents: read` | `.github/workflows/*.yml` | M |
| T-S2 | **T** Malicious/vulnerable dependency | H | lockfiles (`go.sum`, `pnpm-lock.yaml`), govulncheck, OSV scan, weekly schedule | `security.yml`, `evidence/m8/security/` | M (findings: see `verification.md`) |
| T-S3 | **I** Secret committed | H | gitleaks over history + tree in CI | `evidence/m8/security/` | M |
| T-S4 | **T** Image substitution | H | base images pinned by digest; SBOM generated | Dockerfiles, compose files, SBOMs | **P** — images are not signed and no provenance is published (nothing is published yet) |

### Deployment (production example)

| # | Threat | Sev | Mitigation | Evidence | Status |
|---|---|---|---|---|---|
| T-P1 | **I** Secrets visible in `docker inspect`/env | M | secrets as files (`*_FILE`), 0700 directory | production proof (`deployment.md`) | M |
| T-P2 | **E** Container breakout impact | M | nonroot UIDs (Caddy too since M9-BE-02, via the unprivileged-port sysctl), read-only rootfs, `cap_drop: ALL`, `no-new-privileges`, mem/pids limits | `docker inspect` in proof | **P** — PostgreSQL image not read-only |
| T-P3 | **S** Misconfigured production start | H | control plane config fails closed (TLS, default passwords, token length) | config tests; proof "missing secret → exit 1" | M |
| T-P4 | **D** Host loss | H | backups + restore drill | `backup-restore.md` | **P** — no off-host automation, RPO = backup interval |

## Open items (owner / other lanes)

1. Nonce-based CSP for Next.js (web lane) — removes `'unsafe-inline'` (T-B1).
2. Native `*_FILE` secret support in the web config (web lane); the production image uses a wrapper meanwhile.
3. Real-client IP for rate limiting behind Caddy (trusted-proxy `X-Forwarded-For` handling) — backend, needs design (T-A2).
4. PostgreSQL TLS / SCRAM channel binding if the DB ever leaves the host-private network (T-D3).
5. Image signing + provenance before any publication (T-S4); owner approval required to publish.
6. Real GitHub OAuth app and reference host (OD-04) — everything above is verified locally only.
