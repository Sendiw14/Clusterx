# AUDIT-M9-RV-04: Spot-check of fixes made after AUDIT-M9-RV-03

- **Ticket:** TASK-M9-RV-04 (MASTER §14 M9 gate, §15, §16). I am a read-only independent auditor. I authored none of the changes I audited.
- **Auditor:** `claude-opus-5-5`, running as a subagent of the integrator session. Effective effort and Ultracode cannot be seen from inside the subagent (UNVERIFIED, MW-04).
- **Target:** detached worktree `.worktrees/review` at main `1183335`. Diff audited: `27b0094..1183335` (6 commits, 32 files). The contract `api/openapi.yaml` `1.0.0-draft.7` is unchanged.
- **Authority:** this is a reviewer recommendation. **It does not authorize a release.** The human owner decides GO or NO-GO (MASTER §1.4, MW-05).
- **Read:** AUDIT-M9-RV-03 and its traceability file, HANDOFF-M9-FE-03, `docs/product/traceability.md` (header and "Changes after AUDIT-M9-RV-03"), `docs/testing/release-gates.md`, `docs/testing/test-matrix.md`, `docs/testing/evidence/m9/supply-chain.md`, `docs/testing/evidence/web/bundle-size.md`, and the full diff. I also read README, CHANGELOG, every `apps/web/content/**` page in the diff and `src/content/commands.ts`.
- **Writes:** only this file, which is not committed. The review worktree's `git status` was clean before and after. `docker compose ls` showed only `clusterx running(5)` before and after; I did not touch that project.
- **Scratch:** `<S>` = `/private/tmp/claude-501/-Users-sendiwkithnada-Documents-Clusterx/d0be9e8c-d590-4212-a424-878a0768bde1/scratchpad/rv11/`. It is ephemeral, so the decisive lines are quoted below.

## 1. Commands and results

Host: macOS arm64, Node 26.13.1, pnpm 9.15.9, osv-scanner 2.7.1, syft 1.57.2, gitleaks 8.30.1. Docker is Colima `default`.

**Integrator run on `1183335` (I\*, not repeated by me):** web lint, typecheck, test and build exited 0. Live E2E in Chromium, Firefox and WebKit had 213 passed and 0 failed in each engine. Full `scan.sh` exited 0 (govulncheck ×5, pnpm audit, OSV, gitleaks history and tree, SBOM of the source and 2 images).

| # | Command | Exit | Result |
|---|---|---|---|
| S1 | `CLUSTERX_SBOM_IMAGES="clusterx/control-plane:local clusterx/worker:local" scripts/security/scan.sh <S>/scan osv gitleaks sbom`, run in the review worktree (`ROOT` = `1183335`) | **0** | `osv-scanner exit=0`: "No issues found". It scanned `pnpm-lock.yaml` (555 packages) and 6 `go.mod` files (control-plane 24, worker 10, sdk-go 8, proto 7, basic-jobs 9, bench driver 1); 9 local packages were filtered out. `gitleaks history exit=0` (163 commits, "no leaks found"). `gitleaks tree exit=0`. `syft source exit=0` (617 packages). `syft clusterx/control-plane:local exit=0 packages=31`. `syft clusterx/worker:local exit=0 packages=18` |
| S2 | Package versions in the S1 SBOMs, and the image creation dates (`docker images`) | n/a | Both image SBOMs list **`golang.org/x/net v0.60.0`**. The source SBOM and both `go.mod` files list `v0.62.0`. The images were created **2026-11-19 18:14 +0530**. That is before `0f6865d` (the x/net bump for GO-2026-6860/6866) and about 20 other Go commits. See RV11-03 |
| S3 | `git diff --stat e0f36ea..1183335` on lockfiles, `go.mod`/`go.sum`, `package.json` and Dockerfiles | n/a | Empty. The integrator's evidence commit `e0f36ea` and `1183335` have the same dependency inputs, so the OSV and source-SBOM results carry over. S1 confirms this |
| S4 | `git archive 1183335` to `<S>/src`, `pnpm install --frozen-lockfile --offline`, then `vitest run docs.test.tsx chart-ticks.test.tsx`, then full `pnpm test` | 0 / 0 / 0 | 2 files, 22 tests passed. Full suite: 15 files, **136 tests passed** |
| S5 | Mutation check of the new `v1.go` parity test in `<S>/src`: (a) delete the `/api/v1/jobs/{id}/cancel` route line; (b) rename `/api/v1/audit` to `/api/v1/audit2`. `v1.go` was restored and checked with `cmp` | 1 / 1 | (a) `spec operations v1.go does not route: expected [ 'POST /api/v1/jobs/{id}/cancel' ] to deeply equal []`. (b) `… [ 'GET /api/v1/audit' ] …`. The test catches a lost route and a renamed route |
| S6 | `pnpm --filter web build` in `<S>/src`, then a scan of the prerendered `.next/server/app/docs/*.html` | 0 | "Not served yet" appears **0** times on api, concepts, workers, quickstart, deployment, security and reliability. "Served in this build" appears 58 times on `/docs/api` (28 chips plus the callout). The "Executed in the M4 curl guide" chips are still there. See RV11-04 |
| S7 | `node scripts/bundle-size.mjs` on the S6 build, compared with the committed `bundle-size.json` | 0 | Byte-identical for `/app` (593,277 raw / 177,800 gzip), `/docs/api`, `/` and `/demo` |
| S8 | Build-context probe: a scratch context under `$HOME` holding the repo `.dockerignore` plus `apps/control-plane/prod.env`, `infra/x/a.env`, `root.env` and `apps/control-plane/keep.txt`. Then `docker build` (busybox:1.36, already cached) with `COPY . /ctx; find /ctx -type f`. The image and the directory were removed afterwards | 0 | Only `.dockerignore`, `Dockerfile` and `keep.txt` reached the image. **Nested `*.env` files are now excluded** (RV10-05) |
| S9 | Public-copy grep of README, CHANGELOG, `apps/web/content/**`, `src/content/**`, `src/app/**` and `src/features/**` for macOS clean-clone wording, "not served", "not implemented", "log_tail is empty", "not re-reviewed" and OSV/SBOM | n/a | No macOS clean-clone overclaim remains. Remaining items are listed in §4 |

