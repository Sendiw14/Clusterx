# HANDOFF — TASK-M2-FE-01 (M2 design system, route designs, Execution Atlas, motion)

- **Task:** `docs/coordination/tasks/TASK-M2-FE-01.md`. Requirement IDs: FR-WEB-01..08, FR-DEMO-01, NFR-A11Y-01, NFR-RESP-01, AC-21..AC-23 (design inputs); RV01-10 (UI mapping only).
- **Author, actual model/effort, Ultracode availability:** frontend lane, model `claude-sonnet-5-5` as given in the session. Effort `high` was requested; the effective effort is not observable from inside the session (MW-04: UNVERIFIED). Ultracode: not used, availability not checked from inside the session.
- **Branch, commits:** `m2/frontend`, worktree `.worktrees/frontend`, base `efbcbd6`. No push, no merge.
  - `4209c7f` tokens.css, brand-and-tokens.md with measured contrast, contrast tool and evidence, mark.svg
  - `49c418b` console type steps strengthened (page title 28 px, section 18 px); responds to the integrator's flat-type-hierarchy note
  - `1f93b8f` six prototypes, build and render tooling, sprite, 26 screenshots, render evidence
  - `bc4b486` information-architecture, component-specs, states-matrix, execution-atlas, motion-and-assets, accessibility-plan
  - plus the commit containing this handoff file (the SHA is reported in the return message, because a file cannot contain its own commit id)
- **Changed files (all inside allowed paths: `docs/design/**`, `packages/ui/tokens/**`, this handoff):**
  - Docs: `docs/design/{brand-and-tokens,information-architecture,component-specs,states-matrix,execution-atlas,motion-and-assets,accessibility-plan}.md`
  - Tokens: `packages/ui/tokens/tokens.css` (token source only, no components)
  - Prototypes (built output): `docs/design/prototypes/{home,overview,jobs,job-detail,workers,login}.html`; sources in `docs/design/prototypes/_src/` (page templates, `shared.css`, `shared.js`, `sprite.html`, header, footer and console partials)
  - Assets: `docs/design/assets/{mark.svg,icons-sprite.svg}` (the sprite is generated)
  - Tools: `docs/design/tools/{contrast,build-prototypes,render,type-hierarchy}.mjs`
  - Evidence: `docs/design/evidence/{contrast-output.md,render-check.json,type-hierarchy.txt}`
  - Screenshots: 26 PNGs in `docs/design/screenshots/`
  - Not touched: `apps/**`, `api/**`, `packages/proto/**`, `packages/sdk-*/**`, root config, `docs/product/**`, `docs/architecture/**`, `.worktrees/backend`, the main checkout.
- **Contract SHA, generated code, migration impact:** contract `contract-m1-draft.4` = `c48b62f88ac76337f43a21e2adb08ae6dccc5ab2` (OpenAPI 1.0.0-draft.4). `git diff contract-m1-draft.4 HEAD -- api packages/proto` is empty at the base, and this branch changes neither. No generated code, no migration, no API or schema change. No new endpoint was invented; every data statement maps to an existing operation or to a recorded GAP.
- **Implemented / Integrated (distinguished):** **specified** (design docs, tokens) and **prototyped** (static HTML with illustrative data). Nothing is **implemented** in `apps/web`, nothing is **integrated** (no live API, no BFF), nothing is **tested** beyond the prototype checks below. Prototypes are not product code and are labelled "design prototype — illustrative data" in a visible bar on every page.

## Exact commands, exit codes, results

