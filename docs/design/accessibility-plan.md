# CLUSTERX Accessibility Audit Plan (WCAG 2.2 AA)

- Ticket: TASK-M2-FE-01, deliverable 7. Requirement IDs: NFR-A11Y-01, NFR-RESP-01, FR-WEB-07, AC-21, AC-22, AC-23.
- Target: WCAG 2.2 level AA (WCAG 3 is a draft and not a target). MASTER section 10: no claim of conformance without criterion-by-criterion proof; axe complements, never replaces, manual keyboard and screen-reader walkthroughs.
- Status: **plan specified**. Evidence status per row: only items marked **MEASURED** were actually run in M2, against the static prototypes. Everything else is **NOT RUN** (components and routes do not exist yet). No conformance is claimed.

## 1. What was actually run in M2 (prototypes only)

| Check | Command | Scope | Result |
|---|---|---|---|
| Color contrast, 73 required pairs from `tokens.css` | `node docs/design/tools/contrast.mjs` | Tokens | 73 pass, 0 fail (exit 0); `docs/design/evidence/contrast-output.md` |
| No horizontal page overflow at 320, 360, 390, 768, 1024, 1440, 1920 CSS px | `node docs/design/tools/render.mjs` (Playwright, Chromium headless) | Six prototypes, default state | 0 overflows; `docs/design/evidence/render-check.json` |
| One visible `h1`, one `main`, the prototype label present, no console errors, no external request | same | Six prototypes at every width | All pass (network blocked except `file:`) |
| Type hierarchy (computed sizes) | `node docs/design/tools/type-hierarchy.mjs` | Six prototypes at 390 and 1440 | Page title at least 1.66 times body, section heading at least 1.29 times; `evidence/type-hierarchy.txt` |
| Reduced motion: no running animation after load | same as render check | Six prototypes at 390 px | 0 running animations |

Not run: axe-core (not installed, no network use beyond the Playwright browser), Lighthouse, any screen reader, Firefox, Safari, Edge, zoom at 200 and 400 percent, text-spacing override, forced-colors, state variants at 320 px (only default state is overflow-checked), keyboard walkthrough. These are scheduled in section 5.

## 2. Criterion checklist (WCAG 2.2 A and AA)

Legend for "Where verified": **P** = prototype can show it; **E2E** = Playwright test ID from `route-inventory.md` (all proposed, NOT RUN); **M** = manual walkthrough; **CI** = automated check (axe, link check, copy lint). Status: **MEASURED** (run in M2), **PLANNED** (design measure specified, not run), **N/A** (reason given).

