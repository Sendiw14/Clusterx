# AUDIT-M9-RV-03: Final independent re-audit before 1.0 (after AUDIT-M9-RV-01 NO-GO)

- **Ticket:** TASK-M9-RV-03 (MASTER §14 M9 gate, §15, §16). I am a read-only independent auditor and authored none of the audited code or docs.
- **Auditor:** `claude-opus-5-5`, a subagent of the integrator session. Effective effort and Ultracode cannot be seen from inside the subagent (UNVERIFIED, MW-04).
- **Target:** detached worktree `.worktrees/review` at main `27b0094`. The contract `api/openapi.yaml` `1.0.0-draft.7` is unchanged. Diff audited: `8fa9e09..27b0094` (22 commits, 79 files).
- **Authority:** this is a reviewer recommendation. **It does not authorize a release.** The human owner decides GO or NO-GO (MASTER §1.4, MW-05).
- **Read:** MASTER_PROMPT R2 (§0–§16). AUDIT-M9-RV-01 and its traceability table. REVIEW-M9-RV-02. `docs/testing/release-gates.md`, `docs/product/traceability.md` and `OWNER_DECISIONS.md`. HANDOFF-M9-BE-01, HANDOFF-M9-BE-02 and HANDOFF-M9-FE-02. The full diff. README, CHANGELOG and every changed public page and doc.
- **Writes:** only this file and `AUDIT-M9-RV-03-traceability.md` in the main checkout. Neither is committed. The review worktree's `git status` was clean before and after.
- **Scratch:** `<S>` = `/private/tmp/claude-501/-Users-sendiwkithnada-Documents-Clusterx/d0be9e8c-d590-4212-a424-878a0768bde1/scratchpad/rv10/`. It is ephemeral, so the decisive lines are quoted here.

## 1. Commands and results

Host: macOS arm64 (Apple M4), Node 26.13.1, pnpm 9.15.9, Go 1.27.4, PG 18.7 (`/opt/homebrew/opt/postgresql@18/bin`), gitleaks 8.30.1. Colima VM `default` (aarch64, 4 vCPU, 6 GiB), with Docker Compose v5.1.4 and buildx 0.34.1 in the VM.

**Integrator run on `27b0094` (not repeated by me, I\*):** every Go module ran vet + `-race` with exit 0 (PG 18 first on PATH). Web lint, typecheck, test and build: exit 0. Live E2E: Chromium, Firefox and WebKit each had 213 passed, 0 failed and 58 skipped.

