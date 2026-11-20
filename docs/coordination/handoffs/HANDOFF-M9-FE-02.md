# HANDOFF — TASK-M9-FE-02 (web audit fixes: static public pages, docs truth, browsers, performance)

- **Author:** frontend lane, model `claude-sonnet-5-5`, requested effort high. Effective effort and Ultracode: UNVERIFIED from inside the session (MW-04).
- **Branch:** `m9fix/frontend` (worktree `.worktrees/frontend`), base main `6222d0c`. Not pushed, not merged. Main moved to `166b581` while this ran (backend M9-BE-01/02 merged); the web docs were written against main's content (see "Merge notes").
- **Source findings:** AUDIT-M9-RV-01 F-05, F-06, F-08, F-09, F-20; integrator additions: deadline-before-lease precedence; REVIEW-M9-RV-02 RV9-03.
- **Contract:** `api/openapi.yaml` `1.0.0-draft.7`, last changed in commit `c4c3eb0`, blob SHA `4eb0f6800764ce3402b41f29aa329ff260b70e2f`. Not changed. No SQL, OpenAPI or proto edits.
- **Environment:** Apple M4 (10 cores, 16 GiB), macOS (Darwin 27.0, arm64), Node 26.13.1, pnpm 9.15.9, Go 1.27.4, PostgreSQL 18.7, Playwright 1.65.1. Browsers (Playwright builds, headless): Chromium 159.0.8154.6, Firefox 159.0, WebKit 27.2. Downloaded with `playwright install firefox webkit`; the first download stalled for about 20 minutes and then completed.

## Commits (all on `m9fix/frontend`, trailer `Co-Authored-By: Claude Sonnet 5.5`)

| SHA | Content |
|---|---|
| `db84145` | F-05: root `not-found` static, `app/[...unmatched]` per-request 404, `next.config.ts` comment, `static-public.spec.ts`, route table evidence |
| `dd0a83a` | F-06, F-20, RV9-03: changelog, deployment, reliability, concepts, security, quickstart, home and architecture copy, `site.spec.ts` |
| `e41323a` | F-08: firefox/webkit projects, per-browser axe evidence, browser matrix, fixes the new browsers found |
| `5782876` | F-09: `perf-console.spec.ts`, raw data per browser, `console-budgets.md` |
| `c1f0931` | F-05 Lighthouse re-measure, retention wording after RV9-02 |
| (this commit) | handoff plus the last docs wording (ten alert rules, M9 changelog entry) |

## Per item

| Item | Status | What changed | Evidence |
|---|---|---|---|
| **F-05** | **Fixed** | `connection()` removed from root `not-found.tsx`. Unmatched URLs are answered by `src/app/[...unmatched]/page.tsx` (`force-dynamic`, `notFound()`), a per-request render that carries the request nonce, so `/app/x`, `/APP/x`, `/login/x` 404s still hydrate under the nonce CSP (`console-doc-boundary.spec.ts` green in all 3 browsers). `next.config.ts` comment rewritten to be true. New `static-public.spec.ts` fails if a public route leaves `.next/prerender-manifest.json` or loses the static CSP and `x-nextjs-cache: HIT`. Lint rule `no-html-link-for-pages` now sees the catch-all, so one unit-test `<a href="/docs">` became `#docs` | `docs/testing/evidence/web/next-build-route-table.txt`: `○` for `/`, `/architecture`, `/changelog`, `/demo`, `/docs`, `/docs/{api,concepts,deployment,quickstart,reliability,security,workers,search-index.json}`, `/_not-found`; `ƒ` for `/app/*`, `/auth/*`, `/bff/*`, `/login`, `/[...unmatched]`. Lighthouse: `docs/design/evidence/lighthouse-m9.md` and `.json` |
| **F-05 Lighthouse** | **Re-measured** | `/`, `/docs`, `/demo`, `/architecture`, mobile and desktop, 3 runs each (median). `lighthouse-m7.md` marked superseded | Performance 100 on every page and preset except `/` desktop (99); LCP mobile: `/` 1.1 s, `/docs` 0.8 s, `/demo` 0.9 s, `/architecture` 1.1 s; desktop 0.2 to 0.9 s; CLS 0 to 0.021; TBT 0 to 6 ms; a11y, best practices, SEO 100 everywhere. Run-to-run range was zero to 0.1 s. Lab only, localhost, Lighthouse simulate, `node server.ts` on 3300 |
| **F-06** | **Fixed** | See "RV9-03 evidence map" below | web unit tests 127/127; `docs.spec`/`site.spec` green in 3 browsers |
| **Deadline precedence** (integrator) | **Done** | `/docs/reliability` (retry-budget table, timeline note "Deadline before lease") and `/docs/concepts` (lease paragraph, failure_class table) state: deadline wins when both passed; timeout not longer than the lease TTL makes a crash `deadline_exceeded`/`timeout`; the lease-expiry alert counts both outcomes | `git show main:docs/architecture/state-machine.md` E8 and the failure-class table; ADR-0003 text was not separately cited |
| **F-08** | **Done, 3 browsers green** | See `docs/testing/evidence/web/browser-matrix.md` | Final run: chromium 213 passed, firefox 213, webkit 213, each 58 skipped (opt-in), 0 failed, exit 0. Axe: 116 scans per browser, 0 serious or critical |
| **F-09** | **Measured, proposed, not frozen** | `perf-console.spec.ts` (opt-in), per-browser raw JSON, `console-budgets.md` with hardware, dataset, limits and proposed budgets | See "Console numbers" |
| **F-20** | **Fixed** | `/docs/quickstart` gains "Try the console without GitHub": the harness path, its prerequisites and ports, mock OAuth, that it tears itself down (not interactive), and that `--headed`/`--ui` were not exercised. Verified/unverified blocks updated from AUDIT A4 to A9 | `apps/web/e2e/stack/stack.ts`, README, AUDIT-M9-RV-01 §1a |