| SC | Level | Design measure | Where verified | Status |
|---|---|---|---|---|
| 1.1.1 Non-text Content | A | Decorative SVG and glyphs `aria-hidden`; informative SVG has `<title>` and `<desc>`; icon-only buttons have `aria-label`; charts and the Atlas have text alternatives (table, steps list) | E2E, M, CI | PLANNED |
| 1.2.1 to 1.2.5 Time-based media | A, AA | No audio or video in 1.0. If a demo video is added at M9 it needs captions and a transcript | n/a | N/A until video exists |
| 1.3.1 Info and Relationships | A | Landmarks, `h1` to `h3` order, `th scope`, `caption`, `fieldset`/`legend` for groups, labels bound with `for`, definition lists, ordered lists for steps and timelines | E2E-*-01, M, CI | PLANNED (structure counts MEASURED on prototypes) |
| 1.3.2 Meaningful Sequence | A | DOM order equals visual order; stacked mobile table cards keep the same field order | M | PLANNED |
| 1.3.3 Sensory Characteristics | A | Instructions never rely on shape, color or position alone ("the Retry button" not "the blue button") | Copy review | PLANNED |
| 1.3.4 Orientation | AA | No orientation lock | M | PLANNED |
| 1.3.5 Identify Input Purpose | AA | No personal-data fields in 1.0 (sign-in is by GitHub redirect); where fields exist, `autocomplete` is set; secret fields `autocomplete="off"` deliberately | E2E | PLANNED |
| 1.4.1 Use of Color | A | Status is text plus glyph; chart series differ by dash, marker and direct label; links in prose are underlined | E2E-GLOBAL-A11Y-04, M | PLANNED (design in `brand-and-tokens.md` sections 6 and 7) |
| 1.4.2 Audio Control | A | No audio | n/a | N/A |
| 1.4.3 Contrast (Minimum) | AA | All text pairs at least 4.5:1, no large-text relaxation | contrast tool | **MEASURED** (tokens only; real components not yet measured) |
| 1.4.4 Resize Text | AA | Fluid rem-based scale; layouts hold at 200 percent text zoom | M, E2E-*-09 | PLANNED |
| 1.4.5 Images of Text | AA | No images of text; wordmark is live text; the preview image is a screenshot with an alt and caption | M | PLANNED |
| 1.4.10 Reflow | AA | 320 CSS px with no page-level scroll; code, JSON editor and wide SVG scroll in their own focusable region; tables become labelled cards below 640 | E2E-*-09, M | **MEASURED** on six prototypes at 7 widths in default state; other states and real components PLANNED |
| 1.4.11 Non-text Contrast | AA | Control borders 3.5:1, focus ring 5.9:1, status glyphs and chart lines at least 3:1 | contrast tool | **MEASURED** (tokens) |
| 1.4.12 Text Spacing | AA | No fixed heights on text containers; tested with line height 1.5, paragraph spacing 2 times, letter spacing 0.12 em, word spacing 0.16 em | M, E2E | PLANNED |
| 1.4.13 Content on Hover or Focus | AA | Tooltips are dismissible (`Esc`), hoverable and persistent; no information appears only on hover | E2E, M | PLANNED |
| 2.1.1 Keyboard | A | Every control is native or has full key support; the graph has button nodes and zoom and pan buttons; chart data in a table | E2E-*-11 | PLANNED |
| 2.1.2 No Keyboard Trap | A | Only modal dialogs trap focus, and they close on `Esc` (token reveal closes only after acknowledgement, with a visible Close that becomes enabled) | E2E, M | PLANNED |
| 2.1.4 Character Key Shortcuts | A | None. `Ctrl/Cmd+K` is the only shortcut and uses a modifier | M | PLANNED |
| 2.2.1 Timing Adjustable | A | Session expiry warns 60 s ahead (`auth_expiring`); expiry dialog lets the user sign in again and keeps drafts; toasts pause on hover and focus | E2E-GLOBAL-SESSION | PLANNED |
| 2.2.2 Pause, Stop, Hide | A | Atlas has Pause, never loops; skeleton pulse stops at 10 s; no auto-updating content except live counts which can be paused by tab visibility and are not announced per event | E2E-HOME-21, M | PLANNED |
| 2.3.1 Three Flashes | A | No flashing; M-LIVE-ROW is a single fade at most once per 2 s | M | PLANNED |
| 2.4.1 Bypass Blocks | A | Skip link first; landmarks | E2E-GLOBAL-A11Y-01 | PLANNED (skip link present on every prototype) |
| 2.4.2 Page Titled | A | Unique `<title>` per route (`Page: Section: CLUSTERX`) | E2E-*-01 | PLANNED (titles present on prototypes) |
| 2.4.3 Focus Order | A | DOM order, no positive `tabindex`; focus to `main` after route change; focus restored after dialogs | E2E-*-11 | PLANNED |
| 2.4.4 Link Purpose (In Context) | A | Links named by destination; "View" links include context in their text or `aria-label` | M, CI | PLANNED |
| 2.4.5 Multiple Ways | AA | Navigation, docs search, command search, breadcrumbs, in-page links | M | PLANNED |
| 2.4.6 Headings and Labels | AA | Descriptive headings and labels | Copy review | PLANNED |
| 2.4.7 Focus Visible | AA | 2 px ring, 2 px offset, 5.9:1; inset ring on dense lists | E2E-*-11 | PLANNED (ring colors MEASURED) |
| 2.4.11 Focus Not Obscured (Minimum) | AA | `scroll-padding-top` equals sticky topbar plus margin; sticky bars never cover a focused control; prototype label bar is not sticky | E2E-*-11 | PLANNED |
| 2.5.1 Pointer Gestures | A | No path-based or multi-point gesture is required (zoom and pan are buttons) | M | PLANNED |
| 2.5.2 Pointer Cancellation | A | Actions fire on `click` (up event); no down-event actions | M | PLANNED |
| 2.5.3 Label in Name | A | Visible label text is contained in the accessible name (for example "Sign in with GitHub") | CI, M | PLANNED |
| 2.5.4 Motion Actuation | A | No device-motion input | n/a | N/A |
| 2.5.7 Dragging Movements | AA | Topology zoom and pan available as buttons; no drag-only function | E2E-WORKERS-LIST-23 | PLANNED |
| 2.5.8 Target Size (Minimum) | AA | At least 24 by 24 px for every target; controls 32 to 44 px | E2E, M | PLANNED (sizes specified in tokens) |
| 3.1.1 Language of Page | A | `lang="en"` | CI | PLANNED (set on prototypes) |
| 3.1.2 Language of Parts | AA | English-only 1.0; code and identifiers need no `lang` | n/a | N/A |
| 3.2.1 On Focus | A | No context change on focus | M | PLANNED |
| 3.2.2 On Input | A | Filters apply on explicit change of a visible control and the result count is announced; no surprise navigation | M | PLANNED |
| 3.2.3 Consistent Navigation | AA | Same nav order on every page of a surface | M | PLANNED |
| 3.2.4 Consistent Identification | AA | Same glyph and words for the same status everywhere (single legend) | M | PLANNED |
| 3.2.6 Consistent Help | A | Docs link and request ID copy are in the same place in every error state | M | PLANNED |
| 3.3.1 Error Identification | A | Errors identified in text, field-level for 422 | E2E-JOBS-NEW-22 | PLANNED |
| 3.3.2 Labels or Instructions | A | Every input has a visible label and described help; required in words | E2E | PLANNED |
| 3.3.3 Error Suggestion | AA | Errors state the limit or fix ("Payload is 70 KiB; the limit is 64 KiB") | E2E | PLANNED |
| 3.3.4 Error Prevention (Legal, Financial, Data) | AA | Destructive actions (cancel, retry, drain, revoke) use a confirm dialog that states consequences; submit is idempotent | E2E | PLANNED |
| 3.3.7 Redundant Entry | A | Values kept across validation errors; draft retained across session expiry | E2E-JOBS-NEW-30 | PLANNED |
| 3.3.8 Accessible Authentication (Minimum) | AA | No passwords, no cognitive test, no CAPTCHA; GitHub redirect only | E2E-LOGIN-20 | PLANNED (design true by construction) |
| 4.1.2 Name, Role, Value | A | Native controls; ARIA only for dialog, tab-like toggles (`aria-pressed`), live regions, combobox, `aria-current`, `aria-sort`, `aria-busy` | CI, M | PLANNED |
| 4.1.3 Status Messages | AA | One polite status region per page; live indicator announces on state change only; errors that block use `role="alert"`; no per-event announcements | E2E, M | PLANNED |

