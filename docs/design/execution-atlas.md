# Execution Atlas: node and edge grammar

- Ticket: TASK-M2-FE-01, deliverable 5. Requirement IDs: FR-WEB-06, FR-WEB-07, FR-DEMO-01, NFR-A11Y-01, AC-14, AC-16, AC-23.
- Inputs: MASTER section 9 ("Execution Atlas"), `docs/architecture/state-machine.md` (states, events, lease and fence semantics), API contract `contract-m1-draft.4` (`Job`, `Attempt`, `Worker`, `WorkerDetail`), `route-inventory.md` (M-ATLAS, M-TOPO, S-STALE, S-RECONNECT).
- Status: **specified** and prototyped: the marketing Atlas is in `docs/design/prototypes/home.html` (script frames in `docs/design/screenshots/home-atlas-step5-1440.png` and `-step8-1440.png`), the console topology in `workers.html` and `overview.html`.
- Originality: the shapes below are original to CLUSTERX. They are chosen to be distinguishable by silhouette alone (no dependence on color), and to match engine vocabulary rather than any existing diagramming set.

## 1. Purpose and two modes

The Atlas draws the engine's own nouns so that a reader who learns the marketing diagram can read the console topology without relearning it.

| Mode | Where | Data | Label | Motion |
|---|---|---|---|---|
| **Illustrative** | `/` home (and the `/architecture` diagram uses the same shapes, static) | A deterministic in-repo script of eight steps | Persistent tag "Illustrative, scripted"; the word "live" never appears | M-ATLAS: user-driven, plays once on first view, never loops |
| **Data-backed** | `/app` overview, `/app/workers` | `GET /workers` snapshot (and `GET /jobs` active attempts via `WorkerDetail.active`), refreshed by hints plus reconciliation | Live indicator state and "as of" time; stale dims | M-TOPO: pulse only for a real running attempt |
| **Fixture** | `/demo` | Bundled fixtures | Fixed "Demo data" pill; no pulse | Replay is a labelled, user-started script |

The same grammar renders all three; only the data provider and the label differ (shared UI primitives, separate data providers, per MASTER section 4).

## 2. Nodes

All nodes are drawn with a 1.5 px outline, no gradients. Fill is white; state is shown by outline style, glyph and words.

| Entity | Shape (silhouette) | Default size | Content | Notes |
|---|---|---|---|---|
| **job** | Rounded rectangle (r 6) | 110 by 56 | Word "job", short ID (monospace), status written beneath in words | Status text beneath the node is the status of the job, one of the seven states. |
| **queue** | Rounded rectangle with three inner slots | 140 by 60 | Word "queue" above; filled slots show depth up to 3, then "+N" | Depth is a count from the snapshot, never an animation. |
| **attempt** | Numbered circle (r 19) | 38 diameter | Attempt number (1 to 5) inside; "attempt N" and "lease gen G" beneath | One circle per claim. Never reused. |
| **lease** | Ring (r 30) around an attempt, 3 px stroke | 60 diameter | The arc length encodes lease time remaining, clockwise from 12 o'clock | Full ring: fresh. Empty: expired. Never drawn without its attempt. Time remaining is also written as text in detail views. |
| **worker** | Chamfered rectangle (top-right and bottom-left corners cut) | 140 by 56 | Name, incarnation (monospace) | In the console, also state glyph, handler@version, active over capacity. |
| **recovery** | Diamond | 100 diagonal | Word "recover", "if budget left" beneath | A decision made by the engine: retry or fail by attempt budget. Not a state. |
| **result** | Rounded rectangle with a solid 8 px left spine | 120 by 60 | Word "result", outcome in words ("committed") | Success fill tint only after a committed result. |
| **coordinator** (console only) | Solid ink rounded rectangle | 120 by 68 | "coordinator", "claims via gRPC" | Hub that workers pull from; a context node, not an entity of the job lifecycle. |
| **cluster** (console only) | Rounded rectangle, wider than a worker | 250 by 78 | `handler@version cluster`, worker count, active attempts, counts by state | Stands for many workers. See section 7. |

Silhouette test: with all color removed, a reader can still tell the eight shapes apart (rounded block, slotted block, circle, ring, chamfered block, diamond, spined block, solid block).

## 3. Edges

Direction is always shown with an arrowhead at the destination. Line style encodes the *kind* of relationship, so style survives grayscale.

| Edge | Style | Meaning | Label |
|---|---|---|---|
| **Committed transition** | Solid 1.75 px ink, filled arrowhead | A state change that was committed in PostgreSQL (admit, claim, recover, result) | Verb word ("admit", "claim", "report") |
| **Renewal** | Dashed 5-4, accent | An ephemeral lease renewal (no event, no version bump) | "renew, +30 s" |
| **Held-by** | Thin solid 1.5 px ink-2 line, no arrowhead | An attempt belongs to a worker | none |
| **Expiry or rejection** | Dashed 5-4, danger, with a circled slash on rejections | The lease ran out, or a late or stale report was refused by the fence | "lease expired", "stale report, fenced off" |
| **Hint (console only)** | Dotted 1-3, ink-3, no arrowhead | A best-effort live notification; never drawn as a transition | "hint" in the legend only |