**NOT RUN by me:** govulncheck and pnpm audit (I\* only). Live E2E (I\*). Web lint and typecheck (I\*). Everything else that AUDIT-M9-RV-03 listed as NOT RUN is still NOT RUN: GitHub Actions, the manual accessibility pass, real OAuth, amd64, the reference host, the drill re-runs, and real Safari and Edge.

## 2. Disposition of the AUDIT-M9-RV-03 findings

| ID | Sev | Verdict | Evidence |
|---|---|---|---|
| RV10-01 | P2 | **CLOSED** | `README.md:47-48`, `quickstart.md:13,19`, `deployment.md:13`, `commands.ts:27` and the release-gates "Clean-checkout reproduction" row now say: clean clone verified on the Linux arm64 VM only; on macOS the clean clone outside `$HOME` failed; a macOS clean clone under `$HOME` was not run. That matches AUDIT-M9-RV-01 A3/A4. A guard test (`docs.test.tsx` "RV10 stale-claim guards") blocks the old wording in docs and `commands.ts`. It does not cover README. S9 |
| RV10-02 | P2 | **CLOSED** | `openapi.ts` checks "every operation is routed" first, so all 28 operations are served. The callout is reworded. The concepts sentence is removed. The workers note matches `sdk-go/handler.go:19-22` and `logcap.go:14` (`MaxLogTailBytes = 16384`, `redactAttr`). Both tests that had pinned the wrong label now assert served. The new parity test compares every method and path pair in `v1.go` with the spec in both directions (S4, S5). Built HTML has no "Not served yet" (S6). There is a residual copy nuance in RV11-04 |
| RV10-03 | P2 | **CLOSED** (honest overlay; see §3) | The traceability header says the AC/MW table is RV-03 verbatim, and I confirmed that with `diff` against `AUDIT-M9-RV-03-traceability.md`. Later changes sit in a separate table marked "not independently re-audited". release-gates.md is refreshed with 213/engine, 136 unit tests, the WebKit flake note and "fixed after RV-03 (not re-audited)" |
| RV10-04 | P3 | **CLOSED** | The root `CHANGELOG.md` has an M9 audit-fixes entry and says "CI not yet run" |
| RV10-05 | P3 | **CLOSED** | `.dockerignore:20` `**/*.env`. S8 |
| RV10-06 | P3 | **CLOSED** | `reliability.md:29` now reads "No promised performance numbers … not capacity claims" |
| F-09 | P2 | **CLOSED (measured); freeze is OD-06** | S7 reproduces the numbers exactly. Budgets are a proposal only, and no CI gate is claimed |
| F-10 | P2 | **CLOSED** | `docs/testing/test-matrix.md` exists. Its commands and AC mapping are plausible, and its CI line ("have not run") is honest. README has two screenshots. One caption-date nit is in RV11-05 |
| AC-25 OSV/SBOM | — | **PASS locally for source**. Image SBOM is stale (RV11-03) | S1–S3 |
| Chart ticks | — | **CLOSED** | `axisTicks` returns unique integers for counts. The test covers max 0–5 and rendered labels. Counts are always integers ≥ 1 (`chart.tsx:43`), so `ceil` cannot overshoot the scale |