## RV9-03 evidence map (which file backs each changed statement)

| Page and statement | Evidence file |
|---|---|
| Changelog: M8 merged, local-only, CI "never been run on GitHub", nine alerts, no Alertmanager, RPO/RTO measured on drill stack | `CHANGELOG.md`, `docs/coordination/handoffs/HANDOFF-M8-BE-01.md`, `docs/testing/evidence/m8/**`, `docs/testing/release-gates.md` (CI NOT RUN) |
| Changelog: audit NO-GO recommendation, no engine defect | `docs/coordination/reviews/AUDIT-M9-RV-01.md` §5 |
| Changelog: ops fixes after review (hourly retention, Caddy 405 and non-root, network scoping, bounded scrape, rate-limit refusal, node-exporter alert), "proven locally only" | `docs/coordination/handoffs/HANDOFF-M9-BE-02.md` (main `166b581`) |
| Deployment: production example verified locally only (TLS 1.3, gRPC TLS, isolation, hardening, fail-closed, no secrets in env) | `docs/operations/deployment.md` "Local proof", `docs/testing/evidence/m8/production/` |
| Deployment: backup 0.26 s, restore ready 0.22 s later, 12 attempts `lease_expired`, RPO 2.2 s, RTO 14.1 s / 71.8 s | `docs/operations/backup-restore.md`, `docs/testing/evidence/m8/backup-restore/drill.txt`, HANDOFF-M8-BE-01 |
| Deployment: ten alert rules, nine induced, incident runbook, no Alertmanager | `infra/monitoring/alerts.yml` (main), `docs/operations/incident-runbook.md`, `docs/testing/evidence/m8/alerts/`, HANDOFF-M9-BE-02 (rule test only for the tenth) |
| Deployment and reliability: retention hourly in the production example (31-day-old job deleted, API 410); nothing runs it in the local stack unless scheduled | `docs/operations/deployment.md` (main), HANDOFF-M9-BE-02 RV9-02 row. The 30-day deletion no longer reads as unconditional (RV9-02 overclaim) |
| Deployment: migration drill 0002 to 0004; rollback is restore | `docs/operations/upgrades.md`, `docs/testing/evidence/m8/backup-restore/upgrade-drill-0002-to-0004.txt` |
| Deployment, quickstart, README-derived: clean clone on macOS under `$HOME` and Linux arm64 VM; Colima mounts only `$HOME`; amd64 not run | AUDIT-M9-RV-01 A3 to A5, F-11; `README.md` |
| Reliability, home, architecture: real worker kill (28 s after SIGKILL, 12/12 succeeded; M8 w3-kill 4 attempts, slowest 34.1 s) | AUDIT-M9-RV-01 A7; `docs/testing/benchmarks.md` (AC-18 kill section); `docs/testing/evidence/m8/bench/` |
| Reliability: DB outage (admissions 503) | AUDIT-M9-RV-01 G3 (TestAC08 ran); `docs/testing/evidence/m8/alerts/README.md` (stop postgres) |
| Reliability: control plane restart | AUDIT-M9-RV-01 A8; M4 curl guide (already cited) |
| Reliability, home, architecture: measured numbers (115 to 280 jobs/s, p95, admission p99), hardware, "not capacity claims" | `docs/testing/benchmarks.md` (environment and AC-19 tables) |
| Reliability: deadline before lease | `docs/architecture/state-machine.md` E8 (main) |
| Security: threat model and ASVS mapping exist as drafts | `docs/security/threat-model.md`, `docs/security/verification.md` |
| Security: shared auth-failure bucket behind a proxy | AUDIT-M9-RV-01 F-13 |
| Security: dependency scans (`govulncheck`, `pnpm audit`, `gitleaks` 0; OSV and SBOM not run; workflow never ran) | HANDOFF-M8-BE-01 "Supply-chain scans"; `docs/testing/release-gates.md`; REVIEW-M9-RV-02 C7 |
| Security: no independent re-review of M9 web hardening or M8 example | Corrected by REVIEW-M9-RV-02 (F-19 closed for scope). **The security page still says "not had an independent re-review"; REVIEW-M9-RV-02 now exists in the main checkout but is uncommitted there, so I did not cite it as a repo file.** The integrator should change that sentence when the review is committed |
| Security: reporting channel | `SECURITY.md`; AUDIT-M9-RV-01 §2 (private reporting only at publication, OD-19) |
| Quickstart: recovery drill timings, stop commands | AUDIT-M9-RV-01 A7, A9; `README.md` |
| Quickstart: console only via the E2E harness, mock OAuth | `apps/web/e2e/stack/stack.ts`, `README.md`, AUDIT-M9-RV-01 F-20 |
| Quickstart/home: Chromium 211 passed (audit) | AUDIT-M9-RV-01 G8 |
| Home FAQ and architecture "Browser" row | as above; row text now says mock sign-in server, TLS proven once locally |

