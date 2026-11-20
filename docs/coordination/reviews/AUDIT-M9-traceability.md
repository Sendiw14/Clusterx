# AUDIT-M9 — AC / MW reconciliation (for `docs/product/traceability.md`)

Source: AUDIT-M9-RV-01, main `8fa9e09`, 2026-11-20. Independent auditor, read-only. Everything ran on the dev laptop or in its Colima VM, **not CI**. Status vocabulary:

- **verified**: the auditor re-ran it in this audit and it passed.
- **tested-partial**: the passing evidence does not cover the whole criterion, or comes from earlier milestones and was not re-run.
- **NOT RUN**: no evidence.
- **FAIL**: checked, and it does not meet the criterion.
- **blocked-by-owner**: needs an OD decision first.

`<S>` = session scratchpad `…/scratchpad/audit/` (ephemeral). G*/A* refer to the command tables in AUDIT-M9-RV-01 §1.

| ID | Status | Evidence (path / command) | Gap |
|---|---|---|---|
| AC-01 | verified | `TestAC01_AckOnlyAfterCommitAndRestart` PASS (G2/G3). Compose: 201 only after commit; delayed job survived `restart control-plane` and succeeded (A8) | Durability settings are image defaults; documented `synchronous_commit` not re-checked |
| AC-02 | verified | `TestAC02_ConcurrentClaimantsSingleJob/ManyJobs`, `TestAC02_TwentyFourGRPCClaimants`, `TestAC02_FailpointsLeaveNoPartialWrites`, `TestAC02_SchemaRejectsSecondActiveAttempt` PASS `-race` (G2) | — |
| AC-03 | verified | `TestAC03_StaleSuccessRejectedAfterExpiryAndReclaim`, `TestAC03_StaleReportVsRecoveryConcurrent` PASS (G2) | Not shown in the compose demo (north-star) |
| AC-04 | verified | `TestAC04_KillDashNineWorkerProcess`, `TestAC04_WorkerLostMidJobRecovers` PASS. Compose SIGKILL of worker-2: 4 attempts `lease_expired` after 28 s, retried, 12/12 succeeded, invariants 0/0 (A7, `<S>/vm-drill3.log`) | Crash classification when timeout ≤ lease TTL (F-12) |
| AC-05 | verified | `TestAC05_BudgetExactlyThree` PASS. Backoff/jitter tests in `internal/store/engine_test.go`, `internal/config/config_test.go` (G2) | — |
| AC-06 | verified | `TestAC06_ConcurrentCancelVsSuccess`, `_ForcedOrderings`, `_RESTCancelVersusSuccess`, `_CancelThenSuccess`, `_SuccessThenCancel` PASS (G2) | — |
| AC-07 | verified | `TestAC07_Idempotency` PASS. Live compose: same key 200, different body 409 `idempotency_conflict` (A6) | Expiry path is covered only by the Go test |
| AC-08 | verified (local) / FAIL in CI config | `TestAC08_DatabaseOutageMidRun` PASS with `initdb` on PATH (G3). M8 alert drill (PG stop → 503) | CI would silently skip it (F-03) |
| AC-09 | verified | `TestAC09_ClaimDenials`, `_FenceDenials`, `_ReportFenceDenials`, `_GRPCIdentityMatrix`, `TestRV5_RegisterRechecksRevokedCredential` PASS (G2) | — |
| AC-10 | verified | `TestAC10_PriorityDueUnschedulable` PASS. Delayed priority-9 job not run before `run_after` (A8) | — |
| AC-11 | verified | `TestAC11_RESTDrain`, `TestAC11_WorkerSIGTERM` PASS (G2). Compose `stop_grace_period: 40s` | — |
| AC-12 | tested-partial | 413 at 64 KiB (`internal/api/v1_test.go:179-180`), SDK result cap (`packages/sdk-go/sdk_test.go:95`), compose nonroot/RO/cap_drop/pids/mem. No shell/URL handlers exist | No fuzz or adversarial suite (F-21) |
| AC-13 | tested-partial | `TestAC13_RouteMatrix`, `TestM5_*` session/stream tests (G2). E2E `console-auth`, `console-security`, `console-session-hardening`, `console-csp*`, `console-role-refresh` PASS (G8, mock OAuth) | Real GitHub OAuth NOT RUN (OD-03). RV6B not independently re-reviewed (F-19) |
| AC-14 | verified | E2E `demo-isolation.spec.ts` PASS (G8) | — |
| AC-15 | verified (mock OAuth) | E2E `console-jobs`, `console-workers`, `console-states`, `console-tokens` PASS on the live Go+PG stack (G8) | Real IdP (OD-03) |
| AC-16 | verified | E2E `console-stream*.spec.ts` PASS (G8). `TestM5_HubOrderingOverflowAndGap`, `TestM5_StreamSessionExpiry` PASS (G2) | — |
| AC-17 | tested-partial | `TestUpgradeDrill0002To0004` PASS (G3). Backup/restore drill `docs/testing/evidence/m8/backup-restore/drill.txt` (M8, not re-run): RPO window 2.2 s, RTO 14.1 s | Dev VM only. Production volume NOT RUN. RPO/RTO not frozen (OD-06) |
| AC-18 | verified (in-process) / tested-partial (compose) | `TestAC18_ThousandJobsThreeWorkers` PASS (G3). M8 bench `docs/testing/evidence/m8/bench/` (1000 → 900/100, 1100 attempts, ×3) not re-run | Scripted compose bench not re-run by the auditor |
| AC-19 | tested-partial | `docs/testing/benchmarks.md` (1/3/5 workers ×3, p50/p95/p99, DB contention) | Dev Colima VM, not the reference host (OD-04). Not re-run. Not frozen (OD-06) |
| AC-20 | tested-partial | `pnpm contract:lint` 0, `pnpm sdk:check` 0, `packages/proto/generate.sh` no diff, Go protocol/SDK tests PASS (G2, G4–G6) | **"in CI" NOT RUN.** `ci.yml` web job broken (F-02) |
| AC-21 | tested-partial | E2E `routes.spec`, `site.spec` link checker, `console-*` keyboard paths PASS (G8). Build has 24 routes | Chromium only (F-08). Manual keyboard walkthrough NOT RUN |
| AC-22 | FAIL (NOT RUN manual) | axe 0 serious/critical (`docs/design/evidence/axe-m6.md`, `axe-m7.md`, regenerated identical in G8). Reduced-motion and 320 px reflow specs PASS | Manual NVDA/VoiceOver, 200/400 % zoom, forced-colors NOT RUN (F-07) |
| AC-23 | tested-partial | `docs/design/screenshots/{m6,m7}/*`, overflow at 320/390/768/1024/1440/1920. Topology list alternative tested | Screenshots not human-approved. 360 missing. Chromium only |
| AC-24 | FAIL | `docs/design/evidence/lighthouse-m7.md` (3 pages, lab, static build) | Public pages are now dynamic (F-05), so the evidence is stale. Console budgets NOT RUN (F-09) |
| AC-25 | FAIL | Threat model and ASVS map (`docs/security/*`). govulncheck ×5 clean (G10). pnpm audit 0. gitleaks history 0 | **gitleaks tree exit 1** (F-04). OSV and SBOM NOT RUN. CI never ran |
| AC-26 | tested-partial | **Clean clone on Linux arm64** (Colima Ubuntu VM): `bootstrap.sh` 0, README job 201, 3 live workers, SIGKILL recovery, coordinator restart, README stop `down -v` 0 (A4–A9) | macOS checkout outside `$HOME` fails (F-11). amd64 NOT RUN. Console not in the path (F-20, OD-03) |
| AC-27 | blocked-by-owner | M8 local production proof `docs/testing/evidence/m8/production/*` (TLS 1.3, internal CA, fail-closed, nonroot) and alert drills (not re-run) | Reference host, DNS/ACME, firewall (OD-04) |
| AC-28 | FAIL | README, CHANGELOG, web docs compared with code | traceability pointer (F-01), web changelog/deployment stale (F-06), CI wording (F-17), "public pages static" claim (F-05), no CI results |
| AC-29 | verified | Build route table has no `/app/workflows*`. E2E `site.spec` honesty/stale-copy specs PASS. Copy states workflows and HA are not in 1.0 | — |
| AC-30 | FAIL | `docs/product/traceability.md` stops at M1 | Fold this table and the FR/NFR rows in (F-01) |
| MW-01 | verified (partial) | `docs/coordination/tasks/TASK-M1…M9-*.md`, one owner each | Integrator commit `8fa9e09` (README/SECURITY/CHANGELOG) has no ticket. TASK-M9-RV-01 not in the repo |
| MW-02 | verified | Contract unchanged since `1.0.0-draft.7` (`ed2dc61`). `sdk:check` and proto regen give no diff. Cross-lane E2E PASS (G5, G6, G8) | — |
| MW-03 | tested-partial | Lanes merged via separate worktrees. HANDOFF-M8 flags root files (`.github/workflows/security.yml`, `.gitignore`, `.gitleaksignore`) for integrator review | No recorded integrator review of those shared files |
| MW-04 | verified (as UNVERIFIED-documented) | `docs/product/session-log.md:7-13`, OD-05 PARTIAL | Effort and Ultracode never observed (owner `/effort`, `/status`) |
| MW-05 | tested-partial | Separate reviewers for M1, M5, M6 and this audit. No self-approval found | No independent review for M7, M8 or the M9 RV6B fixes (F-19) |
| MW-06 | FAIL | This file is the first reconciliation since M1 | Integrate into traceability.md (F-01) |
| MW-07 | NOT RUN (n/a) | No token/agent-count records. None fabricated | Record only if the platform exposes it |
| MW-08 | verified | `OWNER_DECISIONS.md` OD-11/OD-20 (no push). No deploy or spend observed. The `clusterx` project untouched | — |

Note: MASTER §11 also lists MW-01..MW-05 with different wording. They map to §1.4 as follows: §11 MW-01 → MW-01, §11 MW-02 → MW-02, §11 MW-03 → MW-03 + MW-05, §11 MW-04 → MW-04, §11 MW-05 → MW-08.