Rules: committed edges are the only ones that imply a durable fact. Renewals never imply progress of the job. A hint never changes a node's displayed state; only a snapshot does.

## 4. Statuses and labels

- **Job status** (beneath the job node): `scheduled`, `queued`, `running`, `retry_wait`, `succeeded`, `failed`, `canceled`, in words. `unschedulable` is **never a status**: it appears as a reason chip beside `queued` in the console and is not drawn in the marketing script.
- **Attempt state**: `active`, `succeeded`, `failed`, `lease_expired`, `deadline_exceeded`, `canceled`. Past attempts are dimmed to 38 percent opacity, not removed (history stays legible).
- **Worker state** (console): Online, Stale, Draining, Credential revoked, Deregistered, each with its glyph (`brand-and-tokens.md` section 6) and outline: solid (online), dashed (stale), dotted (deregistered), solid with shield glyph (revoked).
- **Highlight**: the node or edge involved in the current step gets an accent outline and tint; success results get a success outline; rejections a danger outline. Highlight always accompanies a text caption.
- **Text sizes**: node titles 16 px equivalent at 100 percent scale, sublabels 13.5 px mono. Minimum rendered size is not allowed below 12 px: the diagram scrolls horizontally in its own region instead of shrinking (min width 680 px).

## 5. Time context

Time is stated, not implied.

- A clock label in the top-left corner shows the scripted time ("t+30.4 s") in the marketing Atlas, updated per step. Real tick values are scripted and illustrative.
- Leases show seconds remaining as text in the console detail panel and the job page, counting down with text updates each second, **never announced to assistive technology**, and replaced by "unknown while offline" when the connection is lost (a countdown must never keep running as if current).
- Heartbeat age is written as "N s ago, as of hh:mm:ss"; it freezes when stale or offline.
- Timestamps follow the global `<Timestamp>` rule (local with zone, UTC on hover).
- Backoff and deadlines are written as durations ("backoff 2.4 s").

## 6. The eight-step marketing script

Deterministic, identical on every visit. Each step highlights specific elements and moves one token (a small accent dot) along a connector.

| Step | Time | Title | Elements highlighted or shown | Job status written | Caption (shown in a polite status region) |
|---|---|---|---|---|---|
| 1 | t+00 s | Admit | job, queue; edge admit | queued | The API commits the job to PostgreSQL before it answers. |
| 2 | t+00.4 s | Claim | edge claim; attempt 1, lease ring full; worker A | running | One transaction creates attempt 1 and lease generation 1. |
| 3 | t+12 s | Renew | dashed renew edge; ring stays full | running | Worker A extends its lease; heartbeat and renewal are separate. |
| 4 | t+14 s | Silence | worker A dims; ring drains to about half; "silent: no renewals" | running | Renewals stop. The handler may still be running; the engine cannot know. |
| 5 | t+30.4 s | Expire | ring empty; dashed expiry edge to recovery diamond | retry_wait | The database clock ends the lease; the attempt is consumed. |
| 6 | t+32.9 s | Recover | return edge to queue; attempt 2 on worker B, new ring (generation 2) | running | Budget remains, so the job is requeued and claimed again. |
| 7 | t+34.1 s | Commit | report edge to result; result gains success outline | succeeded | Worker B's result is accepted: attempt, generation and identity are current. |
| 8 | t+41.7 s | Fence | dashed rejected edge from worker A to result with circled slash | succeeded (unchanged) | Worker A's late report is rejected; the committed result stands; the handler ran twice. |

Behavior: Play runs steps 1 to 8 (about 2.4 s per step, token travel 600 ms per connector), Pause stops where it is, Step advances one step, Reset returns to step 0. It auto-plays **once** on first entering the viewport, never under reduced motion, never again unless the user presses Play, and pauses when the tab is hidden. Under reduced motion the token jumps to the end of the connector and all text is identical (step-through only).

The eight steps are always visible as an ordered list beside the diagram (and are the only content below 480 px); the list is the text alternative and carries `aria-current="step"`.

## 7. Console topology and clustering at 1, 10 and 100 workers

Data-backed from `GET /workers` (all pages up to a cap, de-duplicated by `id`, sorted client-side because the server order shifts with heartbeats). A worker's `version` bumps on status or incarnation change (not on heartbeat) and keys hint reconciliation.

