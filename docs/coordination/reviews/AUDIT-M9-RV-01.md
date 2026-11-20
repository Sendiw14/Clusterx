# AUDIT-M9-RV-01: Independent pre-1.0 release audit (MASTER §15)

- **Ticket:** TASK-M9-RV-01 (MASTER §14 M9 gate, §15, §16). Read-only independent auditor. I authored none of the audited code.
- **Auditor:** `claude-opus-5-5`, subagent of the integrator session. Effective effort and Ultracode were not observable from inside the subagent (UNVERIFIED, MW-04).
- **Target:** detached worktree `.worktrees/review` at main HEAD `8fa9e09`, plus a fresh `git clone` of the local main repository (same commit). Contract `api/openapi.yaml` `1.0.0-draft.7`. I did not change it.
- **Authority:** this is a reviewer recommendation. **It is not a release authorization.** The human owner decides GO or NO-GO (MASTER §1.4, Appendix A "Release gate", MW-05).
- **Sources read:** MASTER_PROMPT R2 (all of it). `requirements.md`, `traceability.md`, `OWNER_DECISIONS.md`, `session-log.md`. REVIEW-M1/M5/M6-RV-01. HANDOFF-M8-BE-01, HANDOFF-M9-FE-01 (in full). The other handoffs were skimmed for commands. README, SECURITY, CHANGELOG. `docs/testing/*`, `docs/operations/*`, `docs/security/*`. Also `docs/design/evidence/*`, `docs/design/motion-and-assets.md`, `.github/workflows/*`, `scripts/security/scan.sh` and the code named in the findings.
- **Scratch:** every log below sits under `<S>` = `/private/tmp/claude-501/-Users-sendiwkithnada-Documents-Clusterx/d0be9e8c-d590-4212-a424-878a0768bde1/scratchpad/audit/`. The scratchpad is ephemeral, so the important lines are quoted here.

## 1. Commands and results

Host: macOS arm64 (Apple M4). Go 1.27.4. PostgreSQL 18.7 (Homebrew). Node 26.13.1 and pnpm 9.15.9. Colima VM `default` (aarch64, 4 vCPU, 6 GiB, Ubuntu kernel 6.8.0-117, Docker 29.x, Compose v5.1.4 in the VM and 5.7.2 on the host).

### 1a. Clean-checkout reproduction (AC-26)

