# CLUSTERX Motion Matrix and Asset Inventory

- Ticket: TASK-M2-FE-01, deliverable 6. Requirement IDs: FR-WEB-05, FR-WEB-07, NFR-A11Y-01, AC-22, AC-23.
- Inputs: MASTER section 9 motion budgets (hero 450 to 650 ms, editorial entrances 180 to 350 ms, hover and focus 120 to 180 ms, popovers and dialogs 160 to 240 ms, route changes 150 to 250 ms), `route-inventory.md` section 1.7 (M-* IDs), `tokens.css` (`--duration-*`, `--ease-*`).
- Status: **specified**. Values are starting values within the MASTER budgets; durations in the prototypes use the same tokens. No motion performance or reduced-motion run on real components has happened: **NOT RUN** except the prototype check in section 4.

## 1. Principles

1. Motion explains a state change (something appeared, something is being processed, something was refused). It is never decoration for its own sake and never loops without a reason.
2. Only `opacity` and `transform` animate (plus `stroke-dasharray` on the lease ring, `background-color` and `border-color` on hover). No width, height, top, left or box-shadow animation (no layout thrash).
3. Everything is interruptible: a new trigger reverses from the current value, never jumps.
4. Critical controls, focus movement, form submission and navigation never wait for an animation.
5. No parallax, no scroll hijacking, no autoplaying loops, no animated numeric counters in the console (numbers are authoritative and change instantly).
6. Easing: `--ease-out` (`cubic-bezier(0.2, 0, 0, 1)`) for entrances and state changes, `--ease-in-out` for pulses, `linear` for travelling tokens and the lease ring (time is linear).
7. Reduced motion is a first-class path: two sources, one result. (a) OS `prefers-reduced-motion: reduce`; (b) the app preference "Reduce motion: always" in `/app/settings/preferences`. The override can only add reduction, never force motion on. Under either, durations collapse to 0 ms (tokens), translate and scale are removed, state changes are still announced, no function is lost.

## 2. Motion matrix

Columns: trigger; initial to end; delay; duration; easing; interruption; reversal; repeat mount; scroll up and down; viewport and visibility; reduced-motion fallback.