Environment: macOS, Node v26.13.1 (`/opt/homebrew/opt/node@26/bin` first on PATH), Playwright 1.65.1 installed in a scratch directory (`npm i playwright@1`, `npx playwright install chromium`; the command downloaded Chromium headless shell v1255, 100.8 MiB, as authorised by the ticket), Chrome Headless Shell 159.0.8154.6. The repository gained no `package.json` or dependency.

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `node docs/design/tools/contrast.mjs` | 0 | PASS: 73 required pairs, 73 pass, 0 fail; 5 documented rejections (see below) |
| 2 | `node docs/design/tools/contrast.mjs --inject docs/design/brand-and-tokens.md` | 0 | Table regenerated into the doc from `tokens.css` |
| 3 | `node docs/design/tools/build-prototypes.mjs` | 0 | 6 pages built, 0 external references in any output |
| 4 | `node docs/design/tools/build-prototypes.mjs --check` | 0 | Built outputs match `_src` (not stale) |
| 5 | `PW_MODULE=<scratch>/node_modules/playwright/index.mjs node docs/design/tools/render.mjs` | 0 | PASS: 26 screenshots written; checks passed (below) |
| 6 | `PW_MODULE=... node docs/design/tools/type-hierarchy.mjs` | 0 | Page title to body at least 1.66x on every prototype, section heading at least 1.29x |

`render.mjs` checks (evidence: `docs/design/evidence/render-check.json`): for each of the 6 prototypes at 320, 360, 390, 768, 1024, 1440 and 1920 CSS px (default state): no horizontal page overflow; zero requests that are not `file:`/`data:` (everything else is aborted and counted); zero console errors or page errors; exactly one visible `h1` and one `main`; the text "design prototype — illustrative data" present. Reduced motion (390 px, `reducedMotion: 'reduce'`): zero running animations on all six. Result: **PASS**, 0 failures, Chromium only.

During the work the checks found and I fixed: a 338 px overflow of home at 320 px (grid min-width), a 342 px overflow of job-detail at 320 px, a topology graph clipped at the right edge, mobile table captions and multi-part status cells misaligned, chart failures originally drawn on a rescaled axis (replaced by a truthful shared axis), and the console section heading ratio (see below).

### Contrast results summary (numeric, from `tokens.css`)

All text pairs are held to 4.5:1 with no large-text relaxation; non-text to 3:1. Selected measured ratios: ink on white 17.74, ink-2 on white 7.58, ink-3 on panel 5.16, accent on white 5.85, white on accent (primary button) 5.85, accent-ink on accent-tint 6.69, success-ink on white 6.09 and on its tint 5.37, warning-ink on white 6.04 and on its tint 5.35, danger-ink on danger-tint 5.84, focus ring on white 5.85, control border (line-strong) on white 3.51 and on panel 3.12, code ink on code background 14.48 (every code syntax token passes 4.5:1 on that background; full list in `contrast-output.md`).
**Two starting §9 tokens fail as text on white:** success `#0C8A63` = 4.35:1 and warning `#A96B08` = 4.38:1. They are kept for glyphs, bars and chart lines (pass 3:1) and paired with `success-ink #0A7050` and `warning-ink #8A5806` for text. Also rejected for stated uses: neutral `#64748B` as text on panel (4.23), disabled ink `#8A94A6` (3.06, WCAG-exempt for inactive controls, state also stated in text), decorative line `#E2E8F0` (1.23, never a control boundary).

### Responds to the integrator note: `flat-type-hierarchy`

Measured, not assumed. Computed styles (`docs/design/evidence/type-hierarchy.txt`): home hero `h1` 80 px against 17 px body is 4.71x at 1440 (42 px, 2.64x at 390); console page titles were 24 px against 14 px (1.71x). **The hook finding was real for one tier:** console section and card headings were 16 px against 14 px body (1.14x, weight only). Fix in commit `49c418b`: page title 28 px (2.00x), section and card titles 18 px (1.29x), KPI values 32 px; documented in `brand-and-tokens.md` section 4.3. The editorial hero was already above the threshold. If the hook still flags the static source, it is probably reading `var(--text-*)` tokens it cannot resolve, not the computed sizes; I cannot confirm the hook's method.

### Screenshot status

**RUN.** Rendered from the built prototypes with Chromium headless via `node docs/design/tools/render.mjs`, timezone UTC, network blocked except `file:`, scrolled through the page first so reveal animations have played. Saved in `docs/design/screenshots/`:

- 390 and 1440 px for each of the six prototypes: `home-`, `login-`, `overview-`, `jobs-`, `job-detail-`, `workers-` with `-390.png` and `-1440.png`
- State variants: `home-atlas-step5-1440`, `home-atlas-step8-1440`, `login-no-entitlement-390`, `overview-stale-1440`, `overview-first-use-390`, `jobs-empty-filter-1440`, `jobs-error-1440`, `jobs-stale-390`, `job-detail-failed-1440`, `job-detail-viewer-1440`, `job-detail-expired-1440`, `job-detail-unschedulable-390`, `workers-100-1440` (graph, 100 workers), `workers-none-1440`

Limits: the screenshots show system fallback faces (system serif, sans, mono), not IBM Plex or Newsreader, because no font file is bundled (see OQ-13). Only Chromium was used; **Firefox, WebKit and Edge: NOT RUN.** I visually inspected `home-1440`, `home-atlas-step5/8-1440`, `home-390` (viewport), `overview-1440`, `jobs-390`, `job-detail-failed-1440`, `job-detail-viewer-1440`, `workers-100-1440`; I did not individually inspect every other PNG.

## Deliverable map

| # | Ticket deliverable | Where |
|---|---|---|
| 1 | Brand, tokens, measured contrast, type, grid, spacing, shape, status and chart encodings, font licence | `brand-and-tokens.md`, `packages/ui/tokens/tokens.css` |
| 2 | Information architecture | `information-architecture.md` |
| 3 | Component specs (19 components, all of the §8 inventory) | `component-specs.md` |
| 4 | States matrix (24 routes plus gated and global, three roles) and RV01-10 UI mapping | `states-matrix.md` |
| 5 | Execution Atlas grammar, 1/10/100 clustering, illustrative vs live | `execution-atlas.md` |
| 6 | Motion matrix and asset inventory | `motion-and-assets.md` |
| 7 | WCAG 2.2 AA plan, screen-reader scripts, reflow plan, test widths | `accessibility-plan.md` |
| 8 | Prototypes: home (hero plus Atlas), overview, jobs (empty, error, stale), job-detail (attempts, lease, cancel and retry dialogs, 410), workers (topology and list toggle, no-worker state), login | `docs/design/prototypes/*.html` |
| 9 | Screenshots at 390 and 1440 | `docs/design/screenshots/` |

Prototype behavior that works offline: state switcher and role switcher in the label bar, Atlas Play, Pause, Step, Reset (plays once on first view, never under reduced motion), jobs filters with chips and the dead-letter preset, cancel and retry confirm dialogs with scripted outcome, command search (`Ctrl/Cmd+K`) with exact-ID lookup, worker topology at 1, 10 and 100 workers with zoom, pan, reset, keyboard node navigation and list synchronization, copy buttons, mobile drawers. Links to routes outside the set are inert and say so in a toast.

## Auth/security, UX/accessibility and compatibility review

- **Security:** no secrets, tokens or credentials appear. No real job IDs: IDs are illustrative 8-hex strings, and every prototype carries the label. No request is made by any prototype (network blocked in the render check; 0 external references in the built files). The GitHub mark was deliberately not used on the sign-in button to avoid a third-party asset. No cookies or storage are used by the prototypes.
- **Accessibility:** design measures are specified against WCAG 2.2 A and AA in `accessibility-plan.md`. **Measured:** contrast on tokens, reflow overflow at 7 widths, structure counts, reduced-motion animation count. **Not run:** axe, Lighthouse, any screen reader, keyboard walkthrough, 200 and 400 percent zoom, text-spacing and forced-colors tests, non-Chromium browsers. No conformance claim is made.
- **Backwards compatibility:** no API, schema, proto or app change. `route-inventory.md` and `consumer-requirements.md` are not edited; where the design adds a decision (for example `/login` noindex, topology default view, clustering threshold of 12) it is recorded in the design docs, not as a contract change.

## Known defects, warnings, deferred scope, limitations