## Console numbers (full detail: `docs/testing/evidence/web/console-budgets.md`)

Hardware: the Apple M4 above. Stack: production Next build, real Go control plane and two workers, PostgreSQL 18 `fsync=off`, loopback, headless, no throttling. Dataset: 3,000 jobs, 100 seeded worker rows. p50 / p95 / max in ms, Chromium (Firefox and WebKit are in the file and the JSON):

| Measurement | n | Chromium | Firefox p95 | WebKit p95 |
|---|---|---|---|---|
| Jobs list response via BFF (10 filter/sort combos) | 200 | 2.9 / 3.9 / 12.4 | 5 | 5 |
| Filter or sort to painted table (includes Playwright overhead) | 40 | 25 / 41 / 43 | 86 | 43 |
| Route transition to data ready (6 routes) | 60 | 62 / 81 / 82 | 62 | 71 |
| Overview topology with 100 workers painted | 15 | 63 / 76 / 76 | 75 | 89 |
| Workers page, 100 rows | 15 | 68 / 70 / 70 | 89 | 64 |
| Live: commit to "Jobs changed." notice | 15 | 724 / 751 / 751 | 704 | 755 |
| Live: job finished on server to Succeeded on its detail page | 12 | 1162 / 1916 / 1916 | 1973 | 1747 |

Proposals (not frozen, OD-06), each with a check method, are in the file: jobs query p95 under 250 ms, filter/sort painted under 400 ms, route data-ready under 600 ms, 100-worker pages under 800 ms, jobs-changed notice under 3 s, detail terminal state under 5 s. Bundle size was **not measured** and has no proposal. Finding: the workers list asks for `limit: 100` and has no pagination control.

## Test and build results (final source, exit codes)

| Command | Result |
|---|---|
| `pnpm --filter web lint` | exit 0 |
| `pnpm --filter web typecheck` | exit 0 |
| `pnpm --filter web test` | exit 0, 127 passed (14 files) |
| `pnpm --filter web build` | exit 0, route table above |
| `pnpm --filter web e2e:chromium` | exit 0, 213 passed, 58 skipped |
| `pnpm --filter web e2e:firefox` | exit 0, 213 passed, 58 skipped |
| `pnpm --filter web e2e:webkit` | exit 0, 213 passed, 58 skipped |
| docs, site, static-public, routes specs after the last wording edit (rebuilt) | 67 passed in each of the three browsers |