| ID | Component and where | Trigger | Initial to end | Delay | Duration | Easing | Interruption | Reversal | Repeat mount | Scroll behavior | Viewport / visibility | Reduced-motion fallback |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| M-HERO | Home hero (eyebrow, title, lede, CTAs, plate) | First paint of `/` | Opacity 0 to 1, translateY 12 to 0 px; stagger 60 ms across 4 elements | 0 (stagger 0, 60, 120, 180 ms) | 560 ms | ease-out | Navigation away cancels; no reversal | None | Plays once per full load, not on client back-navigation | Not tied to scroll | Starts only while the document is visible | Final state immediately, no translate |
| M-SECTION | Public section blocks, pillars, plates, cards | Element crosses 20 percent visibility (IntersectionObserver) | Opacity 0 to 1, translateY 8 to 0 px; stagger at most 40 ms for siblings, cycle of 4 | 0 to 120 ms | 260 ms | ease-out | Leaving the viewport mid-way leaves the element at its current value; completes on re-entry | None | Once per page view; no replay | Down: plays. Up: stays visible, no replay | Not observed while tab hidden; elements already above the fold are shown instantly | Final state at mount |
| M-HOVER | Buttons, links, nav items, rows, cards | Pointer hover or `:focus-visible` | `background-color`, `border-color`, `color` change by one step; no layout shift; no scale | 0 | 150 ms | ease-out | Reverses from current value on leave | Yes, symmetric | None | n/a | Not applied on touch (`hover: none`) | Color changes with 0 ms |
| M-PRESS | Buttons | `:active` | Background one step darker; no transform | 0 | 0 to 80 ms | ease-out | Reverses on release | Yes | None | n/a | n/a | Same, 0 ms |
| M-OVERLAY-DIALOG | Dialogs, command search, confirm | `showModal()` and close | Open: opacity 0 to 1 and scale 0.98 to 1. Close: reverse | 0 | 200 ms open, 160 ms close | ease-out | Close during open reverses from current value; focus trapped and moved immediately | Yes | Every open | n/a | n/a | Opacity only, at most 80 ms, or instant |
| M-OVERLAY-DRAWER | Mobile menus, filter drawer | Open and close | Opacity 0 to 1 and translateX -16 to 0 px (right-edge drawers +16 px) | 0 | 200 ms open, 160 ms close | ease-out | As dialog | Yes | Every open | n/a | n/a | Opacity only or instant |
| M-OVERLAY-POPOVER | Reason explanation, tooltip | Open and close | Opacity 0 to 1, translateY 4 to 0 px | 0 | 160 ms | ease-out | Dismiss on `Esc` or focus leave at any time | Yes | Every open | Closes on scroll that moves its anchor out of view | n/a | Opacity only |
| M-OVERLAY-TOAST | Supplementary toast | Event | Opacity 0 to 1, translateY 8 to 0 px; exits with opacity | 0 | 200 ms in, 160 ms out | ease-out | New toast replaces the old one | No (fade-out only) | Every show | n/a | Paused while hovered or focused | Opacity only |
| M-ROUTE | Client navigation, all routes | Route change | Content opacity 0.6 to 1; scroll to top; focus to `main` | 0 | 180 ms | ease-out | A new navigation cancels; the outgoing page never blocks input | None | Every navigation | Scroll resets to top | n/a | Instant swap |
| M-LIVE-ROW | Console rows and timeline entries changed by a reconciled snapshot | Snapshot changes a row | Background overlay (accent tint) opacity 0.35 to 0 | 0 | 400 ms | ease-out | Retrigger restarts; at most one flash per row per 2 s | None | Per change | Rows reorder only on explicit Refresh | Suppressed while the tab is hidden | No flash; the changed cell gets an "Updated" text chip for 3 s |
| M-LIVE-STATE | Live indicator state change | State changes | Text and glyph swap; color change 150 ms | 0 | 150 ms | ease-out | Last state wins | n/a | Every change | n/a | n/a | Instant; announcement unchanged |
| M-ATLAS-TOKEN | Marketing Atlas token | Play or Step | Token moves along the step's connector(s) (`getPointAtLength`) | 0 | 600 ms per connector | linear | Pause stops at the current position; Step jumps to the end | Step does not reverse; Reset returns to step 0 | Plays on user demand | n/a | Auto-play once when 60 percent visible; pauses when scrolled out of view or the tab is hidden | Token placed at the connector end, no travel |
| M-ATLAS-NODE | Atlas node and edge highlight | Step change | Opacity and outline color 0 to 1 | 0 | 600 ms | ease-out | New step overrides | n/a | Per step | n/a | As above | Instant |
| M-ATLAS-RING | Lease ring arc | Step change | `stroke-dasharray` from current arc to target arc | 0 | 600 ms | linear | New step overrides | n/a | Per step | n/a | As above | Instant arc |
| M-TOPO | Console worker node with a running attempt | A real running attempt on a live worker | Pulse ring opacity 0.7 to 0 and scale 1 to 1.25 | 0 | 1200 ms, at most 3 cycles per activation | ease-in-out | Stops immediately when the attempt ends | n/a | Per activation | n/a | Suppressed when the tab is hidden and at 100 or more workers | No pulse; static text badge "active" |
| M-SKEL | Skeleton | S-LOAD shown after 200 ms | Opacity 1 to 0.5 and back | 0 | 1000 ms loop, hard stop at 10 s | ease-in-out | Stops when data arrives | n/a | Per load | n/a | Stops when the tab is hidden | Static tint, no loop |
| M-SPIN | Button pending spinner | S-PENDING | Rotate 360 degrees | 0 | 800 ms per turn, linear | linear | Stops on resolve | n/a | Per pending | n/a | n/a | Static ring plus the changed text ("Redirecting to GitHub") |
| M-COPY | Copy button feedback | Copy success | Text swaps to "Copied" | 0 | none (instant), reverts after 1.8 s | none | Second copy restarts timer | n/a | Per copy | n/a | n/a | Same (no animation involved) |
| M-DETAILS | FAQ `details` | Open and close | No animation (native toggle) | 0 | 0 | n/a | n/a | n/a | n/a | n/a | n/a | Same |
| M-CHART | Charts and topology data swap | New snapshot | Instant swap; stale dims opacity to 0.45 over 150 ms | 0 | 0 (swap) | n/a | n/a | n/a | n/a | n/a | n/a | Same |
| M-NONE | Everything not listed: numbers, KPI values, status text, table sort, filter chips, form errors, tabs | n/a | Instant | n/a | 0 | n/a | n/a | n/a | n/a | n/a | n/a | Same |

