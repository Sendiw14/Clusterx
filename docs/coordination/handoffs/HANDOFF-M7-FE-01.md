# HANDOFF — TASK-M7-FE-01 (public site, documentation, isolated demo: craft pass)

- **Task:** `docs/coordination/tasks/TASK-M7-FE-01.md`. Requirements: FR-WEB-01, FR-WEB-05..07, FR-DEMO-01, NFR-A11Y-01, NFR-RESP-01, NFR-PERF-01; AC-14, AC-21, AC-22, AC-23, AC-24 (lab), AC-28, AC-29.
- **Author:** frontend lane, model `claude-sonnet-5-5` as given in the session. Effort `high` was requested; the effective effort is not observable from inside the session (MW-04: UNVERIFIED).
- **Branch:** `m7/frontend`, worktree `.worktrees/frontend`, base main `4ba69c1`. No push, no merge. The session was interrupted twice (usage limit, network error); the work resumed from the commits below.
- **Contract:** `api/openapi.yaml` is `1.0.0-draft.5`, last changed by `8fddad7` (file SHA-256 `6b4b9ac8174cfbb4677e8a07ddfe35c85d1653fecd478a04a5a9e1cab7e4646a`). Nothing under `api/`, `packages/sdk-ts`, SQL, proto, `infra/`, `.github/` or `docs/{product,architecture}` was touched (checked with `git diff --name-only 4ba69c1..HEAD`). The API reference reads the spec at build time, so draft.6 needs no code change; it changes only the build output.
- **Implemented / Integrated:** **implemented and tested** against the production build with Chromium. The public site, docs and demo need no backend. **Not integrated** in the sense of live E2E: there is nothing live to integrate for these routes (the demo is fixture-only by design). The documented commands were verified by the integrator, not by this lane (see "Docs claims" below).

## Commits

| SHA | Content |
|---|---|
| `404c2cf` | Home truthfulness pass: verified quickstart from `src/content/commands.ts`, per-layer build-status chips, limitation FAQ, GitHub CTA replaced |
| `8e85d5c` | `/architecture`: topology SVG with title, description, layer list, connections table, timing defaults, trade-offs, failure domains, sources |
| `fc5e439` | Docs framework (sidebar, on-this-page, prev/next, anchors, copy buttons, build-time search), seven docs pages, API reference generated from `openapi.yaml`; adds `marked 18.2.1` and `yaml 2.9.1` |
| `df5bbfe` | `/changelog` from merged milestones; demo recovery replay and retry lineage; docs, site and demo tests |
| `eab11df` | Hero plate without script dependency, font preloads, favicon, demo times in UTC, M7 screenshots, Lighthouse and axe evidence, motion addendum |
| (this) | This handoff (a file cannot hold its own id; the SHA is in the return message) |

## Routes done

| Route | State |
|---|---|
| `/` | Hero with at-least-once statement and CTAs, labelled illustrative ledger, Atlas (8 steps, Play/Step/Reset, keyboard, no autoplay under reduced motion), three pillars, five-layer architecture with build status, worker-failure story, console preview **labelled "preview sample, not a capture of shipped software"**, verified quickstart with Verified and Not-yet-verified notices, limitations FAQ, CTA band, footer |
| `/architecture` | Layered topology (SVG plus text alternative), connections and credentials table, job and lease path, default timings, trade-offs, failure domains with evidence column, limitations, repo-relative sources |
| `/docs` | Index: guided first steps, concept map, all pages, evidence-marker legend; search field in the sidebar |
| `/docs/quickstart`, `concepts`, `workers`, `reliability`, `deployment`, `security` | Authored in `apps/web/content/docs/*.md`, rendered by `src/lib/docs/markdown.tsx` (no raw HTML) |
| `/docs/api` | Narrative from `content/docs/api.md` plus generated sections: contract and served status, security schemes, error codes and statuses, 33 operations with parameters, request fields, examples, responses, status chips, and every component schema |
| `/docs/search-index.json` | Static build-time index (134 entries: every docs section, every API operation, every schema). No external service |
| `/changelog` | "Unreleased (pre-1.0)": merged milestones M1 to M4 with merge SHAs and dates; no version number, no release date |
| `/demo` | Overview, jobs (with filters and dead-letter view), job detail (failed job `demo-job-0003` links to its retry `demo-job-0013` and back), workers, **recovery replay** (7 steps, visitor-driven), config. Operator and viewer views. All times UTC. Fixture provider only |

