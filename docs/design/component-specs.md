# CLUSTERX Component Specifications

- Ticket: TASK-M2-FE-01, deliverable 3. Requirement IDs: FR-WEB-04, FR-WEB-05, FR-WEB-07, NFR-A11Y-01, NFR-RESP-01, AC-21, AC-22, AC-23.
- Scope: every item in the MASTER section 8 "Shared UI component inventory". This document is the M2 specification. The components are implemented in `packages/ui` (M3 onward); where code and this text differ, the code is what ships and this text is not yet updated.
- Inputs: `tokens.css`, `route-inventory.md` (state IDs `S-*`, motion IDs `M-*`), `motion-and-assets.md`, `accessibility-plan.md`.
- Status: **specified**. Prototypes in `docs/design/prototypes/` realise the components marked "P" in the index; they are static HTML, not the React components.

## 0. Rules for every component

1. **Native first.** Use the native element (`button`, `a`, `input`, `select`, `dialog`, `details`, `table`) before ARIA. ARIA only fills a gap native HTML cannot.
2. **States.** Each interactive component defines default, hover, focus-visible, active, disabled, loading and error. Hover never carries information; focus-visible is always shown (2 px `--color-focus` ring, 2 px offset, 3:1 against the surface, never obscured by a sticky bar).
3. **Targets.** Minimum 24 by 24 CSS px (WCAG 2.5.8); console controls are 32 to 40 px, touch contexts 44 px.
4. **Status is text plus glyph**, never color alone (`brand-and-tokens.md` section 6).
5. **Critical actions never wait for an animation.** Focus handling is synchronous; animation is decoration (M-* in `motion-and-assets.md`).
6. **Every control works or is omitted.** No disabled placeholder for an unbuilt feature. A control that is temporarily unavailable (offline, wrong role state) is `aria-disabled="true"` with the reason in visible text.
7. **Disabled vs absent.** Role-denied actions are absent. State-denied actions (for example Drain on a revoked worker) are shown disabled with a reason.
8. **Keyboard.** No keyboard traps except modal dialogs (which restore focus on close). No single-character shortcuts. The only global shortcut is `Ctrl/Cmd+K`.
9. **Reflow.** Everything works at 320 CSS px with no page-level horizontal scroll. Two-dimensional content (code, JSON, wide diagrams) scrolls inside its own focusable region (`tabindex="0"`, labelled).

Index of components (P = realised in a prototype):

| # | Component | P |
|---|---|---|
| 1 | Brand shell, navigation, footer | home, login |
| 2 | Docs sidebar, search, on-this-page | |
| 3 | App sidebar, topbar, breadcrumbs, command search | overview, jobs, job-detail, workers |
| 4 | Text input, select, textarea and JSON editor, date-time picker, filter chips and tabs | jobs, login |
| 5 | Button (loading, success, disabled, focus) | all |
| 6 | Tooltip, popover, dialog, toast, drawer | job-detail, all drawers |
| 7 | Status badge and legend | all |
| 8 | Table, virtualized list, pagination | jobs, workers, overview |
| 9 | Chart and accessible data alternative | overview |
| 10 | Topology graph and list mode | overview, workers |
| 11 | Job timeline and attempt cards | job-detail |
| 12 | Token one-time reveal | |
| 13 | Code block and copy snippet | home |
| 14 | Skeleton, empty, error, offline and reconnect states | jobs, overview, workers |
| 15 | Live status indicator | all console |
| 16 | Timestamp | all |
| 17 | Masked field | job-detail |
| 18 | Notice (inline banner) | all |
| 19 | Execution Atlas (marketing) | home |

---

## 1. Brand shell, navigation, footer

**Anatomy.** Skip link, header (wordmark link, primary `nav`, Sign in button, Menu button below 768), `main`, footer (four link groups, release status, UTC note).

| State | Spec |
|---|---|
| Default | Header 64 px, 1 px bottom rule, white. Nav links 14 px ink-2. |
| Hover | Link background `--color-panel`, text ink (M-HOVER 150 ms). |
| Focus | Standard ring. Skip link appears at top-left on focus. |
| Active (current page) | `aria-current="page"` plus 2 px accent underline. |
| Disabled | None. |
| Loading | None (server rendered). |
| Error | None. Footer shows release status text, never a fabricated version. |

