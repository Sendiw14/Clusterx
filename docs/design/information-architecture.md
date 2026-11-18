# CLUSTERX Information Architecture

- Ticket: TASK-M2-FE-01, deliverable 2. Requirement IDs: FR-WEB-01, FR-WEB-02, FR-WEB-03, FR-WEB-08, NFR-A11Y-01, NFR-RESP-01, AC-21, AC-29.
- Inputs: `docs/design/route-inventory.md` (route authority, 24 routes), `docs/design/consumer-requirements.md`, contract `contract-m1-draft.4`.
- Status: **specified**. Every route in MASTER section 8 appears in the route table below and has a row in `states-matrix.md` (route IDs match the inventory codes).

## 1. Three surfaces, three navigation models

| Surface | Routes | Audience | Chrome | Indexing |
|---|---|---|---|---|
| Public site | `/`, `/architecture`, `/changelog`, `/login`, `/demo` | ANON evaluators, developers | Brand header, footer | Indexed except `/login` (noindex) |
| Docs | `/docs`, `/docs/{quickstart,concepts,api,workers,reliability,deployment,security}` | Developers, operators, reviewers | Brand header, docs sidebar, on-this-page, footer | Indexed |
| Console | `/app`, `/app/jobs`, `/app/jobs/new`, `/app/jobs/[id]`, `/app/workers`, `/app/workers/[id]`, `/app/observability`, `/app/activity`, `/app/settings`, `/app/settings/tokens`, `/app/settings/preferences` | OPERATOR, VIEWER (authenticated) | App sidebar, topbar, breadcrumbs, live indicator | `noindex`, `Cache-Control: no-store` |

Rules shared by all three:

- Every page has a skip link, one `h1`, landmarks (`header`, `nav` with distinct accessible names, `main`, `footer`), and a `<title>` of the form `Page name: CLUSTERX` (docs: `Page name: Docs: CLUSTERX`; console: `Page name: Console: CLUSTERX`). After a client-side route change, focus moves to `main` (which has `tabindex="-1"`) and the new `h1` is announced; scroll resets to top.
- Navigation items never lead to a dead route. A route that is not built is omitted, not disabled (MASTER section 8). The 1.1 routes `/app/workflows*` and `/app/workflow-runs/*` are absent from every menu, sitemap, command search and doc link and answer the global 404 (AC-29).
- The UI hides what a role cannot use. It never decides authorization: a hidden item is a courtesy, and a direct URL still gets the S-403 route state or the Go 403 (route-inventory section 1.1).
- External links (GitHub) open in the same tab with a visible external icon and "(external)" text for screen readers.
- Dates and times: UTC stored, local displayed with zone; public pages that show dates (changelog) show UTC labelled "UTC".

## 2. Public navigation

**Header (all widths):** wordmark (links to `/`), `Architecture`, `Docs`, `Changelog`, `Demo`, and a `Sign in` button (becomes `Open console` linking to `/app` when a session exists). Current page is marked with `aria-current="page"` and a 2 px accent underline (not color alone).

**Mobile menu (below 768 px):** a `Menu` button (text plus icon, `aria-haspopup="dialog"`) opens a left drawer built on a modal dialog: focus trap, `Esc` closes, focus returns to the button, route change closes it, items are 44 px tall full-width links. The drawer holds the same five items; nothing is hidden on mobile that exists on desktop.

**Footer (all widths):** four columns that collapse to one: brand statement; Product (Architecture, Demo, Changelog, Sign in); Docs (all seven doc pages); Project (Security, GitHub, License). Footer also states the release status ("not yet released" until `CHANGELOG.md` has a version) and the UTC rule.

**Breadcrumbs:** none on public pages (shallow). The docs have them (section 3).

## 3. Docs navigation

- **Sidebar** (`nav` named "Documentation"): `Overview` (`/docs`), then the seven pages in learning order: Quickstart, Concepts, Worker SDK (`/docs/workers`), API reference, Reliability, Deployment, Security. Current page `aria-current="page"`. Below 1024 px the sidebar becomes a `Docs menu` button opening the same drawer pattern.
- **On this page** (`nav` named "On this page"): heading anchors for `h2` and `h3`. Scroll-spy updates `aria-current="true"` without moving focus. Below 1024 px it collapses to a disclosure at the top of the article.
- **Breadcrumb:** `Docs / <Page>` (`nav` named "Breadcrumb", `ol`).
- **Prev/next:** follows sidebar order.
- **Docs search scope:** a client-side index built at build time over the seven pages and the docs index (title, headings, body text). It searches **documentation only**; it never searches jobs, workers or any console data. Opens with the Search button or `Ctrl/Cmd+K` (no single-character shortcuts, WCAG 2.1.4); `Esc` closes and restores focus. Results show page, section and a text snippet; zero results states "No matches" with a link to `/docs`; an index load failure falls back to a plain link list of all docs.
- **Edit on GitHub** link at the end of each page (external).
- Docs pages that contain commands show the verification stamp or the "Not yet verified from a clean checkout" notice (route-inventory D-COMMON).

## 4. Console navigation

