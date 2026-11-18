# HANDOFF — TASK-M3-FE-01 (web foundation: Next.js app, UI library, data providers, test harness)

- **Task:** `docs/coordination/tasks/TASK-M3-FE-01.md`. Requirements: FR-WEB-04, FR-WEB-05, FR-WEB-07, FR-WEB-09 (BFF skeleton), FR-DEMO-01 (provider isolation), NFR-A11Y-01, NFR-DEP-01; AC-14 (isolation harness), AC-21/22 (harness).
- **Author:** frontend lane, model `claude-sonnet-5-5` as given in the session. Effort `high` was requested; the effective effort is not observable from inside the session (MW-04: UNVERIFIED).
- **Branch, commits:** `m3/frontend`, worktree `.worktrees/frontend`, base `76b2401`. No push, no merge.
  - `edbb6be` app tooling, self-hosted fonts, `@clusterx/ui` library, lockfile
  - `02a0319` route shells (24 routes plus global 404/500), data-provider seam, demo isolation, BFF skeleton, headers and CSP
  - `5b7053c` Vitest, Playwright, axe, demo isolation harness, evidence and screenshots, ESLint pinned to 9.39.5
  - plus the commit containing this handoff (its SHA is in the return message; a file cannot hold its own id)
- **Contract:** `contract-m1-draft.4` = `c48b62f88ac76337f43a21e2adb08ae6dccc5ab2`. `packages/sdk-ts` is imported only (types plus `createClusterxClient`); nothing in it was edited. Draft.5 is additive: regenerate and rerun `pnpm --filter web typecheck`.
- **Implemented / Integrated:** **implemented and tested against fixtures and the 501 skeleton**. **Not integrated**: no live E2E exists because the BFF has no session, OAuth or proxy yet (M5). Console views are real components wired to `LiveProvider`, but they have only ever seen the typed 501 answer. They have never rendered live data. The demo renders the same views on labelled fixtures.
- **Changed files (all inside allowed paths):** `apps/web/**`, `packages/ui/**` (package.json, tsconfig, src, tools, generated `tokens/theme.generated.css`; `tokens/tokens.css` untouched), `docs/design/brand-and-tokens.md` (section 9 updated), `docs/design/evidence/axe-m3.{md,json}`, `docs/design/screenshots/m3/*.png`, this file.
- **Root lockfile:** `pnpm-lock.yaml` changed by `pnpm install` for `apps/web` and `packages/ui` only (new importers plus their dependency entries). Existing `@redocly/cli 2.80.0`, `openapi-fetch 0.17.0`, `openapi-typescript 7.13.0`, `typescript 5.9.3` are unchanged; the diff shows some entries moved (sorting), no version changed. `package.json` and `pnpm-workspace.yaml` untouched (the workspace already listed `apps/web` and `packages/ui`).

## Pinned versions for the dependency matrix (tested set, 2026-11-18, macOS arm64, Node 26.13.1, pnpm 9.15.9)

| Package | Pinned | Note |
|---|---|---|
| next | 16.4.6 | App Router, Turbopack build (`next build` default); no webpack fallback needed |
| react / react-dom | 19.3.1 | |
| typescript | 5.9.3 | **Not 7.0.2 or 6.x**: same set as `sdk-ts` and `openapi-typescript@7.13.0`; `typescript-eslint@8.73.0` peer is `<6.1.0`. Re-test 6.x/7.x as one move across lanes |
| tailwindcss, @tailwindcss/postcss | 4.3.5 | token fit verified (see below) |
| eslint | **9.39.5** | **Not 10.15.0.** ESLint 10 ran, but `eslint-plugin-react@7.37.5`, `eslint-plugin-import@2.32.0` and `eslint-plugin-jsx-a11y@6.10.2` (bundled by `eslint-config-next@16.4.6`) declare peers up to 9, so `pnpm install` printed unmet-peer warnings. ESLint 9.39.5 is clean (a "deprecated" notice only). Revisit when `eslint-config-next` supports 10 |
| eslint-config-next | 16.4.6 | flat config, `eslint .` (not `next lint`) |
| typescript-eslint | 8.73.0 | |
| vitest | 5.1.1 | with vite 8.3.8 and @vitejs/plugin-react 6.3.0 (needs vite 8), jsdom 31.0.1 |
| @testing-library/react / dom / jest-dom / user-event | 16.3.3 / 10.4.2 / 7.0.1 / 14.6.12 | |
| @playwright/test | 1.65.1 | Chromium 159 (`chromium-1255`, cached from M2). Firefox and WebKit NOT installed or run |
| @axe-core/playwright | 4.14.0 | |
| @types/react, @types/react-dom, @types/node | 19.3.0, 19.3.0, 26.6.6 | |
| server-only | 0.0.1 | |
| **Motion** | **not installed** | The motion matrix is met with CSS and tiny effects (Atlas uses rAF for the token). `npm view motion` is 14.14.0 if a later slice needs it. No evidence yet that it is needed (MASTER: no library without benefit) |
| Fonts | IBM Plex Sans 400/500/600, IBM Plex Mono 400/500, Newsreader variable roman and italic (Latin) | OFL 1.1 texts fetched from upstream `IBM/plex` and `productiontype/Newsreader` and checked to contain the OFL text; woff2 from Fontsource 5.3.0 packages; SHA-256 in `apps/web/public/fonts/FONTS.md` |