Budget check against MASTER section 9: hero 560 ms (450 to 650), section 260 ms (180 to 350), hover 150 ms (120 to 180), overlays 160 to 200 ms (160 to 240), route 180 ms (150 to 250). Atlas steps at 600 ms and the topology pulse at 1200 ms are outside the section 9 table but within "graph pulses only when a job is active"; they are user-driven or data-driven, not entrance motion.

## 3. Implementation rules

- Durations and easings come from tokens. Reduced-motion collapses durations to 0 in `tokens.css` and components remove translate and scale in the same media block and under `data-rm="1"`.
- Library: CSS first (transitions, keyframes, the native `dialog`, `View Transitions` only if proven). Motion (the library, 14.x) is allowed only where CSS cannot express interruption (the Atlas controller, if proven necessary); no library is justified for hover, overlay, reveal or skeleton.
- No `will-change` by default; no animation on elements outside the viewport; no animation continues after the tab is hidden.
- Reveal and hero animations must not change layout (transform and opacity only) so CLS stays at 0.
- Any element that fades in must be in the DOM and focusable before it is visible; hidden-by-opacity elements are never reachable by keyboard before they are shown.
- Server-rendered content is complete without JavaScript; the `reveal` class only applies once script has marked the document (`.js`).

## 4. Prototype evidence (what was and was not checked)

| Check | Command | Result |
|---|---|---|
| Reduced motion: running animations after load at 390 px, each of the six prototypes | `render.mjs` (Playwright, `reducedMotion: 'reduce'`, `document.getAnimations()`) | 0 running animations on all six (`docs/design/evidence/render-check.json`) |
| Hero, reveal and Atlas honored | Same, plus manual read of the CSS | Reveal and hero keyframes are disabled under `prefers-reduced-motion` and `data-rm="1"`; the Atlas controller places the token without travel and does not auto-play |
| Interruption, repeat mount, visibility pause on components | n/a | **NOT RUN** (no components exist yet) |
| Frame budget or long tasks | n/a | **NOT RUN** |
| Real `prefers-reduced-motion` across Chrome, Firefox, Safari, Edge | n/a | **NOT RUN** (only Chromium headless) |

## 5. Asset inventory

All assets are original to this repository or open-licensed. No raster image, stock photo, third-party illustration or third-party logo is used. Nothing here is fetched at runtime.