**Keyboard.** Tab order: skip link, wordmark, nav links, Sign in, then main. Menu button opens the drawer dialog; `Esc` closes; focus returns to the button.
**ARIA.** `header`, `nav aria-label="Primary"`, drawer `dialog aria-label="Site menu"`, footer `nav`s each labelled. Current page via `aria-current`.
**Responsive.** At 768 and up inline nav; below, Menu button plus drawer. 320 px: wordmark and Menu fit on one line.
**Motion.** M-OVERLAY for the drawer, M-HOVER for links, M-ROUTE on navigation.

## 2. Docs sidebar, search, on-this-page

**Anatomy.** Sidebar tree (eight items), search trigger, article, on-this-page rail, prev/next, edit link.

| State | Spec |
|---|---|
| Default | Sidebar 15 rem, sticky under the header. |
| Hover/focus | As nav links. |
| Current | `aria-current="page"` in the sidebar; `aria-current="true"` on the scroll-spied anchor, focus never moved. |
| Search loading | Popover shows S-LOAD skeleton lines. |
| Search empty | "No matches" plus link to `/docs`. |
| Search error | S-ERR-500 variant, then a plain list of all docs. |
| Offline | Search works once the index is cached. |

**Keyboard.** `Ctrl/Cmd+K` or the Search button opens; arrow keys move through results (combobox pattern with `aria-activedescendant`); `Enter` follows; `Esc` closes and restores focus. No single-key shortcuts.
**ARIA.** `role="search"` landmark; `combobox` + `listbox`; result count announced politely once per query pause (debounced 400 ms); on-this-page is `nav aria-label="On this page"`.
**Responsive.** Below 1024 the sidebar becomes a "Docs menu" drawer and on-this-page a `details` disclosure at the top.
**Motion.** Docs body does not animate text (reading surface). M-HOVER and M-OVERLAY only.

## 3. App sidebar, topbar, breadcrumbs, command search

**Anatomy.** Sidebar (wordmark, `Main` and `Settings` link groups, user block with Sign out), topbar (menu button below 1024, breadcrumb, command button, live indicator, role banner), command dialog.

| State | Spec |
|---|---|
| Default | Sidebar 240 px docked at 1024 and up. Nav items 36 px high. |
| Hover | Panel background (M-HOVER). |
| Current | Accent tint, 2 px accent inset edge, `aria-current="page"`. Not color alone: weight and edge. |
| Focus | Standard ring, inset on dense lists. |
| Role | VIEWER: Tokens item absent; banner "Read-only access (viewer)" persistent. |
| Command: empty query | Page list. |
| Command: ID match | "Open job ..." result from the exact-ID lookup. |
| Command: error | 404 "No job or worker with that ID"; 410 expired notice; 503 S-ERR-503 inline. |

**Keyboard.** `Ctrl/Cmd+K` opens the command dialog from anywhere; arrow keys through results; `Enter` opens; `Esc` closes; focus returns to the trigger. Sidebar is a plain list of links, Tab order follows DOM.
**ARIA.** `nav aria-label="Console"`; topbar breadcrumb `nav aria-label="Breadcrumb"` with `ol`; command is a `dialog` with a labelled input and a live result count.
**Responsive.** Below 1024 the sidebar is a drawer from the menu button (focus trap, `Esc`, return focus). Command button shows only the icon and `Ctrl K` hint at narrow widths but keeps its accessible name "Jump to a job or worker by ID".
**Motion.** M-OVERLAY for the drawer and dialog; M-ROUTE on navigation.

## 4. Form controls

### 4.1 Text input, select

**Anatomy.** Visible label above, control, help text, error text. Required marked in words ("required"), not only an asterisk.

| State | Spec |
|---|---|
| Default | 40 px, 1 px `--color-line-strong` border, 16 px text on mobile (prevents zoom). |
| Hover | Border darkens to ink-2. |
| Focus | Standard ring. |
| Disabled | `disabled` attribute plus visible reason in help text. |
| Error | Border `--color-danger`, error text below with danger glyph, `aria-invalid="true"`, `aria-describedby` the error. |

**Keyboard.** Native. **ARIA.** `label for`, `aria-describedby` for help and error. A form-level error summary (`role="alert"`) lists errors as links to fields and takes focus on a failed submit (S-422); entered values are preserved (WCAG 3.3.7).
**Responsive.** Full width below 768; filter fields wrap in a flex row above.