**Sidebar (docked at 1024 px and up; drawer below):**

| Group | Item | Route | Visible to |
|---|---|---|---|
| Main | Overview | `/app` | OPERATOR, VIEWER |
| Main | Jobs | `/app/jobs` | OPERATOR, VIEWER |
| Main | Workers | `/app/workers` | OPERATOR, VIEWER |
| Main | Observability | `/app/observability` | OPERATOR, VIEWER |
| Main | Activity | `/app/activity` | OPERATOR, VIEWER (viewer sees privileged rows only, masked details; OD-13) |
| Settings | Configuration | `/app/settings` | OPERATOR, VIEWER |
| Settings | Tokens and credentials | `/app/settings/tokens` | OPERATOR only (item absent for VIEWER) |
| Settings | Preferences | `/app/settings/preferences` | OPERATOR, VIEWER |

`Submit job` is not a nav item: it is the primary action on Overview and Jobs (OPERATOR only), with a route at `/app/jobs/new`. Sidebar footer: signed-in GitHub login and role, `Sign out` (a button that ends the session, not a plain link in the product).

**Topbar:** menu button (below 1024), breadcrumb, command search button, live indicator, viewer role banner ("Read-only access (viewer)", persistent, not dismissible), and the user's session actions in the sidebar footer. Topbar is sticky; `scroll-padding-top` plus the 3.5 rem height keeps focused elements from being hidden under it (WCAG 2.4.11).

**Breadcrumb patterns:**

| Route | Breadcrumb |
|---|---|
| `/app` | Console / Overview |
| `/app/jobs` | Console / Jobs |
| `/app/jobs/new` | Console / Jobs / New job |
| `/app/jobs/[id]` | Console / Jobs / `<short id>` |
| `/app/workers` | Console / Workers |
| `/app/workers/[id]` | Console / Workers / `<worker name>` |
| `/app/observability` | Console / Observability |
| `/app/activity` | Console / Activity |
| `/app/settings` | Console / Settings |
| `/app/settings/tokens` | Console / Settings / Tokens and credentials |
| `/app/settings/preferences` | Console / Settings / Preferences |

The last crumb is `aria-current="page"` and plain text. Intermediate crumbs are links. The ID crumb is monospace and never truncated below 8 characters.

**Command search scope (`Ctrl/Cmd+K`, button always visible):**

- Pages: Overview, Jobs, Workers, Observability, Activity, Settings, Preferences, and Tokens (OPERATOR only). Filtered by typed text.
- Exact IDs: a pasted job ID or worker ID runs `GET /jobs/{id}` or `GET /workers/{id}` through the BFF and offers "Open job ..." or "Open worker ...". 404 shows "No job or worker with that ID"; 410 shows an expired notice linking to the expired-data state.
- **Not in scope:** free-text search over jobs, payloads, handlers, workers or audit (the contract has no such operation), docs content (docs have their own search), and 1.1 routes.
- A dialog (modal) with an input labelled "Jump to a page, or enter an exact job or worker ID", live result count, `Esc` closes, focus returns to the trigger.

**URL state:** list filters, window, preset and the topology/list choice live in the URL query so that reload, back and shared links reproduce the view (no payload data ever goes in a URL). Sort order is fixed by the API for jobs and workers, so there is no sort parameter.

**Redirects and auth routing:**

| Situation | Behavior |
|---|---|
| No session on any `/app*` | Redirect to `/login?next=<validated path>` |
| Session on `/login` | Redirect to `/app` |
| Sign-in success | Redirect to validated `next` (same origin, path under `/app`) else `/app` |
| Session ends mid-use | Non-dismissible dialog "Your session ended", sign in again returns to the same URL |
| Sign-out | `/login?reason=signed_out` |
| VIEWER opens operator route (`/app/jobs/new`, `/app/settings/tokens`) | S-403 route state with link to `/app` |
| Unknown or 1.1 route | Global 404 inside the correct shell |

## 5. Route table (all 24 v1.0 routes plus gated and global surfaces)

Columns: **code** (route-inventory ID, also the key in `states-matrix.md`), group, navigation entry, parent for breadcrumb, who can reach it, prototype.