| Asset | Kind | Path | Source and author | License | Responsive export | Status |
|---|---|---|---|---|---|---|
| CLUSTERX mark | SVG | `docs/design/assets/mark.svg` | Original, authored in this repository | Apache-2.0 (repository license, OD-02) | Vector, 16 to 64 px; favicon and touch icon rasterized from this SVG at M7 | specified, done as SVG |
| Wordmark | Live text, 600 weight, 0.14 em tracking beside the mark | none (CSS) | Original | Apache-2.0 (the typeface carries its own OFL, below) | n/a | specified |
| Icon set (status glyphs and UI icons, 41 symbols plus the mark) | SVG sprite | `docs/design/assets/icons-sprite.svg` (generated from `prototypes/_src/sprite.html` by `tools/build-prototypes.mjs`) | Original geometry from primitive shapes, authored in this repository, 16 px grid, 1.5 px stroke | Apache-2.0 | Vector, rendered at 16, 20, 24 px | done in prototype; production sprite moves to `packages/ui` at M6 |
| Execution Atlas diagram | Inline SVG | in `prototypes/_src/home.html` (production: a component) | Original | Apache-2.0 | Vector; scrolls in region below 680 px; steps list below 480 px | done in prototype |
| Atlas legend glyphs | Inline SVG | `home.html` | Original | Apache-2.0 | Vector | done |
| Console topology and charts | Generated SVG | `prototypes/_src/shared.js`, `overview.html` | Original | Apache-2.0 | Vector | done in prototype (illustrative data) |
| Architecture diagram (`/architecture`) | SVG | not yet drawn | Will reuse Atlas shapes | Apache-2.0 | Vector plus text list | not started (route has no prototype in M2) |
| Console preview image on `/` | Raster (WebP or AVIF) | none yet | A capture of the shipped console at M7 | n/a | Required: 2 widths with `srcset`, fixed aspect box, alt text, caption naming the version captured | **blocked until M6/M7** (the prototype shows an HTML mock labelled illustrative) |
| Open Graph image | Raster | none | Generated at M7 from the mark and title | Apache-2.0 | 1200 by 630 | not started |
| IBM Plex Sans, IBM Plex Mono | Fonts (WOFF2, Latin) | not committed | IBM, upstream `github.com/IBM/plex` | SIL OFL 1.1 | Subset Latin, `font-display: swap` | licence **not re-verified this session**; files not committed (see `brand-and-tokens.md`) |
| Newsreader | Variable font (WOFF2, Latin) | not committed | Production Type, upstream `github.com/productiontype/Newsreader` | SIL OFL 1.1 | As above | same |
| Screenshots | PNG | `docs/design/screenshots/` | Rendered from the prototypes by `tools/render.mjs` (Chromium headless) | Apache-2.0 | 390 and 1440 px, full page | done; they show the system fallback faces, not Plex or Newsreader |

Icon inventory by purpose (all in the sprite, all with accessible labels where meaning is carried by the icon alone):

- Job states: scheduled, queued, running, retry, succeeded, failed, canceled.
- Worker states: online, stale, draining, deregistered (off), revoked.
- UI: lock, copy, check, search, menu, close, chevrons (down, right, left), external, info, alert, refresh, plus, minus, play, pause, step, reset, up, down, list, graph, filter, signal-off, dot, pause-bar, clock, arrows.

Rules: icons are decorative (`aria-hidden`) next to text; an icon-only control has an `aria-label`; no icon carries meaning alone. The mark has a title when used as a link target and `aria-hidden` inside the wordmark link that already has text.

## 6. M7 addendum (public site, docs, demo)

Status: implemented and tested on Chromium only. Source: TASK-M7-FE-01.

| ID | Where | What shipped | Reduced-motion result |
|---|---|---|---|
| M-HERO | Home hero | Title block unchanged (stagger 60 ms). The "Fig. 1" ledger plate no longer waits for JavaScript: it uses the same CSS keyframe with a 240 ms delay (`.hero-rise`). The first M7 Lighthouse run showed the plate, inside a scroll-reveal wrapper, as the mobile LCP element (4.1 s, 411 ms render delay), which is why | `animation: none`, content visible at first paint |
| M-SECTION | Home, architecture diagram | As specified (260 ms, once) | Final state at mount |
| M-ATLAS-* | Home Atlas | Unchanged from M3. Auto-play once at 60 percent visible, never under reduced motion | Verified: no auto-play, Play toggles by keyboard |
| (none) | Docs search popup, on-this-page highlight, copy button, docs menu drawer | No entrance animation. The current section marker changes instantly | n/a |
| (none) | Architecture topology SVG | Static. It does not animate; it carries a title, description, a layer list and a connections table | n/a |
| (none) | Demo recovery replay | Visitor-driven (Step, Show all, Reset). No timers, no auto-play, no animation | n/a |

No scroll hijacking: a wheel event scrolls the document by the native amount (tested). No animation library was added.

Asset inventory changes: the architecture diagram (`/architecture`) is now drawn as an inline SVG reusing the Atlas shapes (service block, chamfered worker, slotted store). The console preview image on `/` is still an HTML sample labelled "preview sample, not a capture of shipped software": a real capture is blocked until the console is connected to live data (M6).