Peer check across Next 16.4.6, React 19.3.1, TS 5.9.3, Tailwind 4.3.5, ESLint 9.39.5, Vitest 5.1.1 and Playwright 1.65.1: no unmet peers after the ESLint step-back.

## Exact commands and exit codes (run from the worktree root, Node 26.13.1 first on PATH)

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `pnpm install` | 0 | lockfile updated for `apps/web` and `packages/ui` |
| 2 | `pnpm --filter web lint` | 0 | 0 errors, 0 warnings |
| 3 | `pnpm --filter web typecheck` | 0 | `next typegen && tsc --noEmit` |
| 4 | `pnpm --filter web test` | 0 | 7 files, **50 tests passed** |
| 5 | `pnpm --filter web build` | 0 | Turbopack; 24 routes plus `/_not-found`; 12 public routes static, `/login` and the 11 console routes dynamic, 4 BFF handlers |
| 6 | `pnpm --filter web e2e` | 0 | **78 passed, 14 skipped** (the skipped are the opt-in screenshot tests) |
| 7 | `SCREENSHOTS=1 pnpm --filter web e2e` | 0 | 92 passed; regenerates `docs/design/screenshots/m3/` |
| 8 | `node packages/ui/tools/gen-theme.mjs --check` | 0 | `theme.generated.css` is current (also run inside Vitest) |

Acceptance chain `lint && typecheck && test && build && e2e` ran as separate steps, all exit 0 (rows 2 to 6), on the final tree before the handoff commit.

## What was built

