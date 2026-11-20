# CLUSTERX — Traceability Matrix

Vocabulary (§0): `proposed → specified → approved → implemented → integrated → tested → verified → released`.
Unrun tests are **NOT RUN**. Nothing is **released**.

- Master: R2 (OD-01). Contract: OpenAPI `1.0.0-draft.7` (unchanged since `contract-m6-draft.7`), migrations 0001–0004.
- AC/MW table below = independent re-audit **AUDIT-M9-RV-03** on `27b0094` (verbatim). Later integrator changes
  are listed in the next section and are **not independently re-audited**.
- All evidence is from the dev laptop / Colima VM. GitHub CI ran on `ad645cd` (all jobs green) after one unexplained earlier `web` failure; see `docs/testing/release-gates.md`.

## Changes after AUDIT-M9-RV-03 (integrator + web lane; not re-audited)

| Finding | Change | Evidence | Effect on rows below |
|---|---|---|---|
| RV10-01 | README, quickstart, deployment page, `commands.ts`, release-gates: clean clone verified on Linux arm64 VM only | text; web test guards the wording | AC-26/AC-28 claim fixed |
| RV10-02 | `/docs/api` shows all 28 operations served; unit test parses `v1.go` and must match the spec; concepts/workers pages fixed | `pnpm --filter web test` (136) | AC-28 |
| RV10-03 | this file and `docs/testing/release-gates.md` reconciled | — | AC-30, MW-06 |
| RV10-04 | root CHANGELOG M9 entry | — | AC-28 |
| RV10-05 | `.dockerignore` `**/*.env` | — | AC-25 |
| RV10-06 | reliability wording | — | AC-28 |
| F-09 | client JS bundle size per route group measured (gzip: public 148, docs 152, demo 165, console 178 KB) | `docs/testing/evidence/web/bundle-size.md` | AC-24 (budget proposed, not frozen — OD-06) |
| F-10 | `docs/testing/test-matrix.md`, README screenshots, issue templates | files | AC-28 |
| AC-25 | full local scan: govulncheck ×5, pnpm audit, **OSV** (no issues), gitleaks history+tree, **SBOM** source 617 / control-plane 31 / worker 18 packages — all exit 0 | `docs/testing/evidence/m9/supply-chain.md` | AC-25 locally complete; CI NOT RUN |
| chart | throughput chart duplicate y-axis tick fixed + test | web unit tests | AC-23 |
| AUDIT-M9-RV-04 | Spot-check of all rows above: RV10-01..06 closed; AC-30/MW-06 satisfied locally; (a) team-closable blockers = **none** | `docs/coordination/reviews/AUDIT-M9-RV-04.md` | — |
| RV11-01/02 | security + changelog copy: OSV/SBOM run locally; "closed all but CI runner hardening (RV9-10)" | text; full E2E 213×3 green | AC-28 |
| RV11-03 | images rebuilt from main; image SBOMs show `x/net v0.62.0` | `docs/testing/evidence/m9/supply-chain.md` update | AC-25 |
| RV11-05 | README screenshot caption dates | text | AC-28 |
| RV11-04 | `/docs/api` "executed by" labels only cover the M4 curl guide — **open P3**, not a blocker | AUDIT-M9-RV-04 | AC-28 (minor) |

## Milestones

| Milestone | Status | Independent review |
|---|---|---|
| M0/M1 | approved (OD-01, OD-12) | REVIEW-M1-RV-01 PASS |
| M2 design | implemented | covered indirectly by axe in M3–M9 |
| M3–M5 | integrated, tested | REVIEW-M5-RV-01 PASS |
| M6 console | integrated, tested (mock OAuth) | REVIEW-M6-RV-01 PASS |
| M7 public site, M8 operations, M9 web hardening | integrated, tested | REVIEW-M9-RV-02; AUDIT-M9-RV-03 |
| M9 release | NO-GO pending human + owner gates | AUDIT-M9-RV-01, AUDIT-M9-RV-03 |

## Acceptance and workflow criteria (AUDIT-M9-RV-03, `27b0094`)