### 4.2 Textarea and JSON editor (job payload)

**Anatomy.** Label, editor, toolbar (Format, wrap toggle), size meter ("12 B of 64 KiB", text value), validation list, schema help (disclosure below 1024).

| State | Spec |
|---|---|
| Default | A native `textarea` with monospaced text; a richer editor may layer on top only if it keeps native semantics. |
| Loading | The plain textarea renders first; the richer editor loads lazily so submit never depends on a heavy bundle. |
| Error | Inline parse error with line and column, schema violations listed in a summary region, editor flagged as a whole until the API defines per-field paths (GAP-ERR-02). |
| Over limit | Meter turns to danger text plus icon, submit disabled with reason "Payload is 70 KiB; the limit is 64 KiB". |

**Keyboard.** Tab moves out of the editor (no trap); a visible hint states how to insert a literal tab if a rich editor is used. **ARIA.** Label, `aria-describedby` size meter and help. **Responsive.** In-region horizontal scroll with a wrap toggle; never a page scroll.

### 4.3 Date-time picker

Native `input type="datetime-local"` with an explicit timezone label beside it ("Time zone: Europe/London, UTC+00:00"), a "Run now" default, and the maximum allowed value (now plus 30 days) shown. Errors: "Choose a time within 30 days." DST ambiguous or non-existent local times show the resolved UTC value before submit. No custom calendar widget in 1.0.

### 4.4 Filter chips and tabs

**Chips.** Removable chip = text plus a 24 px remove button with `aria-label="Remove <filter> filter"`; a "Clear all filters" button appears when more than one chip is active. Chips are not the only way to set a filter (the form is). **Tabs and segmented controls.** Where a view switch is not navigation (Chart/Table, Graph/List) use a group of toggle buttons with `aria-pressed` and a labelled `role="group"`; where it is navigation use links. Roving focus is not required for two-option toggles.

## 5. Button

| State | Spec |
|---|---|
| Default | 40 px (32 small), 6 px radius. Primary = accent fill, white text; secondary = white with strong border; danger = white with danger border and `-ink` text, solid danger only inside a confirm dialog; ghost for tertiary. |
| Hover | Fill darkens one step (150 ms, color only). |
| Focus | Standard ring. |
| Active | One step darker than hover; no scale. |
| Disabled | `aria-disabled="true"` (stays focusable when the reason matters) with a visible reason; `disabled` only when no reason needs reading. |
| Loading (S-PENDING) | Spinner plus changed text ("Redirecting to GitHub"), `data-pending`, `aria-disabled`, repeat clicks ignored. Never silently disabled. |
| Success (S-SUCCESS) | Inline confirmation region (polite live) beside or below, not a color flash. A toast may supplement, never replace it. |
| Error | The control stays enabled; the error is shown in a notice near the action. |

**Keyboard.** `Enter` and `Space`. **ARIA.** Buttons that open dialogs set `aria-haspopup="dialog"`; toggles use `aria-pressed`; icon-only buttons have an `aria-label`. **Responsive.** Full width in stacked CTA rows below 480.

## 6. Tooltip, popover, dialog, toast, drawer

| Component | Spec |
|---|---|
| **Tooltip** | Supplementary only (never the sole carrier of required information or an instruction). Shows on hover and focus, is hoverable, dismissible with `Esc`, persists until dismissed or focus leaves (WCAG 1.4.13). `role="tooltip"`, referenced by `aria-describedby`. Not used on touch; the same text is also available in-page. Used for the UTC value of a timestamp and the long form of an abbreviation. |
| **Popover** | Native `popover` or a disclosure for non-modal detail (reason explanation on an unschedulable chip). Closes on `Esc`, outside click, and focus leaving; focus returns to the trigger. |
| **Dialog** | Native `dialog` opened with `showModal()`: focus trap, `Esc` closes, focus restored to the trigger. Title via `aria-labelledby`; consequence text via `aria-describedby`. Initial focus on the safe action ("Keep running", "Not now") for destructive or consequential confirms. Footer buttons right-aligned, primary last. Width at most 32 rem, full-screen sheet below 480 for the token reveal. Backdrop click closes unless the dialog is blocking (reveal, session-expired). |
| **Toast** | Supplementary confirmation only. `role="status"`, visible 4 s minimum plus 1 s per 20 words, pause on hover and focus, never the sole signal of an outcome, never holds a secret. At most one at a time. |
| **Drawer** | A dialog edge-anchored left (menus) or bottom (filters, tour sheet on mobile). Same focus rules as dialog. |