## Verification (all run in the worktree, Node 26.13.1, pnpm 9.15.9, final tree before this handoff)

| Command | Exit | Result |
|---|---|---|
| `pnpm --filter web lint` | 0 | 0 errors, 0 warnings |
| `pnpm --filter web typecheck` | 0 | |
| `pnpm --filter web test` | 0 | 9 files, **64 tests passed** (14 new: renderer, docs honesty, link resolution, API generation, search, UTC) |
| `pnpm --filter web build` | 0 | Static: `/`, `/architecture`, `/changelog`, `/demo`, `/docs`, all docs pages, `/docs/search-index.json` |
| `pnpm --filter web e2e` | 0 | **116 passed, 50 skipped** (the skipped are the two opt-in screenshot specs) |
| `SCREENSHOTS_M7=1 pnpm --filter web exec playwright test e2e/screenshots-m7.spec.ts` | 0 | 36 screenshots |

What the E2E covers beyond M3: every docs route (sidebar current page, on-this-page anchors resolve, prev/next), on-this-page scroll marker, search (keyboard only, `/` shortcut, Esc, no-results message, finds an API operation, axe with the popup open), search index file, copy button copies the exact text and announces, mobile docs menu (search plus nav, focus restored), API page lists every `operationId` from the spec and the spec's version, served and not-served chips, honesty copy checks (no testimonials, uptime figures, "production-ready", exactly-once guarantee), **overflow at 320/390/768/1024/1440/1920 on all 13 public routes**, **axe at 390 and 1024 on all public routes** (320 and 1440 from M3), **link checker** (every internal link on every public page returns 200 and every `#fragment` exists on its target), Atlas Play/Step/Reset by keyboard and no autoplay under reduced motion, no running animation under reduced motion on `/`, `/architecture`, `/docs`, `/docs/api`, `/changelog`, `/demo`, no scroll hijacking (wheel moves the document by the native amount), architecture diagram text alternative, changelog has no invented version, demo recovery replay and retry lineage, demo network trap (same-origin GET only, no `/api`, no `/bff`, no WebSocket, no cookie, no Authorization header) extended to the new screen.

**axe** (`docs/design/evidence/axe-m7.md` and `.json`): 88 scans, critical 0, serious 0, moderate 0, minor 0. Automated only; **no WCAG conformance is claimed**. `axe-m3.*` is restored to its M3 content.

## Lab Web Vitals (`docs/design/evidence/lighthouse-m7.md`, `.json`)

`npx lighthouse@13.6.0` against `next start` on localhost, default simulated throttling, Chrome for Testing, one run per cell. **Field data is unavailable. INP was not measured** (it is a field metric).

| Page | Preset | Perf | LCP | CLS | TBT | A11y | Best practices | SEO |
|---|---|---|---|---|---|---|---|---|
| `/` | mobile | 100 | 1.1 s | 0 | 10 ms | 100 | 100 | 100 |
| `/` | desktop | 99 | 0.9 s | 0.001 | 0 ms | 100 | 100 | 100 |
| `/docs` | mobile | 100 | 0.8 s | 0 | 0 ms | 100 | 100 | 100 |
| `/docs` | desktop | 100 | 0.2 s | 0 | 0 ms | 100 | 100 | 100 |
| `/demo` | mobile | 100 | 0.9 s | 0 | 10 ms | 100 | 100 | 100 |
| `/demo` | desktop | 100 | 0.2 s | 0.021 | 0 ms | 100 | 100 | 100 |

The first run found a real problem, fixed in `eab11df`: on `/` mobile the LCP was 4.1 s because the hero ledger sat inside the scroll-reveal wrapper and stayed invisible until JavaScript ran (then 3.6 s waiting on fonts, then a favicon 404). Not measured: `/architecture`, `/changelog`, the other docs pages, the generated API page (the largest document), client JS bytes.

## Screenshots