| ID | Status | Evidence (path / command) | Gap |
|---|---|---|---|
| AC-01 | verified | `TestAC01_AckOnlyAfterCommitAndRestart` (I*). Compose restart drill (AUDIT-M9 A8) | `synchronous_commit` image default not re-checked |
| AC-02 | verified | `TestAC02_*` under `-race` (I*) | — |
| AC-03 | verified | `TestAC03_*` (I*) | Not shown in the compose demo (north-star) |
| AC-04 | verified | `TestAC04_*` (I*). AUDIT-M9 A7 container SIGKILL. M8 bench kill | Δ crash-vs-deadline labelling is now documented (F-12 closed) |
| AC-05 | verified | `TestAC05_BudgetExactlyThree` (I*) | — |
| AC-06 | verified | `TestAC06_*` (I*) | — |
| AC-07 | verified | `TestAC07_Idempotency` (I*). AUDIT-M9 A6 | Expiry only in the Go test |
| AC-08 | Δ verified (local); CI guard verified | R6: `CI=1` without initdb gives **FAIL** with the guard message (exit 1). Without `CI` it gives SKIP (exit 0). `CI=1` with PG 18.7 first on PATH gives **PASS** 11.5 s under `-race` | CI itself NOT RUN (OD-11/20). RV9-10 runner risks unproven |
| AC-09 | verified | `TestAC09_*`, `TestRV5_*` (I*) | — |
| AC-10 | verified | `TestAC10_PriorityDueUnschedulable` (I*) | — |
| AC-11 | verified | `TestAC11_*` (I*) | — |
| AC-12 | tested-partial | 413 at 64 KiB, SDK result cap, container limits. Δ `/metrics` in-flight bound (R7 `TestMetricsBoundsConcurrentScrapes` PASS), Caddy 405 and 1 MB cap | No fuzz or adversarial suite (F-21) |
| AC-13 | tested-partial | `TestAC13_RouteMatrix`, M5 tests (I*). Console E2E auth/CSRF/CSP/role in 3 engines (I*, mock OAuth). Δ RV6B independently reviewed (REVIEW-M9-RV-02) | Real GitHub OAuth NOT RUN (OD-03) |
| AC-14 | verified | `demo-isolation.spec.ts` in 3 engines (I*) | — |
| AC-15 | verified (mock OAuth) | `console-jobs/workers/states/tokens` in 3 engines (I*) | Real IdP (OD-03). Workers list capped at 100 rows, with no pagination (console-budgets.md) |
| AC-16 | verified | `console-stream*` in 3 engines (I*). `TestM5_Hub*`, `TestM5_StreamSessionExpiry` (I*) | — |
| AC-17 | tested-partial | `TestUpgradeDrill0002To0004` (I*). M8 drill (RPO 2.2 s, RTO 14.1 s). Δ RV9-09 drill hygiene by code read only | Drill not re-run since RV9-09. Dev VM only. RPO/RTO not frozen (OD-06) |
| AC-18 | verified (in-process) / tested-partial (compose) | `TestAC18_ThousandJobsThreeWorkers` (I*). M8 compose bench | Compose bench not re-run |
| AC-19 | tested-partial | `docs/testing/benchmarks.md` | Dev VM, not reference host (OD-04). Not frozen (OD-06) |
| AC-20 | tested-partial | contract lint, `sdk:check`, proto regen (AUDIT-M9 G4–G6; contract unchanged since). Go protocol/SDK tests (I*). Δ `ci.yml` installs PG 18, Go and 3 browsers | **"in CI" NOT RUN** (OD-11/20). RV9-10 |
| AC-21 | Δ tested-partial | `routes.spec`, `site.spec` link checker, keyboard paths in Chromium, Firefox and WebKit (I*). R1: 35-route build table, no `/app/workflows*` | Manual keyboard walkthrough NOT RUN (F-07). `/docs/api` mislabels 19 routed operations (RV10-02) |
| AC-22 | FAIL (needs-human) | axe 0 serious/critical × 116 scans × 3 engines (`axe-m6.json`, `axe-firefox.json`, `axe-webkit.json`). Reduced-motion and 320 px reflow specs PASS (I*) | Manual NVDA/VoiceOver, keyboard-only, 200/400 % zoom, forced-colors, text-spacing NOT RUN (F-07) |
| AC-23 | tested-partial | Screenshots `docs/design/screenshots/{m6,m7}`. Overflow at 320/390/768/1024/1440/1920 in 3 engines (I*). Δ Firefox UUID-wrap fix | Screenshots not human-approved. 360 px missing (F-18). Real Safari/Edge not tested |
| AC-24 | Δ tested-partial | R1: public routes `○` (static), identical to `next-build-route-table.txt`. `static-public.spec.ts` (I*). `lighthouse-m9.md` (lab: perf 99–100, LCP 0.2–1.1 s). `console-budgets.md` (p95 measured in 3 engines, proposals) | Console JS bundle size NOT measured. Budgets not frozen (OD-06). No field data (not deployed) |
| AC-25 | Δ tested-partial | R2: gitleaks 8.30.1 history **0**, tree **0**, `scan.sh` exit 0. R3: build-context probe, every secret/e2e path excluded. govulncheck ×5 and pnpm audit 0 (AUDIT-M9 G10; HANDOFF-M8). Threat model, ASVS map. REVIEW-M9-RV-02 no P0/P1 | OSV and SBOM NOT RUN (tools not installed). govulncheck not re-run on `27b0094`. Security workflow never ran (OD-11/20) |
| AC-26 | tested-partial | Clean clone on Linux arm64 VM (AUDIT-M9 A4–A9). README `$HOME` note (F-11) | amd64 NOT RUN (OD-04). **The macOS clean-clone claim is not backed by any run** (RV10-01). Console not in the path (OD-03) |
| AC-27 | blocked-by-owner (local proof Δ re-verified) | R4: `setup-local-proof.sh` 0, `verify-local-proof.sh` 0 in the Colima VM (`clusterx-rv10`). TLS 1.3 with the CA verified, job over gRPC+TLS succeeded, retention pass deleted a 31-day job (410), TRACE/PROPFIND 405, edge cannot reach 9090/9091, Caddy nonroot, only Caddy publishes, no secret values in env, Prometheus targets up with 10 rules. Alert and backup/restore drills not re-run | Reference amd64 host, public DNS/ACME, firewall (OD-04). Real OAuth (OD-03) |
| AC-28 | Δ FAIL | README links all resolve. CHANGELOG/web changelog/deployment/reliability/security/quickstart/home/architecture largely match evidence (RV9-03 fixed). Numbers cross-checked against `benchmarks.md` and `drill.txt` | `/docs/api`, `/docs/concepts`, `/docs/workers` claim shipped M5 features are not served or not implemented, and tests pin it (RV10-02). Unbacked macOS clean-clone claim (RV10-01). No CI results (OD-11/20). No README screenshots/video (F-10) |
| AC-29 | verified | R1 route table: no `/app/workflows*`. Public-copy grep: no workflow/DAG/HA claim except "not in 1.0" | — |
| AC-30 | Δ FAIL | `docs/product/traceability.md` folds AUDIT-M9-RV-01 (F-01 done) | Stale since FE-02/BE-02/RV-02: it still says AC-24 FAIL (F-05), AC-25 gitleaks red, AC-08 silent skip, F-19 unreviewed, Chromium only. `release-gates.md` likewise (RV10-03). `test-matrix.md` missing (F-10) |
| MW-01 | verified (partial) | `TASK-M9-FE-02`, `TASK-M9-BE-01`, `TASK-M9-BE-02`, one owner each | Integrator commits `bbc2d94`, `fefee3f`, `7a99502`, `27b0094` and the review tasks TASK-M9-RV-01/02/03 have no ticket file in the repo |
| MW-02 | verified | Contract `1.0.0-draft.7` unchanged (`c4c3eb0`, blob `4eb0f68`). Cross-lane E2E in 3 engines (I*) | — |
| MW-03 | tested-partial | Lanes in separate worktrees, merges `166b581`, `0b24002`. `27b0094` records a temporary ownership transfer for `apps/web/content` | No recorded independent review of integrator-owned shared files (`ci.yml`, `.dockerignore`, `scan.sh`) beyond this audit's spot checks |
| MW-04 | verified (as UNVERIFIED-documented) | Every M9 handoff states effort/Ultracode UNVERIFIED. OD-05 PARTIAL | Owner `/effort`, `/status` never captured |
| MW-05 | Δ verified | REVIEW-M9-RV-02 (separate reviewer) closed F-19. This audit is by a non-author. No self-approval found | — |
| MW-06 | Δ FAIL | traceability.md exists and is reconciled to AUDIT-M9-RV-01 | Not reconciled to `27b0094` (RV10-03). This file is the input |
| MW-07 | NOT RUN (n/a) | No token/agent-count records. None fabricated | — |
| MW-08 | verified | OD-11/OD-20 respected: no push, deploy or spend observed. Project `clusterx` untouched (R4 `docker compose ls` before and after) | — |

