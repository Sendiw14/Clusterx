# HANDOFF: TASK-M9-FE-03 (re-audit fixes in web copy, bundle size, chart ticks)

- **Author:** frontend lane, model `claude-sonnet-5-5`, requested effort high. Effective effort and Ultracode: UNVERIFIED from inside the session (MW-04).
- **Branch:** `m9fix3/frontend` (worktree `.worktrees/frontend`), base main `da506db`. Not pushed, not merged.
- **Contract:** `api/openapi.yaml` `1.0.0-draft.7`, last changed in `c4c3eb0`, blob SHA `4eb0f6800764ce3402b41f29aa329ff260b70e2f`. Not changed. No SQL, OpenAPI or proto edits.
- **Source:** AUDIT-M9-RV-03 (RV10-01, RV10-02, RV10-06, F-09) and an integrator addition (duplicate y-axis label).
- **Environment:** macOS arm64 (Apple M4), Node 26.13.1, pnpm 9.15.9, PostgreSQL 18.7 first on PATH, Playwright 1.65.1 (Chromium, Firefox, WebKit).
- **Commits:** see "Commits" at the end (filled in after the commit).

## Per item

| Item | Status | What changed |
|---|---|---|
| **RV10-01** | Fixed in `apps/web`; **two files outside my lane still carry the old claim** | `quickstart.md` (verified block and prerequisites), `deployment.md`, `src/content/commands.ts` now say: clean clone verified in a **Linux arm64 Ubuntu VM** (AUDIT-M9-RV-01 A4 to A9); on macOS a clean clone **outside `$HOME` failed** (A3, Colima mounts only `$HOME`); a macOS clean clone under `$HOME` was **not run**; the only macOS evidence is the integrator's 2026-11-18 run from a working checkout. A unit test fails if the old phrasings return. **Integrator action:** `README.md:40` ("Verified from a clean clone on macOS arm64 with Colima") and `docs/testing/release-gates.md:18` ("macOS under `$HOME`") are outside my allowed paths and still say it. |
| **RV10-02** | Fixed | `lib/docs/openapi.ts`: the draft.6 statement "every operation is routed" is read first, so every operation is `served: true`; the M4-era sentence is used only for a draft that lacks the draft.6 line. `features/api-reference.tsx`: the callout says every operation is routed and that routed is not the same as exercised (the "Executed in the M4 curl guide" chips stay, they are record-based). `docs.test.tsx`: the label test is rewritten, and a new check **parses `apps/control-plane/internal/api/v1.go`** (every `route(mux, "PATH", ... http.MethodX`) and requires its set of method and path pairs to equal the spec's `/api/v1` operations in both directions. `e2e/docs.spec.ts:117` now asserts `cancelJob` shows "Served in this build" and that no "Not served yet" text exists. `/docs/api` header warning and `/docs/quickstart` `not_found_route` row reworded (unknown path, not "unserved route"). `/docs/concepts`: the sentence that cancel and retry are not served is removed. `/docs/workers`: states that handler log capture shipped in M5 (`job.Log`, a `*slog.Logger`, last 16 KiB reported as `log_tail`, secret-like keys redacted, kept 7 days, other output not captured), and the `sdk.Job` field list now includes `Log`. A unit test blocks the old sentences. `/metrics` and `/health/*` are not in `v1.go`; they remain `served` as before (health in `server.go`, metrics on the internal listener); the parse test covers `/api/v1` only. |
| **RV10-06** | Fixed | `/docs/reliability` Non-guarantees: "No promised performance numbers. Measured numbers from one development setup are at the end of this page; they are not capacity claims." |
| **F-09 bundle size** | Measured, budgets proposed, not frozen (OD-06) | `docs/testing/evidence/web/bundle-size.md` and `bundle-size.json`, produced by new `apps/web/scripts/bundle-size.mjs` from `.next/diagnostics/route-bundle-stats.json` (raw, gzip -9, Brotli -11; the script fails if its raw total differs from Next's own figure). Numbers below. |
| **Chart ticks** (integrator) | Fixed | `features/chart.tsx`: new exported `axisTicks(max, unit)`. Counts use unique whole numbers (max 0 or 1 gives 0 and 1; max 2 gives 0, 1, 2; max 3 gives 0, 2, 3); seconds keep halves. Before: ticks `max*0.5` rounded, so max 1 drew "1" twice (0.5 rounds to 1). `src/__tests__/chart-ticks.test.tsx`: ticks for 0 to 5, plus a render check that axis labels are unique for peaks 0, 1, 2, 3. Screenshots `docs/design/screenshots/m6/overview-1440.png` and `overview-390.png` regenerated with `CAPTURE_M6=1` on the live stack (this spec writes all M6 images, each with new ids and timestamps; I reverted every other file it changed, including `public/images/console-jobs.png` and `axe-m6.*`, and kept only the two overview images, which contain the chart). In the new 1440 image the chart shows a single "1" label at the top; the "0" label sits below the 900 px fold of that fixed-size capture, so the image shows the duplicate is gone but not the whole axis. The unit render test is the proof for the whole axis. |

## Bundle size (full detail in `docs/testing/evidence/web/bundle-size.md`)

First-load client JS, KB (1000 B):

| Group | Raw | gzip -9 | Brotli -11 |
|---|---|---|---|
| public (`/architecture`, `/changelog`; `/` is 519.1 / 154.3 / 132.3) | 498.2 | 148.3 | 127.1 |
| docs (`/docs` 505.5 / 151.0 / 129.5; the 7 pages 506.7 / 151.7 / 130.0) | 506.7 | 151.7 | 130.0 |
| demo (`/demo`) | 553.0 | 164.8 | 141.5 |
| console (all 11 `/app/*` routes share one chunk set) | 593.3 | 177.8 | 153.2 |
| `/login`, not-found | 498.2 | 148.3 | 127.1 |

Proposed gzip ceilings (proposal, not frozen): public and docs 170, demo 190, console 205. Not measured: later dynamic loads, CSS, fonts, images, served (Caddy) transfer sizes. No CI check was added (ceilings not agreed). What makes up the 498 KB floor was not investigated.

## Commands and results

| Command | Exit | Result |
|---|---|---|
| `pnpm --filter web lint` (run as `pnpm lint` in `apps/web`) | 0 | no warnings after removing an unused import in the new script |
| `pnpm typecheck` | 0 | |
| `pnpm test` | 0 | all web unit tests pass, including the new chart-tick, v1.go-parity and stale-claim tests |
| `pnpm build` | 0 | route table unchanged from the audit's `○`/`ƒ` split (`docs/testing/evidence/web/next-build-route-table.txt` not regenerated, no route change) |
| E2E `docs`, `site`, `static-public`, `routes` specs, Chromium | 0 | 67 passed |
| Full `pnpm e2e` (Chromium, Firefox, WebKit in sequence) | 1 (WebKit) | Chromium 213 passed, 0 failed; Firefox 213 passed, 0 failed; WebKit **212 passed, 1 failed**, each with 58 skipped (opt-in specs). The failure: `routes.spec.ts:10` `/ @ 320` in WebKit, axe `color-contrast` serious, 3 nodes. It did not reproduce: 78 WebKit runs of every `@ 320` route (6 repeats) all passed, and a second full WebKit project run gave **213 passed, 0 failed, 58 skipped, exit 0**. I did not find the cause; I changed nothing on `/`. Treat it as an unexplained intermittent WebKit contrast result under load, not as a pass-or-fail for the page. |

## Claim to evidence

| Claim | Evidence |
|---|---|
| Every operation in the API reference shows "Served in this build" | `docs.test.tsx` "marks every operation served..."; `docs.spec.ts` cancelJob assertion |
| That claim matches the server | `docs.test.tsx` parses `apps/control-plane/internal/api/v1.go` and compares to the spec both ways. It proves routes are registered, not that each was exercised |
| Cancel and retry are served | `v1.go:99-100`; E2E `console-jobs` (cancel and retry) |
| Handler log capture shipped | `packages/sdk-go/handler.go:19-22` (`Job.Log`), `logcap.go` (`MaxLogTailBytes = 16384`), `worker.go:402,429` (`LogTail`) |
| Clean clone verified on Linux arm64 only | AUDIT-M9-RV-01 A3 (failed) and A4 to A9; AUDIT-M9-RV-03 RV10-01 |
| Chart labels unique for small maxima | `chart-ticks.test.tsx`; `overview-1440.png` |
| Bundle numbers | `bundle-size.json`, reproducible with `scripts/bundle-size.mjs` |

## Limitations and integration needs

- Outside my lane, still stale: `README.md:40`, `docs/testing/release-gates.md:18` (RV10-01). RV10-03 (traceability and release-gates) and RV10-04/05 are integrator items, untouched.
- `content/docs/security.md` and the web changelog still say the M8 production example fixes are "not yet re-reviewed". AUDIT-M9-RV-03 says it is that re-review for RV9-01..11. I did not change this: the ticket does not name it and I would be citing an uncommitted-at-base audit as a basis. Integrator decision.
- The `Executed in the M4 curl guide` chip still marks only the 11 operations that guide ran; the other 17 are routed but have no per-operation "executed" label.
- `/docs/api` still labels `prometheusMetrics` served as before; `/metrics` answers on the internal listener only.