`docs/design/screenshots/m7/`: 18 pages at 390 and 1440 (36 PNG, about 19 MB): home, architecture, docs index, quickstart, concepts, api (first 9,000 px only, Chromium caps full-page height), workers, reliability, deployment, security, search open, changelog, demo overview, jobs, failed job, retry job, workers, recovery replay. Reduced motion, so reveal animations show their final state. They show shipped components.

## Docs claims that are "not yet verified" (every one is marked in the docs themselves)

1. **Quickstart commands** were run by the **integrator** (2026-11-18, Colima): `cp .env.example .env`, `./infra/compose/bootstrap.sh` (exit 0, 3 live workers), `POST /api/v1/jobs` with `hash_sha256` and `fail_n_times`. This lane ran none of them. The exact `curl` text, the read-back calls, the health calls and the observed responses are taken from the executed **M4 curl guide against built binaries on port 18080**; on compose the port is 8080. Marked as such.
2. Not run, marked "not yet verified": a clean-checkout run by someone other than the integrator; a Linux host; **recovery drill with a stopped worker container** (steps given as specification only; what was observed, graceful SIGTERM of host binaries, is stated separately); adding workers beyond three and any throughput number; commands to stop and clean the stack (none given).
3. `/docs/reliability`: the recovery timeline (t+0 to t+43 s) is derived from the specification and default timings, not measured. Not run (stated in the page): real process kill, three OS processes under Compose with 1,000 jobs, database crash durability, backup and restore, DB outage behaviour. The 1,000-job figures quoted are the in-process test output from HANDOFF-M4-BE-01 and are labelled "not a throughput claim".
4. `/docs/deployment`: **the whole production section is unverified** (TLS, secrets, networking, backups, restore, upgrades, RPO and RTO, monitoring). RPO and RTO are stated as not set.
5. `/docs/workers`: there is **no published module version**, so no `go get`; the example module uses a `replace` directive; the Go snippets are copied from `examples/basic-jobs` and compile in the repository (backend lane ran the tests, the integrator has not re-run them for this page); an end-to-end test of a user's own handler is marked unverified because the catalogue is compiled into the coordinator.
6. `/docs/security`: GitHub sign-in, sessions, CSRF and the BFF are **specified, not built** (said so on the page). No formal threat model, no independent review, no rate limiting, **no published security contact or disclosure policy** (this is an owner decision; the page says so rather than invent an address).
7. `/docs/api`: served status comes from the spec's own sentence ("M4 serves ..."); if draft.6 rewords it, the page falls back to "Served status unknown" and marks nothing. The "Executed in the M4 curl guide" chips come from a fixed list in `src/lib/docs/openapi.ts` (`EXECUTED_IN_CURL_GUIDE`) that must be updated by hand when more operations are executed. `getWorker` is served but was not curl-ed in the guide, so it carries no such chip. Examples in the spec use handler names (for example `thumbnail.render`) that the compiled catalogue does not contain; the page says examples are illustrative.
8. Home and architecture: the layer status chips ("Implemented, tested" versus "Specified, not built") are this lane's reading of the handoffs, not an independent audit. The console preview is a hand-built HTML sample, labelled as such, **not** a capture of shipped software (a real capture is blocked until the console is connected to live data in M6).
9. Timing defaults shown (lease 30 s, heartbeat 10 s, and so on) are read from `apps/control-plane/internal/config/config.go` and ADR-0003, and labelled starting values, not tuned.

## Honesty decisions worth reviewing

- **Atlas, hero ledger and story timeline were internally inconsistent** (a renewal at t+12 s followed by a lease that expired at t+30.4 s). Times were shifted to t+42.4, 44.9, 46.1 and 53.7 s so the renewal extends the lease. Labelled "Illustrative, scripted".
- The M3 home quickstart said the stack included "the console" and used a `thumbnail` handler; neither is true (compose has no web service; the handlers are `hash_sha256`, `sleep_ms`, `fail_n_times`). Replaced.
- The home GitHub CTA was replaced by "Read the architecture". The **footer still links to `https://github.com/Sendiw14/Clusterx`** (from M3, matches the Go module path). Repository publication is owner-gated (MASTER section 16), so that link may be dead; internal-link checking does not cover it. Decide before launch.
- The "recovery replay" last step (late report rejected) is **scripted**, because the fixtures hold no diagnostic record. The step is labelled "Scripted, not a fixture record".
- The demo now shows all times in UTC (a `ForceUtcContext` in `@clusterx/ui`), matching its caption. The M3 E2E that used the demo to test the time-zone preference was rewritten (demo stays UTC even when a zone is set); the preference itself is now covered by `src/__tests__/timestamp-zone.test.tsx`. No console page can show a timestamp without data until M6.