## Finding disposition summary (detail in AUDIT-M9-RV-03 §3)

| Finding | Disposition | | Finding | Disposition |
|---|---|---|---|---|
| F-01 | CLOSED (re-opened as RV10-03) | | RV9-01 | CLOSED |
| F-02 | CLOSED in config; run BLOCKED-BY-OWNER | | RV9-02 | CLOSED (see R4) |
| F-03 | CLOSED | | RV9-03 | CLOSED for listed items; RV10-01/02 new |
| F-04 | CLOSED | | RV9-04 | CLOSED (see R4) |
| F-05 | CLOSED | | RV9-05 | CLOSED |
| F-06 | CLOSED for listed pages; residue RV10-02 | | RV9-06 | CLOSED (see R4) |
| F-07 | OPEN, needs-human | | RV9-07 | CLOSED (see R4) |
| F-08 | CLOSED (3 engines); real Safari/Edge open | | RV9-08 | CLOSED |
| F-09 | PARTIAL (bundle size open; freeze OD-06) | | RV9-09 | CLOSED by code read; drill NOT RUN |
| F-10 | PARTIAL (test-matrix, README media open) | | RV9-10 | OPEN P3 (needs a real CI run) |
| F-11 | CLOSED | | RV9-11 | CLOSED |
| F-12 | CLOSED | | | |
| F-13 | ACCEPTED (reviewer ruling P3, 1.x) | | | |
| F-14 | OPEN P3 | | | |
| F-15 | OPEN P3 (fix at tag) | | | |
| F-16 | OPEN P3 | | | |
| F-17 | CLOSED | | | |
| F-18 | OPEN P3 | | | |
| F-19 | CLOSED (REVIEW-M9-RV-02) | | | |
| F-20 | CLOSED (docs); real console BLOCKED-BY-OWNER OD-03 | | | |
| F-21 | OPEN P3 | | | |

Note: MASTER §11 MW-01..MW-05 map to §1.4 as in AUDIT-M9-traceability.md (§11 MW-03 → MW-03 + MW-05, §11 MW-05 → MW-08).