`pnpm --filter web e2e` runs the three projects in order (each starts its own stack; they must not run together because specs share fixed users and ports 55440/18180/19190/19191/19300/3100). The 58 skipped specs are opt-in: screenshots (54) and perf (4). Chromium axe evidence regenerated with no diff.

## Files changed

- Web app: `apps/web/src/app/not-found.tsx`, `apps/web/src/app/[...unmatched]/page.tsx` (new), `apps/web/next.config.ts`, `apps/web/src/app/(public)/page.tsx`, `apps/web/src/app/(public)/architecture/page.tsx`, `apps/web/src/content/commands.ts`, `apps/web/content/{changelog.md,docs/{concepts,deployment,quickstart,reliability,security}.md}`.
- UI package: `packages/ui/src/route-focus.tsx` (waits for a closing dialog), `packages/ui/src/styles.css` (`.timeline li` wraps UUIDs).
- Tests: `apps/web/playwright.config.ts`, `apps/web/package.json` (scripts), `apps/web/e2e/{axe-report,behaviors.spec,console-session-hardening.spec,docs.spec,helpers,site.spec,static-public.spec,perf-console.spec}.ts`, `apps/web/e2e/stack/stack.ts` (`CLUSTERX_E2E_RATE_MUTATIONS` override), `apps/web/src/__tests__/ui.test.tsx`.
- Evidence: `docs/design/evidence/lighthouse-m9.{md,json}`, `lighthouse-m7.md` (note); `docs/testing/evidence/web/{browser-matrix.md,console-budgets.md,console-perf-{chromium,firefox,webkit}.json,axe-{firefox,webkit}.{md,json},next-build-route-table.txt}`.

## What the new browsers found (summary; details in browser-matrix.md)

Real product fixes: (1) the job event timeline overflowed 320 px in Firefox because UUIDs after a hyphen never wrap (fixed in CSS); (2) focus after a client route change could lose to a closing dialog's focus restore in Firefox under load (route focus now waits for no open dialog). Platform behaviour, tests adapted: Safari does not focus buttons on click or tab to links by default; Firefox rejects clipboard permissions; WebKit reports cancelled prefetches as page errors. A Firefox content-process crash reproduces on a bare page with a modal `<dialog>` and is not the app.

## Limits and open items

1. **Chromium `/auth/end` `ERR_ABORTED`** about 1 run in 20 under load: cause not found; the test waits for `networkidle` and retries once. Not excluded as a product race.
2. **Firefox overflow fix is unproven by volume**: the flake was about 1 in 3 full runs; 6 of 6 targeted runs and the final full run passed.
3. Safari, Edge, branded Chrome, mobile devices: **not tested**. Manual screen-reader, keyboard-only, zoom and forced-colors walkthroughs (F-07): **NOT RUN**.
4. Perf runs for Firefox and WebKit predate the CSS and route-focus changes; they do not exercise those code paths.
5. Real GitHub OAuth, TLS and production cookies in a browser: NOT RUN (mock OAuth only). Reference host and amd64: NOT RUN. GitHub CI: never run.
6. Lighthouse was run against `node server.ts` with no control plane behind it (public pages only).
7. Console budgets: polling-fallback staleness (stream down) and bundle size not measured.
8. Workers list shows at most 100 workers (no pagination); a decision, not fixed here.

## Merge notes and integration needs

1. `CHANGELOG.md` (repo root, outside this lane) still says in its M5 line "DB-outage and `kill -9` recovery tests" in a way RV9-03 flagged as inconsistent with the old web page; the web page now agrees with it. Check the root file separately.
2. `docs/design/motion-and-assets.md` font rows (AUDIT F-16) were not touched (not in this ticket).
3. When REVIEW-M9-RV-02 is committed, change the sentence in `/docs/security` that says the M9 web hardening and the M8 example have had no independent re-review (RV9 closed F-19 for scope).
4. Add the merge commit and date to the "M9 audit fixes (web lane)" changelog entry if you want it to match the other entries.
5. CI: `pnpm --filter web e2e` now runs three browsers; the CI job needs `playwright install --with-deps firefox webkit` and takes roughly three times as long. I did not edit `.github/workflows` (not in my lane).
6. No push, merge, publish or deploy was done. Cleanup: all harness stacks stopped, ports 55440/18180/19190/19191/19300/3100/3197/3300 verified free at the end; compose project `clusterx` was never touched.