- **`apps/web`:** Next 16 App Router. Route groups: `(public)` (home, architecture, changelog, login, 7 docs pages plus docs index), `demo`, `(console)/app/**` (11 routes). Global `not-found` (brand shell, 404) and `error`/`global-error` (no stack trace, digest as the correlation id). 1.1 routes (`/app/workflows*`, `/app/workflow-runs/*`) are not built and answer the 404 (tested).
- **`packages/ui` (`@clusterx/ui`, source-consumed):** Icon sprite and Icon; Button, ButtonLink, IconButton (pending and unavailable states); StatusBadge and legend (seven 1.0 states, worker states, never `blocked` or `skipped`), ReasonChip, MaskedField, LiveIndicator; Notice; skeleton, EmptyState, ErrorState (copy chosen from the code, request id with copy), Offline and Stale notices, Region; Dialog (native `<dialog>` with `showModal`, blocking and alert variants), drawer, toast (one at a time, pause on hover or focus), Tabs (ARIA roving focus); DataTable (caption, scoped headers, labelled stacked cards below 640, focusable scroll region) and cursor Pagination; TextInput, Select, Textarea, FilterChip, Segmented; CodeBlock and CopyButton (polite status, blocked-clipboard message); Timestamp (UTC title, zone preference, hydration-safe); preferences store (storage wrapped in try/catch); public header, footer, docs sidebar; console shell (sidebar, topbar, breadcrumbs, Ctrl/Cmd+K command dialog with exact-ID lookup, live indicator, role banner, drawer); RouteFocus (focus to `main` after a client route change); Reveal (M-SECTION).
- **Execution Atlas (home):** ported from the M2 prototype: 8 scripted steps, Play/Pause/Step/Reset, polite caption, always-visible step list, auto-play once at 60 percent visible (never under reduced motion), pauses when hidden, labelled "Illustrative, scripted".
- **Data seam:** `src/data/provider.ts` (one `DataProvider` interface), `live-provider.ts` (browser to `/bff/api/v1/*` through the generated `@clusterx/sdk-ts` client, 10 s timeout, no tokens), `fixture-provider.ts` (in-memory, no network, no timers, viewer view removes payload, result, error text, log tail and hostname from the objects). Shared views in `src/features/*` read the provider from context; the console layout supplies the live one, `/demo` the fixture one.
- **Demo isolation:** ESLint `no-restricted-imports` (live provider, session, BFF contract, SDK runtime, `openapi-fetch`, `server-only`; type-only SDK imports allowed) on `src/app/demo`, `src/features`, fixture provider, context and provider interface; an import-graph Vitest test walks the real graph; an ESLint-API test proves the rule fires; a Playwright network trap fails on any cross-origin request, `/api/` or `/bff/` path, non-GET, `Authorization` header, WebSocket or cookie during a full tour in both views. A control test shows the same trap does flag the console's BFF calls.
- **BFF skeleton (ADR-0006 interfaces, no behaviour):** `GET /bff/session`, `POST /bff/logout`, `/bff/api/v1/[...path]`, `/bff/stream` answer the typed envelope `{code:"not_implemented", message, request_id}` with HTTP 501, `X-Request-Id` and `no-store`. `src/lib/session.ts` is `server-only` with the `Session` union (`unavailable | none | active`); today it always returns `unavailable`. `src/lib/bff-contract.ts` holds the session and error types from the ADR. There is no OAuth, cookie, assertion or CSRF code.
- **Headers (next.config.ts):** `X-Content-Type-Options`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy`, COOP, CORP, HSTS, and a CSP (`default-src 'self'`, `connect-src 'self'`, `object-src 'none'`, `frame-ancestors 'none'`, `base-uri 'self'`, `form-action 'self'`, no remote fonts or images). `/app/*`, `/login` and `/bff/*` are `no-store` plus `X-Robots-Tag: noindex`.
- **Tailwind 4 token fit (the M2 NOT RUN item): verified.** Not a drop-in: a generated `@theme static` file (from `tokens.css`, drift-tested) makes `bg-canvas` and `text-ink` compile to `var(--color-*)` utilities. Details in `docs/design/brand-and-tokens.md` section 9.

## Honest scope notes (what the console shells really show)

- No session exists, so `getSession()` returns `unavailable`. Console routes render the full shell with a persistent warning banner ("not connected to a backend in this build") and a live indicator reading "Offline, not connected". Every data region then shows the typed S-ERR state "Not connected yet" with a copyable request id and Retry. The role is unknown, so Tokens is absent from the nav and Submit job is absent.
- `/app/jobs/new`, `/app/observability`, `/app/activity`, `/app/settings/tokens` hold a header and a plain "built in M6" notice. They have no controls and no fake data. Chart, topology graph, JSON editor, token reveal, mutations (cancel, retry, drain), stream and polling are **not built** (M6). Console `Settings` (config) and `Preferences` (time zone, reduce motion) are real; preferences are stored per browser only.
- `/login`: "Sign in with GitHub" is shown `aria-disabled` with the visible reason "not enabled in this build". The closed outcome set from ADR-0006 renders its copy; any other `error` or `reason` value renders nothing.
- Docs pages carry settled facts only (concepts, reliability, security summaries, error model) and a visible "Documentation in progress" notice where content is M7. The quickstart lists prerequisites and the intended flow, **no commands** (nothing is verified from a clean checkout; the home page keeps the prototype's example block under the "Not yet verified" notice). Docs search, on-this-page rail and scroll-spy are omitted rather than faked. The API reference is not generated yet.
- `/changelog` shows "No releases yet." and no version. The footer says "not yet released".
- Demo timestamps follow the viewer's zone preference (default: the browser zone), so on a non-UTC machine the table times differ from the "Fixture time ... UTC" caption. Timestamps carry UTC in the tooltip. A reviewer may prefer `utc` for demo views.

## Evidence

- **axe** (`docs/design/evidence/axe-m3.md` and `.json`): 59 scans (24 routes at 320 and 1440, demo screens in both views, open mobile menu): **critical 0, serious 0, moderate 0, minor 0**. Tags wcag2a, wcag2aa, wcag21a, wcag21aa, wcag22aa. This is automated coverage only; no WCAG conformance is claimed.
- **Screenshots** (`docs/design/screenshots/m3/`, Chromium, reduced motion so reveal animations show their final state, full page, real self-hosted fonts): `home`, `jobs`, `jobs-failed-filter`, `demo`, `demo-jobs`, `demo-job-viewer`, `login` (the `no_entitlement` outcome), each at `-390` and `-1440`. They show the shipped components, not the prototypes; layout follows the M2 prototypes (home sections, console shell, jobs filters and table, status badges). `jobs-*` show the real 501 "not connected" state, not data.
- **Test coverage by ticket item:** route shells at 320 and 1440 (single `h1`, landmarks, skip link, title pattern, no horizontal overflow, no console errors other than the intentional BFF 501, axe); demo isolation (E2E-DEMO-20/21/22/23/24/25/26/27); menu and command dialog focus trap and restore; skip link; focus ring; reduced motion (no running animation after load, `data-rm` preference, persists, degrades with blocked storage); time-zone preference; URL-driven filters; offline banner recovery; login outcomes; BFF 501 envelope; security headers; 404 for 1.1 routes; copy checks for the home page (at-least-once stated, no exactly-once, HA or workflow claim).

## NOT RUN / limitations

- **Live E2E: NOT RUN.** No BFF session, proxy or Go backend is connected. The feature "integrated" is **not** claimed for any console view. The integrator must run API plus UI E2E (authorized and denied paths, 401, 403, 410, 503, reconnect) after M5/M6.
- Firefox, WebKit, Safari, Edge: **NOT RUN** (Chromium only).
- Manual NVDA, VoiceOver and keyboard walkthroughs (E2E-*-12): **NOT RUN**. 200 percent and 400 percent zoom: only the 320 px viewport is tested (no zoom emulation).
- Widths 360, 768, 1024, 1920 and the other screenshots from the M2 list: **NOT RUN** (320, 390, 1440 only).
- Performance: LCP, INP, CLS, JS size and p95 budgets: **NOT MEASURED**. Client JS was kept small by design (server components by default; client islands are shells, views and the Atlas) but no byte budget was checked. Metric-matched font fallbacks not measured.
- Session-expired dialog (S-SESSION-EXPIRED), CSRF retry (S-CSRF), 429 countdown, S-TOO-MANY, S-EXPIRED-DATA rendering, live indicator state machine (polling, reconnect, paused), live-row flash (M-LIVE), topology and chart components: **not built** (M5/M6). The `LiveIndicator` and error and notice components exist.
- CSP uses `script-src 'unsafe-inline'` because Next 16 emits inline bootstrap scripts. A nonce policy needs `proxy.ts` and dynamic rendering of every page; deferred to M5/M8 with the BFF. The demo has no stricter CSP than other routes because client navigation needs `connect-src 'self'`; demo isolation is enforced by the lint boundary, the import-graph test and the network trap instead. No `upgrade-insecure-requests` (HSTS only) so `http://localhost` runs work.
- Dev server (`next dev`) was not exercised; all checks run on the production build.
- ESLint 9 is flagged "deprecated" by npm (end of life notice); see the version table for the step-back reason.

## Integration needs and next dependencies

1. Integrator: wire `lint`, `typecheck`, `test`, `build`, `e2e` of `apps/web/package.json` into root scripts and CI; install Chromium in CI (`pnpm exec playwright install chromium`); `e2e` needs a prior `build` (the web server is `next start` on port 3100). Add the versions above to `docs/architecture/dependency-matrix.md`.
2. Integrator: when `sdk-ts` is regenerated for draft.5, run `pnpm --filter web typecheck`. The live provider casts the openapi-fetch result through a single `run()` helper, so a schema change shows up as a typecheck error in `live-provider.ts` or the views.
3. M5 (Opus contract owner, frontend implementer): replace `lib/session.ts`, the four `app/bff/**` handlers and the login button; add CSRF, assertion minting and the proxy header allowlist (GAP-BFF-02); redirect on `Session.kind === "none"`. Security review of `app/bff/**` and `lib/**` is required by MASTER section 1.1 before it is marked integrated.
4. M6: build the chart, topology, JSON editor, token reveal, mutations, stream and polling on top of `DataProvider`; add `listAudit`, `getTimeseries`, `listHandlers` and mutation methods to the interface and both providers (the fixture provider must stay network-free; the boundary tests will catch a slip).
5. Review owner: independent visual and accessibility reviewer; Opus validates the permission accuracy of the demo viewer masking against the access matrix.