- **OQ-13 typeface licensing is still an owner decision.** Recommended: IBM Plex Sans and IBM Plex Mono (SIL OFL 1.1, `github.com/IBM/plex`) and Newsreader for public display titles only (SIL OFL 1.1, `github.com/productiontype/Newsreader`). **The licences were recorded from known upstream licence files and were not re-fetched or re-verified in this session.** No font file is committed and none is bundled in the prototypes. Self-hosting steps are in `brand-and-tokens.md` section 4.1. Metric-matched fallbacks are NOT MEASURED.
- Only 6 of 24 routes have prototypes (home, login, overview, jobs, job detail, workers). The other 18 routes (architecture, seven docs pages, changelog, demo, jobs/new, workers/[id], observability, activity, settings, tokens, preferences) are specified in `route-inventory.md`, `component-specs.md` and `states-matrix.md` only. That follows the ticket's prototype list but means the demo, token reveal, and observability charts have no high-fidelity artifact yet.
- The `/architecture` diagram and the console preview image on `/` are not produced (the preview is an HTML mock labelled illustrative; the real capture is blocked until the console exists).
- Tailwind 4 adoption of `tokens.css` is **not verified** (Tailwind is not installed in M2).
- Execution Atlas slot pips are specified but not drawn in the prototype (text `active / capacity` is shown).
- The prototypes' lease countdown and topology pulse are illustrative scripts, not telemetry; "Live" in the prototypes is a prototype state, not a connection.
- Mobile states beyond the default were not overflow-checked at 320 px (the render check covers each page's default state at every width, and screenshots cover several state variants at 390 and 1440 only).
- All E2E IDs cited in the docs remain `proposed`, **NOT RUN**. RV01-10 is a backend test gap; this lane only maps the UI side (`states-matrix.md` section 6). Backend table-driven test status: NOT RUN here.
- Independent visual and accessibility review has not happened (author is not the reviewer, MW-05). Originality is asserted by construction (own grammar, own icons, own layout) but has not been challenged independently.

## Open questions (need an owner or integrator answer)

1. **OQ-13:** approve IBM Plex Sans, IBM Plex Mono and Newsreader (all OFL 1.1), or choose another OFL or Apache fonts? Licence verification should happen when the files are first downloaded at M3 or M6.
2. Is a serif display face for public titles acceptable for the editorial direction (Newsreader on `/`, `/architecture`, `/changelog`, docs page titles only; console stays sans)?
3. `/login` is specified `noindex`. Confirm.
4. Topology and list default: the contract says list is the default; the prototype keeps list default and shows the graph on request. Confirm that the graph should never be the default on desktop.
5. Clustering threshold: individual nodes up to 12 workers, handler clusters beyond that. Confirm or set another threshold once the 100-worker interaction budget is measured at M6.
6. Documentation for the `/demo` tour needs a decision on which fixture screens and how many tour steps ship at M7 (the route contract lists them but gives no count).

## Next owner and required merge or contract step

- **Integrator:** review for data semantics against `contract-m1-draft.4`; decide OQ-13 and the questions above; merge `m2/frontend` after review. No contract change or regeneration is required. Suggested follow-ups for M3: add `@axe-core/playwright` and the `render.mjs` checks to web CI, decide whether the prototype build tool and screenshots stay under `docs/design/` or move, and record the font decision in `docs/product/OWNER_DECISIONS.md` (outside this lane).
- **Reviewer independent from author:** independent visual and accessibility reviewer (not the author) per MASTER section 1.1; Opus to validate data semantics and technical claims (state names, field names, masked fields, unschedulable as a reason). Reviewer feedback: none yet.
- **Traceability movement proposed (integrator updates `docs/product/traceability.md`):** FR-WEB-01..05, 07, 08 and FR-DEMO-01 design: proposed to **specified** with a prototype for six routes; FR-WEB-06 Atlas grammar **specified** and prototyped (marketing Atlas and console topology); NFR-A11Y-01 and NFR-RESP-01: plan **specified**, partial measurement on prototypes only; AC-21 to AC-23: design inputs ready, no evidence for the test states (NOT RUN); RV01-10: UI mapping **specified**, backend execution outstanding.
- **Release gate disposition:** M2 design gate evidence is in place for review. Not claimed: approved, implemented, integrated, tested or verified.