| Fleet size | Graph rendering | List |
|---|---|---|
| **1** | The coordinator and one worker node, plus a detail line | One row |
| **2 to 12** | One worker node each, in a 3-column grid, connected to the coordinator; node shows name, state glyph and words, `active / capacity` | One row per node, row count equals node count |
| **13 to 100** | Handler clusters (`handler@version`), one per capability group; a worker advertising several capabilities is grouped by its first handler for layout and appears under every handler when the list is filtered by it; cluster node shows worker count, active attempts and counts by state (on, stale, draining, revoked) | Full table, 100 rows maximum rendered; selecting a cluster filters the table to that handler |
| **More than 100** | Same clusters, plus "Showing N of M; narrow by handler or state" (S-TOO-MANY); the topology never renders more than 12 element nodes and 12 clusters | Page cap with filter prompt (no load-everything) |

Rules:

- **Pulses** (M-TOPO) only on live worker nodes with a running attempt, at most 3 cycles per activation, suppressed at 100 or more workers, when the tab is hidden, and under reduced motion (a static "active" text badge replaces them).
- **Mixed health is visible on the cluster itself:** if any member is stale, draining or revoked the cluster outline is dashed and the counts say so; a cluster is never drawn healthy when a member is not.
- **Slot pips (specified, not in the prototype):** below a worker node's text, one 6 px square per capacity slot, filled for an active attempt; count also written as text. Prototypes show the text only.
- **Selection:** clicking or pressing Enter on a node selects it, outlines it in accent (2.5 px) and shows a one-line detail below the graph; the same entry is highlighted in the list (synchronized in both directions, by worker ID).
- **Zoom and pan:** buttons for zoom in, zoom out, four pan directions and reset; range 25 percent to 200 percent of the base view. No pinch, wheel or drag is required (WCAG 2.5.7); pointer drag and wheel may be offered as extras.
- **Detail focus:** selecting a worker opens its detail line; "Open worker" goes to `/app/workers/[id]`.
- **Keyboard:** every node is a focusable button; arrow keys move between nodes in layout order; Enter or Space selects. Focus is visible (3 px accent outline on the node body) and never hidden.
- **Reconnection:** the diagram never renders from a hint. On reconnect it keeps the old picture dimmed (opacity 0.45) with "as of hh:mm:ss" and the Reconnecting indicator until a snapshot is reconciled by version; only then does it return to full strength and Live.
- **Stale:** the same dimming and label, plus heartbeat ages frozen.
- **No workers:** an empty state (S-EMPTY-FIRST) with a link to the worker docs and (operators) to credentials; if all are offline, a critical notice with a link to unschedulable jobs.

## 8. Synchronized list equivalent

The list is the primary representation; the graph is the enhancement. Both are always available (default is List). They share:

- the same entries and counts (row count equals node count for 1 to 12 workers; cluster totals equal the sum of member rows);
- the same selection (by ID), the same filters (state, handler) and the same stale and offline labels;
- the same glyphs and words for state, so nothing is color-only.

The graph's accessible name states the count and refers to the list ("Worker topology: 100 workers, grouped into handler clusters. The List view has the same entries."). Switching views preserves selection and filters; the choice persists as a local preference and defaults to List below 768 px.

## 9. Illustrative versus live: marking rules

| Surface | Required marking |
|---|---|
| Marketing Atlas | Tag "Illustrative, scripted" on the card header; the clock reads "t+..." not a wall time; no element named or styled as live |
| Console topology | Live indicator in the topbar plus "as of hh:mm:ss"; Stale or Offline dims the graph |
| `/demo` | Fixed "Demo data" pill replaces the live indicator; banner "Illustrative demo: seeded fixture data, read-only, not live"; no pulses; Replay is user-started |
| Screenshots in docs | Caption states the source (prototype, fixture, or captured build with version) |

## 10. Data mapping (console)

| Atlas element | API field |
|---|---|
| job status text | `Job.status` |
| attempt circle and number | `Attempt.attempt_number`, `Attempt.state` |
| lease ring | `Attempt.lease_expires_at` against `as_of` (DB clock) |
| lease generation | `Job.lease_generation`, `Attempt.lease_generation` |
| worker node | `Worker.name`, `status`, `live`, `capacity`, `active_attempts`, `capabilities`, `credential_revoked`, `last_seen_at` |
| active attempts | `WorkerDetail.active[]` |
| rejection edge | Diagnostic `/audit` rows for the job or worker (operators) |
| stale or freshness | `as_of` on every snapshot |

## 11. Performance and verification (not yet measured)

No budget is frozen yet (MASTER section 10). Intended checks: render time and interaction latency for the 100-worker graph, with the list as fallback if a budget is missed; DOM node count at 100 workers (graph at most 12 clusters plus 1 hub); a screenshot comparison at 320 to 1920 px; keyboard walk of the graph; axe on the SVG. All **NOT RUN** in M2: only the prototype renders were produced (`render-check.json` records overflow, console errors and external requests, not graph performance).