**Session-expired dialog (S-SESSION-EXPIRED).** Non-dismissible, focus-trapped, primary action "Sign in again". **Alert dialog.** `role="alertdialog"` is reserved for the token reveal and destructive confirms that need an immediate answer.
**Motion.** M-OVERLAY: 200 ms in, 160 ms out, opacity plus 0.98 to 1 scale (drawers translate 16 px). Reduced motion: opacity only, at most 80 ms or instant. Focus never waits for the animation.

## 7. Status badge and legend

**Anatomy.** Pill: glyph (16 px, silhouette per state), text, tint. A legend is a list of every glyph with its words, shown wherever more than one state can appear (jobs list, overview KPIs via labels, docs concepts, demo).

| State | Spec |
|---|---|
| Default | Per `brand-and-tokens.md` section 6: seven job states, worker states, live states. |
| Hover/focus | Not interactive; where a badge opens an explanation (unschedulable reason), the whole chip is a button with the standard ring. |
| Forced colors | Border `CanvasText`; text and glyph unchanged. |

**ARIA.** Plain text content; the glyph is `aria-hidden`. **Rule.** The canonical legend lives on `/docs/concepts` and is reused. `blocked` and `skipped` never appear. Unschedulable is a reason chip beside the `Queued` badge, never a status.

## 8. Table, virtualized list, pagination

**Anatomy.** `table` with `caption` (count, window, ordering), `thead` with `th scope="col"`, row link as a real anchor on the primary cell, status cell, numeric cells right-aligned and tabular, optional "meta" lines below a cell.

| State | Spec |
|---|---|
| Default | 44 px rows, `--color-panel` header with 1 px rules. |
| Hover | Row background canvas; no whole-row click trap. |
| Focus | Ring on the link or control inside the cell. |
| Selected (list/graph sync) | Accent tint row plus `aria-selected` where the table is a grid; otherwise `data-sel` with a visible text cue. |
| Loading (S-LOAD) | Fixed number of skeleton rows; on filter change, previous rows stay dimmed with `aria-busy="true"` rather than blanking. |
| Empty | S-EMPTY-FIRST or S-EMPTY-FILTER, distinct. |
| Error | S-ERR-500, 503 or timeout region with retry; filters preserved. |
| Stale | "as of" in the caption; rows dim to 0.45 for charts only, tables stay readable. |
| Live update | M-LIVE: changed rows flash at most once per 2 s; reorder only on explicit Refresh. |

**Mobile (below 640 px).** The table becomes labelled stacked cards: header row visually hidden but present, each cell prefixed by its column name, same fields in the same order, primary cell (ID plus status) first. No data hidden.
**Sortable headers** (only where the API supports sort, or client-side over a fully fetched bounded set): `button` inside `th` with `aria-sort`. Jobs list has no sort control (GAP-SORT-01).
**Pagination.** `nav aria-label="Pagination"` with Previous and Next buttons (opaque cursors: no jump to page N), disabled-with-reason ("first page", "no more"), page size stated. DOM cap: 100 rows per page and 500 accumulated, then S-TOO-MANY. **Virtualization** only if measured budgets prove it necessary; native table semantics are kept otherwise.
**Keyboard.** Native tab order through links and controls. **Responsive.** Wide tables scroll inside `.tbl-wrap` only between 640 and the content width.

## 9. Chart and accessible data alternative

**Anatomy.** Card with heading, Chart/Table toggle, SVG chart, legend, freshness note, data table.

| State | Spec |
|---|---|
| Default | SVG with `role="img"`, `<title>`, `<desc>` summary (range, min, max, last, trend), `aria-labelledby`. Encoding per `brand-and-tokens.md` section 7. |
| Loading | Skeleton at the chart's final height (no layout shift). |
| No data | S-NO-METRICS three variants: no data in range, source unavailable, not yet aggregated. Never zeros. |
| Stale/offline | Dim to 0.45 opacity and "as of" under the chart (S-STALE). |
| Error | S-ERR-503 or timeout region with retry; other charts on the page keep working. |