| # | Command (literal README "Local quickstart" unless noted) | Exit | Result |
|---|---|---|---|
| A1 | `git clone /Users/sendiwkithnada/Documents/Clusterx <S>/clone` (host) | 0 | HEAD `8fa9e09` |
| A2 | `cp .env.example .env`, fill 3 secrets with `openssl rand -hex 24`, set `CLUSTERX_{PG,HTTP,METRICS}_PORT=55580/18580/19591`, `CLUSTERX_IMAGE_PREFIX=clusterx-audit`, `COMPOSE_PROJECT_NAME=clusterx-audit` | 0 | `docker compose config` resolves `name: clusterx-audit` from the env file alone |
| A3 | `./infra/compose/bootstrap.sh` **in the host clone under `/private/tmp/...`** | **1** | `invalid mount config for type "bind": bind source path does not exist: <S>/clone/infra/compose/.secrets/database_url`. Colima mounts only `$HOME` (`colima ssh -- ls /private/tmp` gives "No such file"). Finding F-11. Partial project removed with `down -v` |
| A4 | Same flow **inside the Colima Linux VM**: `git clone` the host repo into VM `/tmp/clusterx-audit/clone` (ephemeral, not the host filesystem), same `.env` edits, `./infra/compose/bootstrap.sh` | **0** | 342 s (cold BuildKit cache in the VM). Final line `Ready: http://127.0.0.1:18580/health/ready`. Log `<S>/vm-bootstrap.log` |
| A5 | `curl /health/ready` | 0 | `{"status":"ready"}` 200 |
| A6 | README submit (`hash_sha256`, `Idempotency-Key: first-job`, port 18580) | 0 | **201**, `status:queued`. The same key again gives **200**. Same key with a different body gives **409** `idempotency_conflict`. After 3 s: `succeeded`, sha256 of "hello" correct. `<S>/vm-drill1.log` |
| A7 | 12 × `sleep_ms {ms:20000}`, then 4 active attempts on each of 3 workers, then `docker compose kill -s SIGKILL worker-2` at 09:45:42Z | 0 | worker-2's 4 attempts became **`lease_expired` at 09:46:10Z** (28 s after the kill: lease renewed at half life, 30 s TTL). The jobs were promoted, re-claimed on worker-3, and **12/12 succeeded by 09:46:32Z**. Events: admitted 12, claimed 16, lease_expired 4, promoted 4, succeeded 12. Invariant queries (running without active attempt; active attempt on a non-running job): **0, 0**. API shows worker-2 `live:false`. `<S>/vm-drill2.log`, `<S>/vm-drill3.log` |
| A8 | Submit a delayed job (`run_after` +25 s, priority 9). Then `docker compose restart control-plane`, then restart worker-2 | 0 | Job `scheduled` at 201. Ready again after 1 s. Job ended **`succeeded`, attempts 1, `terminal_at >= run_after`**. worker-2 re-registered `live:true`. No-token `GET /api/v1/jobs` gives 401 and `/metrics` without token gives 401. Only loopback ports are published (gRPC 9090 is not). `<S>/vm-drill4.log` |
| A9 | README stop: `docker compose -f infra/compose/compose.yaml --env-file .env --profile workers down -v`, then `docker image rm clusterx-audit/{control-plane,worker}:local`, then `rm -rf` of the VM dir | 0 | Volume and network removed, 2 images deleted. `docker compose ls` before and after: only `clusterx running(5)` (the integrator's project, never touched) |

### 1b. Gates re-run on the review worktree

| # | Command | Exit | Result |
|---|---|---|---|
| G1 | `go vet ./...` in all 5 modules (`go.work`) | 0 ×5 | `<S>/go-gates.log` |
| G2 | `CLUSTERX_PG_ADMIN_DSN=postgres://clusterx@localhost:55434/postgres?sslmode=disable go test -race -count=1 ./...` in all 5 modules. Own PG 18.7 cluster on 55434 (`-k ''`, `LC_ALL=en_US.UTF-8`, `max_connections=300`), PG 18 bin on PATH | 0 ×5 | control-plane: all 12 packages `ok` (chaos 31.4 s, e2e 18.7 s, store 11.8 s). worker, sdk-go, examples `ok`. proto has no tests |
| G3 | `go test -count=1 -v ./...` (control-plane, sdk-go) to count skips | 0 | control-plane **162 PASS / 0 FAIL / 1 SKIP** (`TestExplainAtScale`, opt-in). sdk-go 18/0/0. TestAC01/04/08/18 and TestUpgradeDrill0002To0004 ran and passed (not skipped) |
| G4 | `pnpm contract:lint` (in a `git archive 8fa9e09` copy, `pnpm install --frozen-lockfile --offline`) | 0 | 5 warnings (same as the M1 baseline) |
| G5 | `pnpm sdk:check` | 0 | generated `schema.d.ts` has no diff |
| G6 | `./packages/proto/generate.sh && git diff --exit-code packages/proto/gen` (protoc 29.3) | 0 / 0 | proto codegen reproducible |
| G7 | `pnpm --filter web lint` / `typecheck` / `test` / `build` | 0/0/0/0 | Vitest 127/127. **Build: every public page is `ƒ` (dynamic)**. Finding F-05 |
| G8 | `CLUSTERX_E2E_PG_DIR=<S>/auditpg pnpm --filter web e2e` (ports 55440/18180/19190/19191/19300/3100 free before and after) | 0 | **211 passed, 54 skipped** (opt-in screenshot specs), 1.5 min, Chromium only. Committed axe evidence regenerated with no diff. `<S>/web-e2e.log` |
| G9 | `scripts/security/scan.sh <S>/sec gitleaks pnpm` | **1** | gitleaks history **0**, **gitleaks tree exit 1 (2 findings)**, pnpm audit 0. Finding F-04 |
| G10 | `govulncheck@v1.9.0 ./...` (GOWORK=off) in all 5 modules | 0 ×5 | "No vulnerabilities found." ×5 |
| G11 | osv-scanner, syft SBOM, promtool | **NOT RUN** | not installed; not attempted |
| G12 | GitHub Actions `ci.yml` / `security.yml` | **NOT RUN** | **Never executed on GitHub.** The repository has not been pushed (OD-11/OD-20). Findings F-02/F-03 show `ci.yml` would not pass as written |

**Not re-run by me (taken from committed M8 evidence and only spot-read):** the 1/3/5-worker benchmarks (AC-19), the scripted 1,000-job bench (AC-18; the in-process `TestAC18_ThousandJobsThreeWorkers` did run in G2), the backup/restore drill (AC-17), the alert drills, and the production TLS proof (AC-27).

**Cleanup:** PG on 55434 stopped (`pg_ctl stop`). The E2E harness stopped its own processes and ports were verified free. The audit compose project and images were removed. The VM scratch dir was deleted. Generated `.env`/`.secrets*` in `<S>/clone` were deleted. The `clusterx` project was only read with `docker compose ls`. `git status` of the review worktree is clean.

## 2. Verdicts per §15 area

| Area | Verdict | Basis |
|---|---|---|
| Product fit | **PASS with gaps** | A real Go control plane, PG authority, pull workers and SDK. Reproduced end to end from a clean clone (A4–A8). North-star gaps: the console is not in the clean-checkout path (OAuth app needed, OD-03), and the stale-completion rejection is shown only by tests, not by the compose demo |
| Routes / controls | **PASS (automated), manual NOT RUN** | 24 routes built, no `/app/workflows*`. E2E link checker, console flows, 404/403/expired states all green in G8. Chromium only |
| Worker execution | **PASS** | A7 real SIGKILL recovery. TestAC04 (`kill -9`), TestAC11 (SIGTERM drain) pass |
| Concurrency invariants | **PASS** | TestAC02/03/06/09, NF01 deadlock tests and RV proofs pass under `-race` (G2). Code re-read of `RecoverExpired`/report fence: single tx, CAS on `current_attempt_id`, row-count checks. One classification nuance: F-12 |
| Data lifecycle | **PASS (local)** | Retention, NF-03/04 tests and upgrade drill pass (G2). Backup/restore drill evidence is M8 and was not re-run. RPO/RTO not frozen (OD-06) |
| Permissions | **PASS (automated)** | TestAC13_RouteMatrix, M5 tests, console E2E authorized/denied paths. Real GitHub OAuth NOT RUN (OD-03) |
| Trust boundaries (browser/BFF/API/gRPC) | **PASS with P3** | M6 review rounds 1–2 PASS. The M9 RV6B fixes pass their specs in my G8 run. I read `csp.ts`/`proxy.ts`. **No full independent code re-review of RV6B-01..05** (F-19). Per-peer auth bucket is P3 (F-13) |
| Demo isolation | **PASS** | `demo-isolation.spec.ts` (network trap) green. `/demo` is labelled fixture data |
| Accessibility evidence | **FAIL for release** | axe 0 serious/critical. Reduced-motion and 320 px overflow E2E green. **Manual NVDA/VoiceOver and keyboard walkthroughs NOT RUN**. Firefox/Safari/Edge NOT RUN (F-07, F-08) |
| Performance evidence | **FAIL for release** | Lighthouse M7 is lab-only, 3 pages, taken when the pages were static. They are now dynamic (F-05). Console p95/staleness/bundle budgets were never measured (F-09). Backend numbers are from a dev VM and not frozen |
| Asset / font rights | **PASS (P3 doc)** | OFL texts ship next to the woff2 files with SHA-256s (`public/fonts/FONTS.md`). Marks are original. `motion-and-assets.md` rows are stale (F-16) |
| Responsiveness | **PASS (automated)** | Overflow checks at 320/390/768/1024/1440/1920. 360 is not in the list (P3). The screenshots are not human-approved (AC-23) |
| Motion / reduced motion | **PASS (automated)** | reduced-motion and no-scroll-hijack specs green |
| Deployment | **PARTIAL** | Local compose verified on Linux arm64 by this audit. Production example verified only locally (M8). Reference amd64 host blocked (OD-04) |
| Upgrade / recovery | **PASS (local)** | TestUpgradeDrill0002To0004 green. A8 coordinator restart OK. AC-08 chaos green |
| CI | **NOT RUN / defective** | Never run on GitHub. `ci.yml` web e2e would fail on Ubuntu (F-02). The go job would silently skip AC-08 (F-03). `security.yml` would fail on gitleaks tree (F-04) |
| Supply chain | **FAIL (gate red)** | govulncheck 0, pnpm audit 0, gitleaks history 0, gitleaks tree 1 (false positives not in the ignore list). OSV/SBOM NOT RUN. Actions pinned to SHAs, `contents: read` |
| Documentation | **FAIL** | traceability.md is stale since M1 while README points to it (F-01). The web `/changelog` and `/docs/deployment` contradict the root CHANGELOG and M8 (F-06). test-matrix and release-gates docs are missing (F-10) |
| License | **PASS** | Apache-2.0 LICENSE (OD-02). Font OFL texts present |
| GitHub clarity | **blocked-by-owner** | Not published (OD-20). Private vulnerability reporting enabled only at publication (OD-19). No issue templates (F-10) |
| Unnecessary complexity | **PASS** | One modular coordinator, PG as the only queue, no extra services. The proxy/CSP split is intricate but justified and tested. The root `not-found` per-request render has a global cost (F-05) |

## 3. Findings

Severity: P0 = critical/unsafe. P1 = release blocker. P2 = must fix or formally accept before 1.0. P3 = should fix.

| ID | Sev | file:line | Problem | Evidence | Fix | Blocks 1.0? |
|---|---|---|---|---|---|---|
| F-01 | P1 | `docs/product/traceability.md:1-47`; `README.md:7` | Traceability stops at M1 (lists AC-02/03/06/09 as "PoC level", "Other AC … NOT RUN", MW-02 "generated client NOT RUN"). README sends readers there for the current status. AC-30 and MW-06 cannot pass | File content; `AUDIT-M9-traceability.md` (this audit) has the current reconciliation | Integrator folds `AUDIT-M9-traceability.md` into traceability.md, with FR/NFR rows per milestone and evidence paths | **Yes** |
| F-02 | P1 | `apps/web/e2e/stack/stack.ts:18`; `.github/workflows/ci.yml:28-41` | CI `web` job runs `pnpm --filter web e2e`, but the harness defaults `PG_BIN` to `/opt/homebrew/opt/postgresql@18/bin`. The job neither installs PostgreSQL 18 nor sets `CLUSTERX_E2E_PG_BIN`. It also has no `setup-go`, yet the harness builds Go. The required E2E gate cannot pass on `ubuntu-24.04` | Code read. CI never ran (G12) | In the web job: install PG 18 (PGDG apt) and set `CLUSTERX_E2E_PG_BIN=/usr/lib/postgresql/18/bin`, add `actions/setup-go` (pinned SHA) with `go-version-file: go.work`. Prove with a first real CI run | **Yes** (AC-20, AC-28 "CI results") |
| F-03 | P2 | `apps/control-plane/internal/chaos/chaos_test.go:264-265`; `.github/workflows/ci.yml:45-67` | `TestAC08_DatabaseOutageMidRun` skips when `initdb` is not on PATH. The CI go job uses a PG service container and never puts PG 18 binaries on PATH, so CI would report `ok` while AC-08 never runs (same silent-skip pattern as RV5 notes) | Code read. Locally the test ran (G3) | Install PG 18 binaries in the go job and add them to PATH. Make the skip fail when `CI=true` | Yes (release gate needs race+chaos in CI) |
| F-04 | P2 | `.gitleaksignore:3-6`; `apps/web/e2e/console-stream-hardening.spec.ts:9`; `apps/web/src/__tests__/stream-relay.test.ts:96` | `scan.sh gitleaks` tree mode finds the RFC 6455 sample `Sec-WebSocket-Key` in 2 web test files. `.gitleaksignore` holds only *history* fingerprints for them (tree fingerprints exist only for `config_test.go`). `security.yml` would go red at HEAD. The M8 "rerun 0" predates the M9 merge | G9: `gitleaks tree exit=1`, `scan.sh exit=1`. `gitleaks dir -v` lists exactly those two files | Add `apps/web/e2e/console-stream-hardening.spec.ts:generic-api-key:9` and `apps/web/src/__tests__/stream-relay.test.ts:generic-api-key:96`, or use a `gitleaks:allow` comment. Re-run | **Yes** (AC-25 gate red) |
| F-05 | P2 | `apps/web/src/app/not-found.tsx:14`; `apps/web/next.config.ts:7`; HANDOFF-M9-FE-01 line 50 | `await connection()` in the **root** `not-found.tsx` (added in `52ca940`) makes every route dynamic. `/`, `/architecture`, `/docs/*`, `/demo`, `/changelog` are no longer prerendered. The handoff says "public pages static" and the `next.config.ts` comment says so too; both are false. Public pages are now SSR per request (TTFB, LCP, origin load), and the AC-24 Lighthouse evidence no longer describes the shipped build | G7 build table: all `ƒ`. **Probe:** with that one line commented out, `/`, `/architecture`, `/changelog`, `/demo`, `/docs/api`, `/docs/quickstart`, `/_not-found` become `○`. Restored after | Scope the per-request 404 to the nonce paths (e.g. a dynamic catch-all `notFound()` under `app/(console)` and `/login`/`/auth`) and keep the root 404 static. Add a build-output assertion that public routes are `○`. Re-measure Lighthouse | **Yes** (AC-24 evidence invalid; AC-28 claim) |
| F-06 | P2 | `apps/web/content/changelog.md:59-60`; `apps/web/content/docs/deployment.md:9,80-96`; `CHANGELOG.md:7-10` | Shipped web docs contradict the repo. `/changelog` says "M8 … not done" and "no independent re-review". `/docs/deployment` says backup, restore, TLS and alerts are untested or nonexistent. The root CHANGELOG lists M8 as merged. M9 handoff need #3 was not done at merge | File content at `8fa9e09` | Add M8/M9 to `content/changelog.md`. Update the deployment page with the M8 local-verified production proof, alerts and drill (marked local-only) | Yes (AC-28) |
| F-07 | P1 | `docs/design/evidence/axe-m7.md:4`; `docs/design/accessibility-plan.md:112-116` | No manual screen-reader (NVDA/VoiceOver), keyboard-only, 200/400 % zoom, forced-colors or text-spacing walkthrough exists. MASTER §10 forbids claiming WCAG without it, and AC-22 requires it | Every axe evidence file states "manual … NOT RUN" | Independent a11y reviewer runs scripts S1–S11 and records MANUAL-PASS/FAIL | **Yes** (AC-22) |
| F-08 | P2 | `apps/web/playwright.config.ts:23` | Only a Chromium project. MASTER §10 requires a defined browser matrix (Chrome/Firefox/Safari/Edge) before release | Config read. G8 ran Chromium only | Add firefox/webkit projects (at least the routes, a11y and auth specs) and write the supported-browser matrix | Yes (AC-21/22/23) |
| F-09 | P2 | `docs/design/evidence/lighthouse-m7.md:28-34`; `docs/testing/benchmarks.md:109-111` | Console budgets (p95 table filter/sort, route transition, topology interaction, live-update staleness, bundle size) were never measured. Public lab vitals cover 3 pages. Backend SLOs not frozen (OD-06) | Grep found no console budget evidence | Measure, then the owner freezes budgets and SLOs (OD-06) | Yes (AC-24, NFR-PERF-02/03) |
| F-10 | P2 | MASTER §12, §13; `docs/testing/`; `.github/` | `docs/testing/test-matrix.md` and `release-gates.md` are missing (only `benchmarks.md`). No issue templates. No README screenshots/video (M9 deliverable) | `ls docs/testing .github` | Add test matrix and release-gate record (this audit can seed it), issue templates and README media | Yes (M9 exit) |
| F-11 | P3 | `README.md:40-48` | On the README's own verified platform (Colima, default mounts), a checkout outside `$HOME` fails with an opaque bind-mount error | A3 | One README line: "with Colima, keep the checkout under `$HOME` (Colima mounts only `$HOME`)". Also update "Linux … not yet verified": this audit verified a clean clone on Linux arm64 (Ubuntu VM). amd64 is still open | No |
| F-12 | P3 | `apps/control-plane/internal/store/scheduler.go:45-60,72-76`; `infra/monitoring/alerts.yml:55`; `docs/architecture/state-machine.md:73,163-164` | M8 open item. When `timeout_seconds <= lease TTL` (e.g. `hash_sha256` 30 s = TTL), claim sets `lease_expires_at = LEAST(now+TTL, now+timeout) = hard_deadline_at`. A worker crash is then always recorded `deadline_exceeded`, and with an exhausted budget `failure_class=timeout`. **Rule (my ruling):** deadline-before-lease precedence is consistent with the report fence order (`lease.go:472-475`) and with §5 E8 (both events lead to the same transition), so it is acceptable. But (a) it is undocumented. (b) `ClusterxLeaseExpiriesRepeated` counts only `state="lease_expired"`, so crashes of short-timeout jobs never feed it. (c) Operators see "timeout" for a crash | M8 w3-kill: 3 `lease_expired` + 1 `deadline_exceeded`. Code read | Document in state-machine E8 and `/docs/reliability`: "if both have passed, `deadline_exceeded` wins; with timeout ≤ lease TTL a crash is classified as a deadline". Change the alert expr to `state=~"lease_expired\|deadline_exceeded"` (with a promtool test). Optionally refine: `deadline_exceeded` only when `hard_deadline_at < lease_expires_at` at last renewal | No |
| F-13 | P3 | `apps/control-plane/internal/api/ratelimit.go:96-101`; `internal/api/auth.go:85-96`; threat-model T-A2 | M8 open item. Behind Caddy all direct-API clients share one auth-failure bucket. **Severity ruling P3:** only failed credential guesses drain it, and a verifying credential is never refused (`authFailed` runs only after auth fails). So it cannot lock out valid users. It becomes a global guess throttle (stricter, not weaker). Effects: a misconfigured client gets 429 instead of 401 during an attack, and audit loses the real client IP. Logins (per GitHub id) and mutations (per principal) are unaffected | Code read | Optional trusted-proxy setting (`CLUSTERX_TRUSTED_PROXIES` CIDRs), using the right-most untrusted `X-Forwarded-For` hop only from those peers. Record client IP in security events | No |
| F-14 | P3 | `infra/production/web.Dockerfile:20-27`; `infra/production/web-entry.mjs`; `apps/web/src/server/config.ts:10,67` | M8 open item. The web config has no native `*_FILE` support; the wrapper covers it and is proven in the M8 local proof. The runtime image copies the whole build stage (dev deps, sources; 1.43 GB): a larger attack and SBOM surface | Code read | Native `*_FILE` in `config.ts` (same rules as the wrapper). `output: "standalone"` plus a pruned runtime stage (keep `server.ts` relay) | No |
| F-15 | P3 | `packages/sdk-go/worker.go:26` | SDK reports `sdk_version` `0.5.0-m5`, a milestone string, to every coordinator (seen in A6 `/api/v1/workers`) | A6 output | Set the release version (e.g. via `-ldflags` or bump at tag time) and document the compatibility matrix | No (fix at tag) |
| F-16 | P3 | `docs/design/motion-and-assets.md:81-82` | The font rows say "not committed; licence not re-verified", but the fonts and OFL texts are committed and checksummed in `public/fonts/FONTS.md` | File content | Update the rows | No |
| F-17 | P3 | `CHANGELOG.md:9,18` | "supply-chain scans in CI" and "CI" read as if CI runs. No workflow has ever executed (G12), and F-02 and F-04 show it would fail | G12 | Say "CI workflows defined (not yet run)" until a real run | No (part of AC-28 fix) |
| F-18 | P3 | `.nvmrc:1`; `apps/web/e2e/site.spec.ts:4` | `.nvmrc` is `26` (MASTER §3: pin the patch; local evidence uses 26.13.1). Overflow widths omit 360 (MASTER §10 list) | File content | `.nvmrc` → `26.13.1`. Add 360 to `WIDTHS` | No |
| F-19 | P2 | HANDOFF-M9-FE-01 "Needs" #1; `docs/coordination/reviews/` | No independent code re-review of RV6B-01..05 (nonce CSP partition, role re-seal, relay half-close/per-user cap, `/bff` matcher exclusion). No review exists for M7 (public site) or M8 (production example, security docs, scripts). My G8 run passes their specs and I read `csp.ts`/`proxy.ts`, but that is not the security re-review the handoff asks for | `ls docs/coordination/reviews` = M1, M5, M6 only | Short Opus critical re-review of RV6B and the M8 production example (TLS, secrets flow, fail-closed) | Yes (MW-05 / §7 security gate) |
| F-20 | P2 | MASTER §2 north-star, AC-26; `README.md:56-58` | The clean-checkout path starts no web console. The console needs a real GitHub OAuth app (OD-03), and §4 forbids a production-bypass login. AC-26 "API/docs" passes. The north-star "web console" part is reproducible only via the E2E harness with mock OAuth | A4–A8 | Owner decides OD-03. Meanwhile document the harness path as the local console demonstration (labelled mock OAuth) | Yes (owner-dependent) |
| F-21 | P3 | MASTER §11 AC-12; `apps/control-plane/internal/api/v1_test.go:179-180` | Bounds are tested (413 on payload > 64 KiB; SDK result cap; compose CPU/mem/pids/RO). There is no adversarial or fuzz suite (`func Fuzz` absent), so this was NOT RUN since M5 | grep | Add `FuzzCreateJob`/`FuzzSchemaValidate` (JSON depth, unicode, numbers) and run them for a bounded time in CI | No |

**No P0 found.** I found no correctness or security defect in the engine. The blockers are evidence, documentation and CI gaps, plus owner-dependent items.

## 4. M8 open items: rulings

1. **`deadline_exceeded` vs `lease_expired` when timeout == lease:** the behaviour is consistent with the fence precedence and §5, so it is accepted as a rule. Document it and fix the alert (F-12, P3).
2. **Per-peer auth bucket behind Caddy:** P3, not a release blocker. Valid credentials are never throttled. A trusted-proxy option is a 1.x improvement (F-13).
3. **Web `*_FILE` / image size:** P3. The wrapper is acceptable for 1.0 if documented. Native support and a standalone image are recommended (F-14).

## 5. Release readiness (§15 item 6)

**Recommendation: NO-GO for 1.0.** This is the reviewer's recommendation only. The human owner keeps release authority.

The engine is in good shape. Every Go race, chaos and integration test passes. The live web E2E passes 211/211. A clean clone on Linux arm64 boots, runs jobs, recovers a SIGKILLed worker within one lease TTL, and survives a coordinator restart. NO-GO rests on unmet release gates, not on engine defects.

**Failing or unmet gates (fixable by lanes):**
- AC-30 / MW-06: traceability stale (F-01).
- AC-20 / AC-28: CI never run, and `ci.yml` cannot pass as written (F-02, F-03).
- AC-25: supply-chain gate red on gitleaks tree (F-04). OSV and SBOM NOT RUN.
- AC-24: public pages accidentally dynamic, so the perf evidence is stale (F-05). Console budgets never measured (F-09).
- AC-28: web changelog and deployment docs stale, CI wording (F-06, F-17).
- AC-22: manual a11y walkthroughs NOT RUN (F-07). Browser matrix is Chromium only (F-08).
- M9 exit: missing test-matrix/release-gates docs, issue templates, README media (F-10). No independent re-review of RV6B and M8 (F-19).

**Owner-dependent blockers (§16):**
- OD-03: real GitHub OAuth app and first operator (AC-13 real-provider, AC-20 console path, F-20).
- OD-04: reference Linux amd64 host, domain and TLS (AC-26 amd64, AC-27).
- OD-06: freeze SLO/RPO/RTO and console budgets (AC-17/19/24).
- OD-11/OD-20: push and publication (CI evidence, private vulnerability reporting per OD-19, tags/releases).
- OD-05: model effort and Ultracode remain UNVERIFIED (MW-04, documented, not blocking by itself).

**Shortest path to a re-audit:** fix F-01..F-06, F-10, F-17. Get OD-11 approval for a push or private CI run, and obtain one green `ci` + `security` run on the exact commit. Run the manual a11y pass (F-07) and add a browser matrix. Re-measure Lighthouse after F-05, and measure the console budgets. Then the owner decides OD-03/04/06.