| # | Command | Exit | Result |
|---|---|---|---|
| R1 | `git archive 27b0094` to `<S>/src`, `pnpm install --frozen-lockfile --offline`, `pnpm --filter web build` | 0 / 0 | **`○` (static):** `/`, `/_not-found`, `/architecture`, `/changelog`, `/demo`, `/docs`, the 7 `/docs/*` pages, `/docs/search-index.json` and `/icon.svg`. **`ƒ`:** `/[...unmatched]`, `/app/*`, `/auth/*`, `/bff/*` and `/login`. The table is byte-identical to `docs/testing/evidence/web/next-build-route-table.txt`. There is no `/app/workflows*` |
| R2 | `scripts/security/scan.sh <S>/sec gitleaks` on the review worktree (HEAD `27b0094`) | 0 | `gitleaks history exit=0`, `gitleaks tree exit=0` |
| R3 | Build-context probe. A scratch context holds the repo's `.dockerignore` plus planted `.env`, `.env.local`, `apps/web/.env.local`, `apps/web/.env.production`, `apps/web/.e2e/bff-private.jwk.json`, `apps/web/test-results/x`, `apps/web/playwright-report/index.html`, `apps/web/.next/x`, `infra/production/secrets/s`, `infra/compose/.secrets/database_url`, `infra/compose/.secrets.env`, `infra/production/.env.production`, `apps/control-plane/.env`, `apps/control-plane/prod.env`, and others. Then `docker build` with `COPY . /ctx; find /ctx -type f` (busybox:1.36). Image removed | 0 | Files that reach the image: `.dockerignore`, `apps/web/.env.example`, `infra/production/.env.production.example`, `apps/control-plane/prod.env`, `apps/web/coverage.txt`, `apps/web/tsconfig.tsbuildinfo`, `apps/web/page.tsx`. **Every secret and e2e path is excluded.** One residue: a nested `*.env` (e.g. `prod.env`) is not excluded, because `*.env` is root-anchored (RV10-05, P3) |
| R4 | Production example: an ephemeral `git clone` of `27b0094` in the **Colima VM** `/tmp/clusterx-rv10` (Colima mounts only `$HOME`, so compose secrets cannot bind from `<S>`). `.env.rv10`: `COMPOSE_PROJECT_NAME=clusterx-rv10`, `CLUSTERX_IMAGE_PREFIX=clusterx-rv10`, `TLS_MODE=files`, `127.0.0.1:18743/18780`, Prometheus 19790. Then `setup-local-proof.sh`, `verify-local-proof.sh`, the `--profile monitoring` targets and a runtime-image file probe | see §1a | see §1a |
| R5 | `promtool check rules alerts.yml` and `promtool test rules alerts_test.yml` (`quay.io/prometheus/prometheus:v3.16.0@sha256:9332…`, `--rm`) | 0 / 0 | `SUCCESS: 10 rules found`, `SUCCESS`. Rules: ControlPlaneDown, DatabaseUnavailable, QueueAgeHigh, JobsUnschedulable, WorkerOffline, NoLiveWorkers, LeaseExpiriesRepeated, FailureRateHigh, DiskSpaceLow, NodeExporterDown |
| R6 | AC-08 in `<S>/src/apps/control-plane`, `env -i`, Go only on PATH (`/opt/homebrew/bin` excluded because it holds PG 14 `initdb`). (a) `CI=1`, no initdb. (b) no `CI`, no initdb. (c) `CI=1`, PG 18.7 first on PATH, `-race` | 1 / 0 / 0 | (a) `--- FAIL: TestAC08_DatabaseOutageMidRun … initdb/pg_ctl not on PATH in CI; AC-08 must not skip silently (AUDIT-M9 F-03)`. (b) `--- SKIP`. (c) `initdb (PostgreSQL) 18.7`, `--- PASS: TestAC08_DatabaseOutageMidRun (11.54s)` |
| R7 | `go test -race -run TestMetricsBoundsConcurrentScrapes ./internal/api` and `go test -race ./internal/config` | 0 / 0 | RV9-05 test PASS. Config tests PASS, including `prod mutation limit off` (error), `prod … 5` (ok) and `dev … 0` (ok) |
| R8 | Built `/docs/api` HTML (from R1) scanned for served labels | n/a | **19 of 28 operations are labelled "Not served yet: answers 404 not_found_route"**: cancelJob, retryJob, drainWorker, getMetricsOverview, getMetricsTimeseries, listAuditEvents, createSession, deleteCurrentSession, getMe, list/create/revokeApiToken, list/create/revokeWorkerCredential and openStream. `apps/control-plane/internal/api/v1.go:92-122` routes all of them. RV10-02 |