## Changed files (all inside allowed paths)

- `apps/web/**`: pages `(public)/page.tsx`, `architecture`, `changelog`, `docs/**` (layout, index, seven pages, `search-index.json/route.ts`); `src/lib/docs/{content,markdown,openapi,search-index}`; `src/features/{docs,docs-search,on-this-page,api-reference,architecture-diagram,home.css}`, `features/demo/{tour,replay}.tsx`, `features/job-detail.tsx` (lineage rows), `data/fixtures/data.ts` (job 13, retry of job 3); `src/content/commands.ts`; `content/docs/*.md`, `content/changelog.md`; `src/app/icon.svg`, `layout.tsx` (two font preloads); tests `e2e/{docs,site,screenshots-m7}.spec.ts`, `e2e/{behaviors,demo-isolation}.spec.ts` (edited), `src/__tests__/{docs.test.tsx,timestamp-zone.test.tsx}`; `e2e/axe-report.ts` (writes `axe-m7.*`).
- `packages/ui/src`: `public-shell.tsx` (docs sidebar search slot, page order), `styles.css` (docs layout, search, API reference, concept map), `timestamp.tsx` (`ForceUtcContext`).
- `docs/design/`: `motion-and-assets.md` (section 6 addendum), `evidence/{axe-m7,lighthouse-m7}.*`, `screenshots/m7/*`.
- **Lockfile:** `pnpm-lock.yaml` changed by `pnpm add` for the `web` importer only. Two direct dependencies added with exact pins: `marked 18.2.1` (Markdown lexer for docs; renders to React elements, never raw HTML) and `yaml 2.9.1` (OpenAPI parsing at build time). pnpm also re-keyed `vite`, `vitest` and two Vitest-related entries with an optional `yaml@2.9.1` peer; no version of any existing package changed.

## Integration needs and open items

1. Integrator: add `marked 18.2.1` and `yaml 2.9.1` to `docs/architecture/dependency-matrix.md` (read-only for this lane).
2. The build reads `../../api/openapi.yaml` and `content/` relative to `apps/web` (the working directory of `pnpm --filter web build`). A build run from another directory fails loudly rather than producing an empty reference.
3. Decide the GitHub URL in the footer (see above) and whether to publish a security contact (owner decisions).
4. When more operations are executed in a curl guide (M5), update `EXECUTED_IN_CURL_GUIDE`. When draft.6 lands, run `pnpm --filter web test` (it asserts the generated operation list equals the spec's `operationId`s).
5. M6: replace the home console preview with a real capture; the docs Quickstart will need a console step; `/docs/security` must be rewritten when sessions ship.
6. Review owners: independent visual and accessibility reviewer; Opus to check the permission accuracy of the security page and the failure-mode evidence column on `/architecture` and `/docs/reliability` against the M4 tests.

## NOT RUN / limitations

- Firefox, WebKit, Safari, Edge: NOT RUN (Chromium only).
- Manual NVDA and VoiceOver walkthroughs, 200 and 400 percent zoom emulation: NOT RUN. Reflow was tested only through viewport widths (320 and up).
- Field Web Vitals and INP: unavailable. Lighthouse is one lab run per cell on localhost.
- The docs search is verified in Chromium with keyboard and mouse only; no screen-reader test of the combobox pattern.
- The diagram on `/architecture` is hidden below 480 px by design; the layer list and connections table carry the content there.
- Screenshots show Chromium rendering with real fonts under reduced motion.
- CSP still uses `script-src 'unsafe-inline'` (inherited from M3). The docs search fetches `/docs/search-index.json` from the same origin.
