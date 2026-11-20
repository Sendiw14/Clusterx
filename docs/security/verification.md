# CLUSTERX — Security verification (OWASP ASVS 5.0 mapping, AC-25)

Status: **M8 draft for review** (TASK-M8-BE-01). Maps the ASVS 5.0 chapters that apply to CLUSTERX to the tests and
evidence that exist today, and states the gaps. Mapping is at chapter/section level; it is **not** a claim of ASVS
compliance at any level, and nothing was assessed on a deployed host (OD-04). Threat IDs refer to `threat-model.md`;
the route/field matrix is `permissions.md` (consolidated access matrix, enforced in Go).

Legend: **Tested** = automated test in the repository asserts the control · **Drilled** = executed locally with recorded
evidence · **Doc** = documented/configured, no automated check · **Gap** = not done.

| ASVS 5.0 chapter | Applies to | Controls in CLUSTERX | Evidence | Status / gap |
|---|---|---|---|---|
| V1 Encoding and Sanitization | web, API, logs | React escaping; JSON-only API; structured (slog/JSON) logs; parameterised SQL (pgx) | `TestPanicIsContractError`; code review | Tested / Doc. No dedicated log-injection test |
| V2 Validation and Business Logic | API, workers | strict JSON schemas per handler, size limits (payload 64 KiB, result 128 KiB, log 16 KiB), state-machine guards in SQL triggers, idempotency, rate limits | `TestV1CreateValidationAndIdempotency`, catalog tests, `TestNF04_PrivilegesAndDeleteGuards`, AC-02/05/06/07 tests | Tested |
| V3 Web Frontend Security | console/BFF | CSP, `X-Frame-Options DENY`, HSTS, nosniff, COOP/CORP, `__Host-` cookies | `apps/web/next.config.ts`; `console-security.spec.ts` | Tested. **Gap:** CSP `script-src 'unsafe-inline'` (T-B1) |
| V4 API and Web Service | REST, gRPC, WebSocket | typed errors, 401/403/404/409/413/422/429/503 contract, request ids, Origin allowlist on WS, gRPC message size limits | `TestV1AuthMatrix`, `TestAC13_RouteMatrix`, `TestM5_Stream*`, `TestAuthAndLimits` | Tested |
| V5 File Handling | — | no file upload or path-based task types | design (§5, §7) | N/A |
| V6 Authentication | GitHub OAuth (humans), tokens, worker credentials | OAuth via BFF; 256-bit API/worker secrets stored as SHA-256; failure rate limits; revocation | `TestM5_TokensAndWorkerCredentials`, `TestRV5_AuthFailureBucketIsNotCrossUser`, `TestWorkerAuthFailureLimit`, `console-auth.spec.ts` | Tested (mock OAuth). **Gap:** real GitHub app not exercised |
| V7 Session Management | BFF sessions | server-side sessions, idle + absolute timeout, regeneration at login, logout revocation, WS closed on expiry | `TestM5_SessionsLifecycle`, `TestM5_StreamSessionExpiry`, `console-auth.spec.ts` | Tested |
| V8 Authorization | every API operation and field | Go-side scope ∩ role per operation, viewer field masking, object-level checks, DB role re-check | `TestAC13_RouteMatrix`, `TestV1ReadsMaskingPaginationExpiry`, `TestRV5_LogTailMaskAndAnyReadScope`, `TestHubHintsFilteredByScope` | Tested |
| V9 Self-contained Tokens | BFF assertions (EdDSA JWS) | strict JOSE (alg/typ/kid only), closed claims, `aud`, `exp ≤ 60 s`, `nbf`, `jti` replay cache, JWKS by `kid` | `TestVerify`, `TestVerifyStrictJOSE`, `TestReplayCacheBoundedAndExpiring`, `TestJWKS` | Tested. Accepted residual: replay cache in memory (T-F2) |
| V10 OAuth and OIDC | BFF as OAuth client | authorization code + PKCE + `state`, exact callback, no tokens in the browser, production refuses endpoint overrides | `console-auth.spec.ts` (forged callback), web config | Tested against a mock provider only |
| V11 Cryptography | secrets, signatures | crypto/rand secrets, SHA-256 hashes of high-entropy secrets, Ed25519 assertions, TLS ≥ 1.2 in Go | code; `auth` tests | Doc / Tested. No key rotation runbook beyond JWKS `kid` |
| V12 Secure Communication | edge, BFF→Go, workers, scrape | Caddy TLS edge; Go HTTP+gRPC TLS; production config refuses plaintext off loopback; worker refuses insecure in production | `config` tests; production proof (`deployment.md`, `evidence/m8/production/`) | Drilled locally (self-signed CA). **Gap:** PostgreSQL connection is plaintext on the private network |
| V13 Configuration | all services | fail-closed config (missing/invalid/default secrets ⇒ exit 1), secrets as files, pinned digests, nonroot/RO/cap_drop | `config` tests, `file_test.go`; proof "missing secret → exit 1" | Tested / Drilled |
| V14 Data Protection | payloads, results, backups | viewer masking, bounded retention (ADR-0004), log tail expiry, backups 0600 | retention tests, `backup-restore.md` | Tested / Doc. **Gap:** backup encryption is the operator's job |
| V15 Secure Coding and Architecture | repo, supply chain | dependency locks, pinned actions/images, govulncheck, OSV, gitleaks, SBOM | `.github/workflows/security.yml`, `evidence/m8/security/` | Drilled (results below). **Gap:** no image signing/provenance |
| V16 Security Logging and Error Handling | audit, logs | privileged actions and denials audited (budgeted), request ids, no secrets in errors | `TestM5_AuditAndMetrics`, `TestRV5_DenialAuditRateLimited` | Tested |
| V17 WebRTC | — | not used | — | N/A |

## Supply-chain scan results (local run, 2026-11-20)

Commands: `scripts/security/install-tools.sh` (pinned: govulncheck v1.9.0, gitleaks v8.30.1, osv-scanner v2.7.0,
syft v1.57.0, built with Go 1.27.4 from the Go module proxy) and `scripts/security/scan.sh <dir> <check>`. Raw reports:
`docs/testing/evidence/m8/security/`. CI: `.github/workflows/security.yml` (PR, main, weekly), integrator review required.

| Check | Result | Disposition |
|---|---|---|
| govulncheck, 5 Go modules (before) | 0 vulnerabilities reachable; 2 in required module `golang.org/x/net@v0.60.0` (GO-2026-6860, GO-2026-6866), not called | fixed anyway: bumped to v0.62.0 (commit `0f6865d`), race suite green (`before-xnet-bump/`) |
| govulncheck, 5 Go modules (after) | **No vulnerabilities found** (all 5) | — |
| `pnpm audit --audit-level moderate` (pnpm 9.15.9, 560 dependencies) | **0** info/low/moderate/high/critical | — |
| gitleaks history (129 commits) + working tree | 3 findings (history) / 1 (tree), all `generic-api-key` false positives: the RFC 6455 sample `Sec-WebSocket-Key` in two web tests and a fixed test-only metrics token in `config_test.go` (`before-ignore/`, redacted) | fingerprints listed in `.gitleaksignore`; rerun: 0 findings, exit 0 |
| osv-scanner (pnpm + Go lockfiles) | **NOT RUN**: building v2.7.0 failed on module-proxy network errors (`tools-install` DNS failures / connection resets) | covered for npm by `pnpm audit` and for Go by govulncheck; CI job runs it |
| syft SBOM (source + images) | **NOT RUN** locally for the same reason | CI job generates SPDX SBOMs of the control-plane and worker images |

No critical or high finding is open. No exception is requested.