WCAG 2.2 removed 4.1.1 Parsing. Level AAA criteria (including 2.4.12, 2.4.13, 3.3.9, 2.5.5, 1.4.6) are not targets; where a design choice already satisfies one (for example 2.5.5 on mobile buttons) it is a bonus, not a claim.

## 3. Test widths, browsers, zoom

- **Widths (CSS px):** 320, 360, 390, 768, 1024, 1440, 1920. Dense, empty, error, offline and viewer-masked screens each tested at 320, 390 and 1440 at minimum.
- **Reflow:** 320 CSS px viewport and 400 percent browser zoom on a 1280 px desktop (equivalent to 320 px). 200 percent zoom at 1440. Two-dimensional exceptions: code, JSON editor, the Atlas diagram and the topology scroll inside their own focusable, labelled regions, with an adjacent text alternative.
- **Browsers (planned matrix, to be fixed before release):** current stable Chrome, Firefox, Safari and Edge on desktop, plus Safari on iOS and Chrome on Android for the mobile console. Only Chromium headless was available in M2.
- **Operating settings:** reduced motion on and off; forced colors (Windows High Contrast); increased text size; reduced transparency (not used by design).

## 4. Screen-reader walkthrough scripts

Pairs: NVDA with Firefox and Chrome on Windows; VoiceOver with Safari on macOS and iOS. Each script lists the steps and what must be announced. Record MANUAL-PASS or MANUAL-FAIL with date, version, and notes; never infer from axe.