The earlier P3 items and RV9-10 are unchanged: F-14, F-15 (fix at tag), F-16, F-18, F-21, RV9-09 (drill not re-run) and RV9-10 (CI runner hardening, OPEN).

## 3. Traceability and release-gates accuracy (AC-30, MW-06)

- **No row claims a better state than its evidence supports, with one exception.** The overlay entry "AC-25 locally complete" and the release-gates row "Supply chain … PASS (local)" include image SBOMs of images that predate the release commit (RV11-03). Source-level OSV, gitleaks and the source SBOM are accurate at `1183335` (S1, S3).
- The AC table is deliberately a verbatim copy of the RV-03 snapshot, so AC-28, AC-30 and MW-06 still read FAIL, and AC-21, AC-24, AC-25 and AC-26 still name gaps that are now closed. That is honest, because the overlay says so, but it is out of date as of this audit. **Integrator action (bookkeeping, not a blocker):** fold this audit's verdicts into the rows. Suggested values: AC-21 tested-partial (manual NOT RUN). AC-24 tested-partial (bundle measured, not frozen). AC-25 tested-partial (local PASS, image SBOM to regenerate, CI NOT RUN). AC-26 tested-partial (claim fixed; amd64 NOT RUN). AC-28 tested-partial (P3 residue below; CI results blocked OD-11/20). AC-30 and MW-06 verified (local).
- `supply-chain.md` names commit `e0f36ea`, not the final commit. S3 shows the inputs are identical, so it is accurate in substance. Add "re-confirmed at `1183335` by AUDIT-M9-RV-04 S1".
- **AC-30 / MW-06 verdict: PASS (local), with the bookkeeping fold above.** Every AC and MW ID is mapped to evidence and a disposition, every open gap is named, NOT RUN is used correctly, and no CI or manual result is fabricated.

## 4. New findings

| ID | Sev | file:line | Problem | Evidence | Fix | Blocks 1.0? |
|---|---|---|---|---|---|---|
| RV11-01 | P3 | `apps/web/content/docs/security.md:83`; `apps/web/content/changelog.md:86` | Both pages still say OSV and an SBOM were not run: "OSV scanning and an SBOM were **not run**", and on the changelog "have not been run locally either". They now have been, locally, with exit 0. The claim is stale in the conservative direction, but under AC-28 public claims should match | S1; `supply-chain.md` | Replace with "OSV and an SBOM were run locally (no issues found); the scan workflow has never run on GitHub" | No (underclaim) |
| RV11-02 | P3 | `apps/web/content/docs/security.md:83`; `apps/web/content/changelog.md:87` | "its findings … are fixed, and a final audit (AUDIT-M9-RV-03) re-checked … and closed them" is broader than the record. RV9-10 (CI runner hardening) is **OPEN**. RV9-09 was closed by code read only (drill not re-run). The RV9-02 hourly tick was not observed. `security.md:11` ("re-checked in a final audit") is accurate | AUDIT-M9-RV-03 §3 RV9 table | "all but one are fixed (the open one hardens the CI runner, which has never run); AUDIT-M9-RV-03 re-checked and closed the rest" | No, but fix before tag (AC-28 literal) |
| RV11-03 | P3 | `docs/testing/evidence/m9/supply-chain.md`; traceability overlay "AC-25"; release-gates "Supply chain" row | The image SBOMs (31 / 18 packages) come from `clusterx/*:local` images built on 2026-11-19 18:14 from pre-M8 code. Those images contain `golang.org/x/net v0.60.0`; the release `go.mod` has `v0.62.0`. The evidence file sets them under commit `e0f36ea`, which implies they describe that commit. They do not describe the release artefact. govulncheck reports the x/net advisories unreachable, so this is an evidence-accuracy issue, not a known exploitable defect | S1, S2 | Build images from the release commit under a **separate** compose project or tag (never the running `clusterx` project), then re-run `scan.sh … sbom` with those tags. Or label the image SBOMs with the image ID and build date and drop them from the "locally complete" claim | No for local evidence. **Yes for the tag's release artefacts** |
| RV11-04 | P3 | `apps/web/src/features/api-reference.tsx:94`; `apps/web/content/docs/api.md:9` | The text says "the labels below say which operations a recorded run or an end-to-end test executed". The only label is "Executed in the M4 curl guide" (11 operations). Operations exercised by E2E, such as cancel, retry, drain, tokens and worker credentials, have no label, so a reader infers they were never run. That is an underclaim, and HANDOFF-M9-FE-03 notes the limitation itself | S6; `api-reference.tsx:42` | Say "the M4 curl-guide label marks the 11 operations that guide ran; end-to-end coverage is listed in docs/testing/test-matrix.md", or drop the sentence | No |
| RV11-05 | P3 | `README.md:14` | "captured 2026-11-19". `overview-1440.png` was re-captured on 2026-11-20 (`61dc366`, HANDOFF-M9-FE-03) | `git log` | "captured 2026-11-19/20" | No |