**Alternative.** The table under the chart contains every plotted value and is the interactive alternative (keyboard-focusable cells, no hover dependency). A client-side CSV download of the displayed series is offered on Observability. **Keyboard.** Chart/Table toggle buttons; table reachable by Tab. **Responsive.** Chart scrolls in its own region below 480 only if labels cannot fit; table stacks. **Motion.** Data swap is instant; no animated redraw.

## 10. Topology graph and list mode

Companion: `execution-atlas.md` (grammar), section 8 (console topology).

**Anatomy.** Coordinator node, worker nodes (or handler clusters at 13 or more), connectors, zoom and pan buttons, selection detail line, legend, synchronized table (List mode).

| State | Spec |
|---|---|
| Default | List is the default view. Graph is an enhancement with the same entries. 1 worker: one node plus detail. 10: all nodes. 100: handler clusters with counts. |
| Node: online / stale / draining / revoked / deregistered | Glyph plus state words plus outline (solid, dashed for stale, dotted for deregistered). |
| Selected | 2.5 px accent outline plus the detail line below the graph; the same row is highlighted in the list. |
| Active attempt | Pulse ring (M-TOPO), only on live nodes with a running attempt, at most 3 cycles per activation, hidden at 100 or more workers and when the tab is hidden. Reduced motion: a static text badge "active". |
| Stale data | Graph dims; "as of" shown. Reconnect never leaves a convincing stale diagram. |
| Cluster | Shows worker count, active attempts and live/stale/draining/revoked counts; selecting it filters the list to that handler. |

**Keyboard.** Each node is a focusable button (`tabindex="0"`, `role="button"`, full `aria-label`); arrow keys move between nodes, `Enter` or `Space` selects. **Pointer alternatives (WCAG 2.5.7).** Zoom in, zoom out, pan four directions and reset are buttons; no drag, pinch or wheel is required.
**ARIA.** SVG `role="img"` with a summary label naming the count and pointing to the List view; the List is the full alternative.
**Responsive.** Below 768 the graph is replaced by summary counts plus the list (no hidden essential data). Graph scrolls inside its region.
**Reconciliation.** Selection survives refetch by worker ID; a vanished ID clears the selection with a polite message.

## 11. Job timeline and attempt cards

**Attempt card.** `article` with `h3` ("Attempt 2"), state badge, retryable/permanent badge, worker and incarnation, start and end time, error (RBAC), log tail (RBAC, bounded 16 KiB, truncation text), lease generation.

| State | Spec |
|---|---|
| Active | Running badge, lease expiry countdown text updated each second and **not announced**; replaced by "unknown while offline" when offline. |
| Failed/expired | Glyph plus words ("Lease expired", "Failed"). |
| Log: output / empty / masked / expired | Four distinct renderings: code well; "No log output"; "Hidden for viewer role"; "Log removed by the 7-day log retention". |
| Truncated | Text marker "Log truncated at 16 KiB". |

**Timeline.** Ordered list, oldest first, each entry: version chip (monotonic per job), event type in words, transition, actor, `<time>`. Marker shapes differ (circle normal, square failure, diamond warning, dashed rule for diagnostic) in addition to color. Diagnostic entries (from the audit log, operators only) are visually separate and labelled. A gap in versions triggers a refetch; "Load older events" is a button with a disabled-with-reason state. **Keyboard.** Native. **Responsive.** Vertical on all widths.

## 12. Token one-time reveal

**Anatomy.** `alertdialog` titled "Copy this secret now: it is shown once", read-only labelled text field holding the secret (`autocomplete="off"`, `spellcheck="false"`, wraps, never truncated), Copy button, acknowledgment checkbox "I have stored this secret", Close (disabled until acknowledged).

| State | Spec |
|---|---|
| Default | Dialog is blocking: no backdrop dismissal, no `Esc` close before acknowledgment. |
| Copied | Polite status "Copied". The secret is never placed in a live region, title, toast, URL, history or storage. |
| Session expiry while open | The dialog persists until acknowledged; the secret is never re-shown after close. |
| After close | The list shows only the non-secret prefix; no way to re-reveal. |
| Create outcome unknown | List refresh with the guidance "If a credential appeared, its secret cannot be shown again: revoke it and create a new one." |