| Route | Code | Group | Entry points | Parent | Roles | Prototype |
|---|---|---|---|---|---|---|
| `/` | HOME | Public | Wordmark, footer | none | ANON | `home.html` |
| `/architecture` | ARCH | Public | Header, footer, home architecture strip | none | ANON | none |
| `/docs` | DOCS-IDX | Docs | Header, footer, sidebar | none | ANON | none |
| `/docs/quickstart` | DOCS-QS | Docs | Sidebar, home CTA, docs index | Docs | ANON | none |
| `/docs/concepts` | DOCS-CON | Docs | Sidebar, docs index, console doc links | Docs | ANON | none |
| `/docs/api` | DOCS-API | Docs | Sidebar, docs index | Docs | ANON | none |
| `/docs/workers` | DOCS-WRK | Docs | Sidebar, docs index, empty workers state | Docs | ANON | none |
| `/docs/reliability` | DOCS-REL | Docs | Sidebar, docs index, 500 state | Docs | ANON | none |
| `/docs/deployment` | DOCS-DEP | Docs | Sidebar, docs index | Docs | ANON | none |
| `/docs/security` | DOCS-SEC | Docs | Sidebar, footer, login | Docs | ANON | none |
| `/changelog` | CHANGELOG | Public | Header, footer | none | ANON | none |
| `/login` | LOGIN | Public | Header "Sign in", redirects | none | ANON (redirect if signed in) | `login.html` |
| `/demo` | DEMO | Public | Header, home CTA, login | none | ANON | none |
| `/app` | APP-HOME | Console | Sidebar, post-login | Console | OPERATOR, VIEWER | `overview.html` |
| `/app/jobs` | JOBS-LIST | Console | Sidebar, overview CTAs, KPI links | Console | OPERATOR, VIEWER | `jobs.html` |
| `/app/jobs/new` | JOBS-NEW | Console | "Submit job" on Overview and Jobs | Jobs | OPERATOR | none |
| `/app/jobs/[id]` | JOBS-DETAIL | Console | Jobs table, command search, lineage links | Jobs | OPERATOR, VIEWER | `job-detail.html` |
| `/app/workers` | WORKERS-LIST | Console | Sidebar, overview topology | Console | OPERATOR, VIEWER | `workers.html` |
| `/app/workers/[id]` | WORKERS-DETAIL | Console | Workers table or graph, command search, job detail | Workers | OPERATOR, VIEWER | none |
| `/app/observability` | OBS | Console | Sidebar, overview chart link | Console | OPERATOR, VIEWER | none |
| `/app/activity` | ACTIVITY | Console | Sidebar, per-item audit links | Console | OPERATOR, VIEWER | none |
| `/app/settings` | SETTINGS | Console | Sidebar | Console | OPERATOR, VIEWER | none |
| `/app/settings/tokens` | TOKENS | Console | Sidebar (OPERATOR), empty workers state | Settings | OPERATOR | none |
| `/app/settings/preferences` | PREFS | Console | Sidebar, user menu | Settings | OPERATOR, VIEWER | none |
| `/app/workflows`, `/app/workflows/[id]`, `/app/workflow-runs/[id]` | GATED | Absent in 1.0 | None | none | none | none |
| any unknown URL | GLOBAL-404 | Global | n/a | none | any | none |
| uncaught error | GLOBAL-500 | Global | n/a | none | any | none |

Prototypes cover the route families that matter most for design review (hero and Atlas, sign-in, overview, list, detail, topology). Routes marked "none" are specified by `route-inventory.md`, `component-specs.md` and `states-matrix.md` and are built from the same components; they have no separate prototype in M2.

## 6. Wireframe descriptions per route family

Desktop (at least 1024) and mobile (below 640). The prototypes realise these at high fidelity for the families that have one.

| Family | Desktop | Mobile |
|---|---|---|
| Home | Two-column hero (title and CTAs, fenced ledger plate), full-width Atlas card with a steps column, three-pillar rule row, five-layer architecture strip, story timeline, console preview frame, quickstart code block, FAQ, CTA band, footer | Single column; CTAs full width; ledger plate below hero; Atlas diagram hidden below 480 px and the eight steps become the primary content; pillars and layers stack with rules between |
| Docs | Left sidebar, centered prose (68 characters), right on-this-page rail | Menu and on-this-page disclosures at top; code blocks scroll inside their own focus-reachable region |
| Sign-in | One centered column with the outcome panel above the button | Same, button full width |
| Console list (Jobs, Workers, Activity) | Page header with actions, filter row, active-filter chips, table, pager | Filters in a disclosure with a count badge, chips stay visible, table becomes labelled cards with the same fields in the same order |
| Console detail (Job, Worker) | Header with ID, copy, status, actions; two-column state and payload cards; attempts as cards; versioned timeline; lineage | Single column; actions wrap and stay reachable; payload and logs scroll inside their own region |
| Overview | KPI grid, attention list, chart and topology side by side, recent jobs, shortcuts | KPIs 2 columns then 1; chart and topology stack with table and list alternatives; recent jobs as cards |
| Observability | Range and step controls, one chart per metric with table under each | Controls in a disclosure; tables stack |
| Settings | Definition lists and link rows | Same, single column |
| Tokens | Tabs, list, create form, one-time reveal dialog | Reveal becomes a full-screen sheet below 480 px |

## 7. IA acceptance

| Check | Evidence plan |
|---|---|
| Every MASTER section 8 route has an IA row | Table in section 5 (24 plus gated and global) |
| Every route has a states-matrix row | `states-matrix.md` section 3 |
| No navigation item leads to a missing route | `E2E-GLOBAL-GATED-02` and link-check in CI at M6/M7 (NOT RUN, not built) |
| Mobile menu holds everything the desktop menu does | `E2E-GLOBAL-MENU-01..03` (proposed, NOT RUN) |
| Viewer never sees operator-only entries | `E2E-GLOBAL-ROLE-01`, `E2E-TOKENS-23` (proposed, NOT RUN) |