No P0, P1 or P2 found. No engine, authorization, lease, cancel or credential code changed in this diff (`git diff --stat` touches only web, docs, `.dockerignore` and `scan.sh`), so this audit re-reviewed none of those.

## 5. Public claims (AC-28 / AC-29)

- **macOS clean clone:** no overclaim remains in README, the web content, `commands.ts` or release-gates (S9). The README wording is accurate: run on macOS from a checkout under `$HOME`, not from an audited clean clone.
- **`/docs/api`, `/docs/concepts`, `/docs/workers`:** these now match `v1.go` and the SDK (S4–S6). There is one underclaim (RV11-04).
- **Overclaims:** one mild overclaim remains, RV11-02 ("findings are fixed … closed"). There are also two stale underclaims (RV11-01) and one caption date (RV11-05). Nothing claims CI ran, real OAuth, amd64, a reference host, a manual accessibility pass or frozen budgets.
- **AC-29:** unchanged, PASS. No route was added.
- **AC-28 overall:** **tested-partial.** The RV10 overclaims are fixed and the P3 residue is above. The "CI results" part of AC-28 remains blocked by OD-11/20.

## 6. Release readiness: recommendation

**Recommendation: NO-GO for 1.0 at `1183335`, for (b) and (c) reasons only.** This is the reviewer's recommendation. The human owner keeps release authority.

**(a) Team-closable locally:** **no release-blocking item remains.** Every RV-03 (a) blocker is closed: RV10-01, RV10-02, RV10-03, F-09, F-10 and the OSV/SBOM run. Small P3 items remain. I recommend doing them before the tag, and each takes less than an hour:
1. RV11-02: narrow the "findings fixed and closed" sentence (`security.md:83`, `changelog.md:87`).
2. RV11-01: update the OSV/SBOM sentences (`security.md:83`, `changelog.md:86`).
3. RV11-03: regenerate the image SBOMs from images built at the release commit, in a separate compose project or tag, or relabel them. This is required for the tagged artefacts.
4. Fold this audit into the traceability AC rows (§3).
5. Optional: RV11-04, RV11-05, F-14, F-15 (at tag), F-16, F-18, F-21, the RV9-09 drill re-run, and RV9-10 hardening before the first CI run.

**(b) Needs a human:**
- F-07 / AC-22: manual NVDA/VoiceOver, keyboard-only, 200/400 % zoom, forced-colors and text-spacing walkthroughs (accessibility-plan S1–S11).
- Human approval of the AC-23 screenshots.
- Real Safari or Edge, unless the owner accepts the Playwright-engine matrix (F-08 residual).

**(c) Owner decisions:**
- **OD-03:** real GitHub OAuth app and the first operator (AC-13 real provider, AC-15).
- **OD-04:** reference Linux amd64 host, DNS/ACME and firewall (AC-19, AC-26 amd64, AC-27).
- **OD-06:** freeze SLO, RPO, RTO, console budgets and the bundle ceilings (AC-17, AC-19, AC-24).
- **OD-11/OD-20 (and OD-19):** authorize a push or a private CI run, so that `ci` and `security` run green once on the exact release commit (AC-20, AC-25 in CI, AC-28 "CI results", RV9-10). Also publication with private vulnerability reporting.

**Plain statement:** (a) is now **empty of release blockers**. Only P3 corrections are left, and the team can do them without owner input. The remaining 1.0 blockers are (b), the manual accessibility pass and screenshot approval, and (c), OD-03, OD-04, OD-06 and OD-11/20.