**Keyboard.** Focus starts on the secret field (select-all on focus); Tab cycles Copy, checkbox, Close. **Responsive.** Full-screen sheet below 480 with Copy reachable without scrolling.

## 13. Code block and copy snippet

**Anatomy.** Header (language label, Copy button), `pre` with `code`, optional "Expected output" block labelled as such. Dark surface by design (`--color-code-*`, all at least 4.5:1).

| State | Spec |
|---|---|
| Default | Mono 13 px, 1.65 line height, command prompts excluded from copy. |
| Hover/focus | Copy button standard states; the `pre` is focusable (`tabindex="0"`, labelled) so keyboard users can scroll it. |
| Copied | Button text changes to "Copied" for 1.8 s and a polite status announces it. |
| Copy blocked | Polite message "Copy blocked. Select the text and copy manually."; text is selectable. |
| Unverified | A notice above any command block states "Not yet verified from a clean checkout" until the AC-26 evidence exists. |

**Responsive.** Scrolls inside its own region; never forces page width.

## 14. Skeleton, empty, error, offline and reconnect states

| Component | Spec |
|---|---|
| **Skeleton (S-LOAD)** | Shape-matched to the final layout, shown only after 200 ms, `aria-busy="true"` on the region, 1 s opacity pulse that stops at 10 s (becomes S-ERR-TIMEOUT) and is static under reduced motion. |
| **Empty (S-EMPTY-FIRST)** | Title, one sentence on what creates data, role-aware primary action, docs link. Never a cartoon. |
| **Empty filtered (S-EMPTY-FILTER)** | Shows active filters as removable chips and a "Clear filters" button; positive framing for the dead-letter view ("No failed jobs in this window"). |
| **Error (S-ERR-*)** | Plain-language cause, `request_id` with copy button, recovery action, link to docs. `role="alert"` only for blocking errors. Copy comes from the `code`, not the raw message. |
| **Offline (S-OFFLINE)** | Banner, last data labelled "as of", mutations disabled with reason, auto-recover on `online`. |
| **Reconnect (S-RECONNECT)** | Live indicator changes; polling fallback engaged; forced snapshot before returning to Live. |
| **Expired data (S-EXPIRED-DATA)** | "Expired by retention" with finished and removed times and the retention window; distinct from not found. |

## 15. Live status indicator

One component, six console states plus the demo pill: Live, Reconnecting, Polling (interval in text), Stale, Offline, Paused (tab hidden). `role="status"` with an `aria-label`; announces politely **only on state change, never per event**. Glyph plus text (`brand-and-tokens.md` section 6). In the public demo it is replaced by a fixed "Demo data" pill and nothing in the demo is labelled Live.

## 16. Timestamp

`<time datetime="UTC ISO">` showing user-local time with explicit zone (for example `14:42:07 GMT+4`); title and copy give UTC; relative time ("3 s ago") is supplementary and paired. Frozen when offline or stale ("as of"). Public pages that show dates use UTC labelled "UTC". Zone preference lives in `/app/settings/preferences` (local storage) and falls back to the browser zone; DST ambiguity shows the offset.

## 17. Masked field

Inline placeholder with a padlock and the words "Hidden for viewer role", dashed outline on panel. Never blank, never `null`-looking, never a fake value. Keys only on the API's `masked_fields`; distinct from empty ("Not produced yet", "No log output") and expired. Raw values are never in the DOM for viewers (DOM canary test AC-25).

## 18. Notice (inline banner)

Left icon plus bold lead sentence plus detail plus optional actions. Variants: info, success, warning, error, each with an icon and a lead phrase that stands without color. `role="status"` for non-blocking, `role="alert"` for blocking errors; never both. Dismissal is not offered for conditions that still hold (stale, offline, viewer banner).

## 19. Execution Atlas (marketing)

Full specification in `execution-atlas.md`. Component contract: card with a "Illustrative, scripted" tag, SVG diagram (`role="img"`, `<title>`, long `<desc>`), controls (Play/Pause, Step, Reset: native buttons with `aria-pressed` on Play), a polite caption region with the current step, an always-visible ordered list of the eight steps with `aria-current="step"`, a legend. Auto-plays once on first entering the viewport (not under reduced motion), never loops, pauses when scrolled out of view or the tab is hidden. Below 480 px the diagram is hidden and the step list is the content.