**NOT RUN:** OSV scan and SBOM (`osv-scanner`/`syft` are not installed), and govulncheck on `27b0094` (also not installed; AUDIT-M9 G10 was 0 ×5 at `8fa9e09`, and only `go.mod`-neutral Go changed since). GitHub Actions (OD-11/20). The full E2E suite by me (the integrator's I\* run is the evidence). The backup/restore and alert drills. Benchmarks. Real GitHub OAuth. amd64. Manual accessibility. Real Safari/Edge.

### 1a. Production example (R4)

`setup-local-proof.sh` exited **0** after 1,091 s; the slow part was the cold pnpm download inside the web image build. `docker compose ls` before the run showed only `clusterx running(5)`. `verify-local-proof.sh` exited **0** on its second invocation. On the first invocation `docker compose run` inside `setup` used up the piped script's stdin, so I re-ran verify with `</dev/null`. That was a harness artefact, not a product issue. Selected lines:

```
TLS: console.rv10.test / api.rv10.test  Protocol TLSv1.3  Verify return code: 0 (ok)  issuer=CN = CLUSTERX local proof CA
client without the CA -> curl exit=60
GET /health/ready 200 http/2 | GET /metrics on the API host 404 | GET /api/v1/jobs without a token 401
job ... status=succeeded attempts_used=1 ; workers worker-1..3 "env":"production","control_plane":"control-plane:9090","tls":true
RV9-02  CLUSTERX_RETENTION_INTERVAL=1h ; CLUSTERX_RETENTION_DATABASE_URL_FILE=/run/secrets/retention_database_url
        before: jobs_row=1 tombstone=0 -> jobs_deleted=1 complete=true retention exit=0 -> after: jobs_row=0 tombstone=1 ; GET expired id 410
RV9-04  TRACE console 405, TRACE api 405, PROPFIND console 405, PROPFIND api 405 ; OPTIONS api 405 application/json (Go answers)
RV9-06  edge->control-plane:8080 open, edge->:9090 closed, edge->:9091 closed ; workers->:9090 open, workers->:9091 closed ;
        monitoring->cp-metrics:9091 open, monitoring->control-plane:9090 closed
web     GET / 200, GET /app 307 -> /login, GET /demo 200 ; static CSP, HSTS, X-Frame-Options DENY ; HTTP :18780 -> 308 https://console.rv10.test/ (no port: documented limit)
ports   only caddy publishes: 127.0.0.1:18780->80, 127.0.0.1:18743->443
RV9-07  caddy-1 user=65532:65532 read_only=true cap_drop=[ALL] cap_add=[] ; control-plane, web, workers same ; postgres user= (entrypoint, known exception)
secrets in env: only NODE_EXTRA_CA_CERTS=/run/secrets/internal_ca_crt (a path) ; networks data/workers/monitoring internal=true, edge false
monitoring profile: target https://cp-metrics:9091/metrics up ; http://node-exporter:9100/metrics up ; rules loaded 10
runtime web image (RV9-01): /app/apps/web/.e2e absent, test-results absent, playwright-report absent, no .env* outside node_modules
```

A cosmetic issue in the verify script: it prints `worker … active None` because it reads `w.get("liveness")`, which the workers API does not return under that name. It is not a product defect (P3 note, no ID).

**Cleanup:** `docker compose … --profile monitoring --profile retention down -v --remove-orphans` exited 0. Every `clusterx-rv10_*` network and volume was removed. The three `clusterx-rv10/*:local` images were deleted (a second `docker image rm` was needed, because my first teardown script fed `xargs` from `/dev/null`). The VM dir `/tmp/clusterx-rv10` was deleted. The probe image `clusterx-rv10-ctxprobe:tmp` was removed. Ports 18743/18780/19790 were free on the host afterwards. `docker compose ls` before and after showed only `clusterx running(5)`, which I never touched. `<S>/src` and `<S>/ctx` are scratch only. The review worktree's `git status` is clean.

## 2. Verdicts per §15 area (changes since AUDIT-M9-RV-01 marked Δ)

| Area | Verdict | Basis |
|---|---|---|
| Product fit | PASS with gaps | Unchanged. The console needs OD-03, and the stale-completion demo is shown only by tests |
| Routes / controls | Δ PASS (automated, 3 engines); manual NOT RUN | R1 route table. I\* E2E ×3 |
| Worker execution / concurrency | PASS | I\* `-race` all modules, chaos included. R6 |
| Data lifecycle | Δ PASS (local) | Retention now scheduled in the production example (R4) |
| Permissions / trust boundaries | Δ PASS (automated) | REVIEW-M9-RV-02 closed F-19. RV9 fixes re-verified (R4, R7). Real OAuth NOT RUN (OD-03) |
| Demo isolation | PASS | I\* `demo-isolation` ×3 |
| Accessibility | **FAIL for release (needs a human)** | axe 0 ×3 engines. Manual walkthrough NOT RUN (F-07) |
| Performance | Δ PASS-with-gaps | Public pages static (R1). Lighthouse lab re-measured. Console p95 measured. Bundle size not measured. Nothing frozen (OD-06) |
| Asset/font rights | PASS (P3 doc) | F-16 still open |
| Responsiveness / motion | PASS (automated) | 3 engines. 360 px missing (F-18). Screenshots not human-approved |
| Deployment | Δ PARTIAL | Production example re-verified locally by this audit (R4). Reference host blocked (OD-04) |
| Upgrade / recovery | PASS (local) | Unchanged |
| CI | Δ NOT RUN; config now plausible | F-02/F-03 fixed in `ci.yml` and the test. RV9-10 runner risks remain. Never run (OD-11/20) |
| Supply chain | Δ PASS (local, partial) | R2 gitleaks 0/0. R3 build context clean. OSV/SBOM NOT RUN |
| Documentation | **Δ FAIL (narrower)** | RV10-01, RV10-02, RV10-03 |
| License | PASS | Apache-2.0 |
| GitHub clarity | blocked-by-owner | OD-20. Issue templates now exist. README media missing (F-10) |
| Complexity | PASS | The catch-all 404 split is small and tested (`static-public.spec.ts`) |

## 3. Disposition of every prior finding

### AUDIT-M9-RV-01

| ID | Sev | Disposition | Evidence |
|---|---|---|---|
| F-01 | P1 | **CLOSED** as written (traceability folded, README pointer valid). Staleness since then is reopened as RV10-03 | `docs/product/traceability.md` |
| F-02 | P1 | **CLOSED in config. Proving run BLOCKED-BY-OWNER** (OD-11/20) | `ci.yml` web job: setup-go, PGDG `postgresql-18`, `CLUSTERX_E2E_PG_BIN`, `playwright install --with-deps chromium firefox webkit` |
| F-03 | P2 | **CLOSED** | R6 a/b/c. The go job puts PG 18 on `GITHUB_PATH` |
| F-04 | P2 | **CLOSED** | R2. `scan.sh` tree mode scans `git archive HEAD` with the tree fingerprints |
| F-05 | P2 | **CLOSED** | R1. `not-found.tsx` has no `connection()`. `[...unmatched]/page.tsx` is `force-dynamic` + `notFound()`. `static-public.spec.ts` guards it. `lighthouse-m9.md` re-measured |
| F-06 | P2 | **CLOSED for the pages named** (changelog, deployment). Residue on other pages: RV10-02 | content read, numbers cross-checked against `benchmarks.md` and `drill.txt` |
| F-07 | P1 | **OPEN, needs a human tester** | `browser-matrix.md` note 2: NOT RUN |
| F-08 | P2 | **CLOSED** for engines: 3 Playwright engines, 213/0 each (I\*), supported matrix written in `browser-matrix.md`. Residual: real Safari/Edge and branded Chrome untested. The owner accepts the matrix as stated, or the team runs real Safari on this Mac | `playwright.config.ts`, `browser-matrix.md` |
| F-09 | P2 | **PARTIAL.** Console p95 measured in 3 engines, budgets proposed. **Bundle size not measured** (team). Freeze is OD-06 (owner) | `console-budgets.md` |
| F-10 | P2 | **PARTIAL.** `release-gates.md` and issue templates were added. **`docs/testing/test-matrix.md` is still missing, and the README has no screenshots or video** (M9 deliverable) | `ls docs/testing .github/ISSUE_TEMPLATE`; README |
| F-11 | P3 | **CLOSED** (README `$HOME` line). See RV10-01 for an overclaim introduced alongside | README:40-41 |
| F-12 | P3 | **CLOSED** | R5 promtool. The state-machine E8, ADR-0003, `/docs/reliability` and `/docs/concepts` notes |
| F-13 | P3 | **ACCEPTED** as a 1.x item under the AUDIT-M9 §4 ruling (valid credentials are never throttled). Public security page discloses it | `content/docs/security.md` |
| F-14 | P3 | OPEN, not blocking (the wrapper is documented; image 1.43 GB is disclosed on `/docs/deployment`) | — |
| F-15 | P3 | OPEN, fix at tag (`packages/sdk-go/worker.go:26` `Version = "0.5.0-m5"`) | — |
| F-16 | P3 | OPEN (`docs/design/motion-and-assets.md:81-82` still says "not committed") | — |
| F-17 | P3 | **CLOSED** (CHANGELOG says "CI workflows defined (CI not yet run)") | — |
| F-18 | P3 | OPEN (`.nvmrc` = `26`; `site.spec.ts:4` WIDTHS lacks 360) | — |
| F-19 | P2 | **CLOSED** by REVIEW-M9-RV-02. Its fixes are re-verified here (R3–R7) | — |
| F-20 | P2 | **CLOSED for documentation** (`/docs/quickstart` "Try the console without GitHub"). A real console is **BLOCKED-BY-OWNER** (OD-03) | — |
| F-21 | P3 | OPEN (no `func Fuzz` in the repo) | — |

### REVIEW-M9-RV-02

| ID | Sev | Disposition | Evidence |
|---|---|---|---|
| RV9-01 | P2 | **CLOSED** | R3, and the R4 runtime-image probe |
| RV9-02 | P2 | **CLOSED** (retention loop set; on-demand pass deletes a 31-day job, tombstone written, API 410). The hourly tick itself was not observed | R4 |
| RV9-03 | P2 | **CLOSED for the items listed** (home, architecture, commands, reliability, security, quickstart, deployment, changelog, F-12 sentence). New residue RV10-01, RV10-02 | content read |
| RV9-04 | P3 | **CLOSED** (TRACE/PROPFIND 405 on both hosts) | `Caddyfile` `@badmethod`, R4 |
| RV9-05 | P3 | **CLOSED** | `server.go` `MaxRequestsInFlight: 1, Timeout: 10s`. R7 |
| RV9-06 | P3 | **CLOSED** (edge cannot reach 9090/9091; metrics only from `monitoring`; Prometheus targets up). gRPC reachable from `workers` peers by design | R4 |
| RV9-07 | P3 | **CLOSED** (`caddy-1 user=65532:65532 cap_add=[]`, serves 443/80) | R4 |
| RV9-08 | P3 | **CLOSED** | `config.go:315-318`, R7 |
| RV9-09 | P3 | **CLOSED by code read** (`.secrets/database_url.drill-orig`, umask 077, `trap cleanup EXIT`; `backup.sh` traps `.partial`/`.toc`). **The drill was NOT re-run** after the change | diff |
| RV9-10 | P3 | **OPEN**, unchanged: no `locale-gen`, no `create_main_cluster = false`, `postgresql-18` unpinned, PGDG key fetched without a fingerprint check, service image `postgres:18.7-bookworm` not digest-pinned (`ci.yml:43-49,57-60,78-85`). Only a real run settles it | `ci.yml` |
| RV9-11 | P3 | **CLOSED** | R5 (`ClusterxNodeExporterDown`, `up == 0` or `absent`) |

## 4. New findings

| ID | Sev | file:line | Problem | Evidence | Fix | Blocks 1.0? |
|---|---|---|---|---|---|---|
| RV10-01 | P2 | `README.md:40`; `apps/web/content/docs/quickstart.md:13`; `apps/web/content/docs/deployment.md:13`; `apps/web/src/content/commands.ts:27`; `docs/testing/release-gates.md:18` | The files claim an independent audit reproduced the quickstart **from a clean clone on macOS under `$HOME`**. No such run exists. AUDIT-M9-RV-01 A3 (macOS host clone) **failed** because it was outside `$HOME`. The only successful clean clone was A4, in the Linux VM. "Works under `$HOME`" is an inference from F-11, not evidence. At `8fa9e09` the README said macOS was verified by the integrator, and a clean checkout was "not yet verified". `traceability.md:81` (AC-26) is still correct, so the docs disagree with it | AUDIT-M9-RV-01 §1a A3/A4. HANDOFF-M9-FE-02 evidence map cites "A3 to A5" for a macOS claim A3 refutes | Say "clean clone verified in a Linux arm64 VM; on macOS it failed outside `$HOME` (Colima mounts only `$HOME`); a macOS clean clone under `$HOME` has not been run". Or run it (any non-author, under `$HOME`) and cite the log | **Yes** (AC-28; trivial fix) |
| RV10-02 | P2 | `apps/web/src/lib/docs/openapi.ts:129-130,166`; `api/openapi.yaml:32-33`; `apps/web/content/docs/concepts.md:91`; `apps/web/content/docs/workers.md:134`; tests `apps/web/e2e/docs.spec.ts:117`, `apps/web/src/__tests__/docs.test.tsx:81-82` | The shipped public API reference marks **19 of 28 routed operations** as "Not served yet: answers 404 not_found_route". The cause: the page parses the draft.5 changelog sentence "M4 serves … with direct API tokens" from `info.description` and ignores the draft.6 line "every operation is routed". `/docs/concepts` says cancel and manual retry "are not served in the current build" (they have been routed since M5; `v1.go:99-100`; E2E `console-jobs` cancels and retries). `/docs/workers` says "Handler log capture is not implemented: attempt `log_tail` is empty" (`packages/sdk-go/logcap.go`, commit `6dbd6dd` M5, reports `log_tail`). Two tests **pin** the wrong label. This is the same pattern RV9-03 found in `site.spec`. These pages were in the scope of TASK-M9-FE-02 F-06 ("and any other web docs") | R8. `v1.go:92-122`. `TestAC13_RouteMatrix` | Make the served list come from the contract truth. Either add a machine-readable flag (`x-clusterx-served`) or treat every operation as served from draft.6 on. Remove the two sentences. Update `docs.spec.ts:117` and `docs.test.tsx:81-83` to assert cancelJob is served. A contract edit is integrator/backend-owned, so the simplest fix is a web-side rule | **Yes** (AC-28, AC-21: public docs contradict the product) |
| RV10-03 | P2 | `docs/product/traceability.md` (whole AC table, "Post-audit changes", Milestones); `docs/testing/release-gates.md:9-25` | Neither record was updated after FE-02, BE-02 or REVIEW-M9-RV-02 merged. traceability still says: AC-24 FAIL "public pages are now dynamic"; AC-25 FAIL "gitleaks tree exit 1"; AC-08 "CI would silently skip"; AC-13/MW-05 "RV6B not independently re-reviewed (F-19)"; AC-21/23 "Chromium only"; M7/M8 review "none (F-19)". release-gates says cross-browser NOT RUN, independent review NOT RUN, live E2E "211 passed", and still has the NO-GO line. Both are integrator-owned. MASTER §0 and MW-06 require the record to reconcile with the actual state | File content at `27b0094`, set against §3 above | Fold `AUDIT-M9-RV-03-traceability.md` into traceability.md and refresh release-gates.md in the same change. Add the I\* run (213/0/58 ×3) with the exact commit | **Yes** (AC-30, MW-06) |
| RV10-04 | P3 | `CHANGELOG.md:5-24` | The root CHANGELOG has no entry for the M9 audit fixes (static public pages, 3-engine E2E, retention in the production example, Caddy 405/nonroot, network scoping, bounded scrape, prod rate-limit refusal, tenth alert). The web changelog has them | File content | Add one M9 line | No |
| RV10-05 | P3 | `.dockerignore:7` | `*.env` is root-anchored. A nested `foo.env` (e.g. `apps/control-plane/prod.env`) still enters the image context. `.env*` is covered by `**/.env*`. Neither file name exists today | R3 | Add `**/*.env` | No |
| RV10-06 | P3 | `apps/web/content/docs/reliability.md` "Non-guarantees" | "No published performance numbers. See the end of this page." The end of the page publishes a measured table. Read literally, these contradict each other | File content | "No promised performance numbers." | No |

No P0 or P1 found. I found no engine, authorization or session defect. All three P2s are documentation or record defects, and each takes under an hour.

## 5. Public claims (AC-28 / AC-29)

- **README:** every link resolves. Guarantees match code and ADRs. One false line (RV10-01).
- **CHANGELOG (root):** honest about CI ("defined, not yet run"). M9 fixes are missing (RV10-04).
- **Web changelog, `/docs/deployment`, `/docs/reliability`, `/docs/security`, home and architecture:** these match the evidence. I checked the numbers 114.7–129.9 / 199.2–233.9 / 242.3–280.5 jobs/s, 34.1 s, RPO 2.2 s, RTO 14.1/71.8 s, backup 0.26 s and ready 0.22 s against `docs/testing/benchmarks.md`, `docs/operations/backup-restore.md` and `drill.txt`. Ten alert rules matches R5. Retention wording now matches the production compose (`CLUSTERX_RETENTION_INTERVAL: 1h`). The "fixes not yet re-reviewed" sentence was true at `27b0094`; this audit is that re-review for RV9-01..11.
- **`/docs/api`, `/docs/concepts`, `/docs/workers`:** false "not served / not implemented" claims (RV10-02).
- **`/docs/quickstart`:** the macOS clean-clone claim is unbacked (RV10-01). The harness section is honest about mock OAuth.
- **AC-29:** PASS. The route table has no `/app/workflows*`. Grepping public copy for workflow, DAG and HA finds only "not in 1.0" statements and the GitHub-Actions sentence.

## 6. Updated AC / MW table

See `AUDIT-M9-RV-03-traceability.md` (same format as `AUDIT-M9-traceability.md`). Summary: **verified 20 (AC-18 counted as partial), tested-partial 12, FAIL 4** (AC-22, AC-28, AC-30, MW-06), **blocked-by-owner 1** (AC-27), and **NOT RUN 1** (MW-07, n/a).

## 7. Release readiness: recommendation

**Recommendation: NO-GO for 1.0 at `27b0094`.** This is the reviewer's recommendation only. The human owner keeps release authority.

The engine, security boundaries, production example and supply-chain gates hold under re-verification. Of the 21 AUDIT-M9 findings, 11 are closed and F-13 is accepted. Of the 11 RV9 findings, 10 are closed. The remaining blockers fall into three groups.

**(a) Engineering gates the team can still close locally (no owner input needed):**

1. RV10-02: fix the `/docs/api` served labels, the concepts and workers sentences, and the two tests that pin them. Re-run docs specs in 3 engines.
2. RV10-01: correct the macOS clean-clone claim in 5 places, or run a macOS clean clone under `$HOME` with a non-author and cite it.
3. RV10-03: reconcile `traceability.md` and `release-gates.md` to this audit (AC-30, MW-06).
4. F-10: add `docs/testing/test-matrix.md` and README screenshots/video.
5. F-09: measure the console JS bundle size and propose a budget.
6. AC-25: run OSV and SBOM locally (`scripts/security/install-tools.sh`, then `scan.sh <dir> osv sbom govulncheck pnpm`) on the final commit.
7. Recommended but not blocking: RV10-04/05/06, F-14/15/16/18/21, RV9-10 hardening before the first CI run, and the RV9-09 drill re-run.

**(b) Gates that need a human:**

- F-07 / AC-22: manual NVDA/VoiceOver, keyboard-only, 200/400 % zoom, forced-colors and text-spacing walkthroughs (accessibility-plan scripts S1–S11). Approving the AC-23 screenshots is also a human task. If the owner wants the matrix to include real Safari or Edge (F-08 residual), a human runs it or the owner accepts the Playwright-engine statement.

**(c) Owner decisions:**

- **OD-03:** real GitHub OAuth app and first operator (AC-13 real provider, AC-15, F-20).
- **OD-04:** reference Linux amd64 host, domain and TLS (AC-19, AC-26 amd64, AC-27).
- **OD-06:** freeze SLO, RPO, RTO and console budgets (AC-17, AC-19, AC-24).
- **OD-11/OD-20:** authorize a push or a private CI run, so `ci` and `security` run green once on the exact release commit (AC-20, AC-25, AC-28 "CI results", F-02, RV9-10). Also publication with private vulnerability reporting (OD-19).

**Plain statement:** the remaining blockers are **not yet only (b) + (c)**. Three small (a) items block AC-28 and AC-30: RV10-01, RV10-02 and RV10-03, plus F-10 test-matrix/README media and the OSV/SBOM run. Each takes under a day, and none touches engine code. Once they are fixed and spot-checked, the only remaining blockers will be (b) the manual accessibility pass and (c) OD-03, OD-04, OD-06 and OD-11/20.