| # | Journey | Steps | Must hear |
|---|---|---|---|
| S1 | Home and Atlas | Load `/`; move through landmarks; reach the Atlas; activate Step four times; open an FAQ item; activate the quickstart Copy button | Skip link first; one `h1`; "Illustrative, scripted" before the diagram; the diagram announced as an image with a description; each Step announces "Step N of 8" plus the caption once (no double announcement); Play announces pressed state; Copy announces "Copied to clipboard" |
| S2 | Sign in | Load `/login?error=no_entitlement`; then default | Outcome panel announced as an alert on load and focus moved to it; button name "Sign in with GitHub"; no password field exists |
| S3 | Jobs list | Open `/app/jobs`; change Status to Failed; remove the chip; page with Next | Table caption with count and window; headers read per cell; result count announced politely once after the change; focus stays on the changed control; chip removal announced; disabled Previous reads its reason |
| S4 | Job detail, operator | Open a failed job; read attempts; open Retry; confirm | Status in words; each attempt as an article with heading; four log renderings distinguishable (output, none, masked, expired); dialog title and consequence read on open; focus returns to the trigger; "New job created" announced |
| S5 | Job detail, viewer | Same job as viewer | Banner "Read-only access (viewer)"; masked fields read "Hidden for viewer role"; no Cancel or Retry controls exist; no diagnostic entries |
| S6 | Running job and offline | Open a running job; go offline | Lease countdown is not announced per second; offline notice announced; countdown text becomes "unknown while offline"; actions read as unavailable with reason |
| S7 | Workers and topology | Open `/app/workers`; switch to Graph at 10 and 100 workers; move with arrows; select; return to List | Graph named with count and a pointer to the List; nodes read name, state, handler, slots; selection reads the detail; List has the same entries; zoom and pan buttons are named |
| S8 | Tokens reveal (when built) | Create a token | Reveal dialog announced with "shown once"; the secret readable by exploring the labelled field; copy announced without reading the secret aloud automatically; Close unavailable until acknowledged; secret never present after closing |
| S9 | Session expiry | Let the session expire while a form is open | Non-dismissible dialog announced; focus inside; sign in returns to the same URL; draft restored |
| S10 | Demo | Open `/demo`; toggle Operator and Viewer; run Replay | Banner announced as a region and present on every screen; toggle is a radiogroup; no element named "live"; Replay steps announced politely |
| S11 | Command search | `Ctrl/Cmd+K`; type an ID | Dialog labelled; result count announced; Escape returns focus to the trigger |

Keyboard-only walkthrough (all routes, before any screen reader): Tab order matches reading order, focus always visible, no focus loss on state change, dialogs trap and restore, graph is operable, `Esc` closes overlays, no keyboard shortcut collides with assistive technology.

## 5. Schedule and responsibilities

| When | What | Owner | Evidence |
|---|---|---|---|
| M3 | Add `@axe-core/playwright` and Lighthouse a11y to the web test setup; run on the component gallery | Frontend lane | CI log |
| M6 | Console routes: axe zero serious and critical on every route and state; keyboard walkthrough; 320 px and 400 percent reflow per state | Frontend lane | Test report |
| M7 | Public routes: same; plus S1, S2, S10 screen-reader scripts | Frontend lane, independent reviewer | MANUAL results |
| Before RC | Full S1 to S11 on NVDA and VoiceOver, forced colors, 200 and 400 percent zoom, text spacing | Independent accessibility reviewer (MASTER section 1.1: visual and accessibility reviewer), integrator for data semantics | Signed checklist |
| Release | Criterion table above with a result per row | Owner decides GO or NO-GO | `docs/testing/` evidence files (M8 and M9) |

Reviewer independence: the author of a component does not sign off its walkthrough. A failing manual walkthrough blocks the route even if axe is clean.

## 6. Known risks to watch

- The topology graph and the Atlas are the highest-risk components for 2.1.1, 2.5.7, 1.1.1 and 4.1.3; their list and step-list alternatives are normative, not optional.
- Live updates can over-announce. The contract is one status region per page and announcements on state change only; verify with a real screen reader at M6.
- The token reveal combines a secret, a live region risk and a blocking dialog; keep the secret out of every live region and the document title.
- Prototype fonts are system fallbacks; contrast was computed on tokens, not on rendered font weights. Re-measure on rendered pixels once Plex and Newsreader are bundled.
