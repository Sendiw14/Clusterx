# CLUSTERX Brand and Design Tokens

- Ticket: TASK-M2-FE-01, deliverable 1. Lane: frontend. Author model: `claude-sonnet-5-5`, effort high requested (effective effort not observable from inside the session, MW-04: UNVERIFIED).
- Contract: `docs/product/MASTER_PROMPT.md` R2 section 9 (identity), section 10 (accessibility). API contract base `contract-m1-draft.4` (c48b62f88ac76337f43a21e2adb08ae6dccc5ab2). Nothing in this file touches the API.
- Token source of truth: `packages/ui/tokens/tokens.css`. Every number in the contrast table below is computed from that file by `docs/design/tools/contrast.mjs`; the table is regenerated, not hand-typed.
- Status: **specified** (design). Visual review by an independent reviewer is still required before it is **approved**.

## 1. Identity: Precision Under Motion

CLUSTERX is a distributed job engine, so the identity is about exact things that change over time: leases, attempts, clocks, fences. White is the ground. Structure comes from one-pixel rules, not from shadows or tinted cards. Three recurring devices carry the brand:

| Device | Where | Rule |
|---|---|---|
| **Tick rule** | Section dividers on public pages, timelines | A one-pixel ink baseline with short ticks every 8 px, like a ruler or a time axis. Decorative: `aria-hidden`, never conveys data. |
| **Mono label** | Eyebrows, ledger tables, IDs, step numbers | Uppercase mono at 12 px with 0.08 em tracking, always preceded by a short rule. Used for labels only, never for sentences. |
| **Fenced plate** | Hero figure, atlas card, preview frame | A 1 px ink border with a second 1 px offset border behind it (6 px down and right). Used at most once per viewport. |

Rules that keep it original and uncluttered:

- One accent (cobalt `#2457E7`). Success, warning and danger are status colors, not decoration.
- Large display type is a serif (Newsreader) used only for public hero and page titles; everything else is a grotesque sans (IBM Plex Sans) with mono (IBM Plex Mono) for technical values. The console uses sans and mono only.
- No gradients, glass, glow, neon, dark theme, stock illustrations or emoji. The only dark surfaces are code blocks, the prototype label bar and the footer call-to-action band, each used deliberately as contrast.
- Depth: borders first. The only shadow in the system lifts overlays (dialogs, drawers, toasts).
- Corners are small (6 px default). Pills are reserved for status chips and the live indicator.
- The wordmark is the text CLUSTERX in 600 weight with 0.14 em tracking beside a mark: a 2 by 2 grid of squares whose fourth square is detached and dashed, meaning "the unit that moved". The mark is original geometry (`docs/design/assets/mark.svg`).

Originality check: no layout, illustration, wording or motion is taken from another product. The Execution Atlas grammar (`execution-atlas.md`) is original; the fenced plate, tick rule and mono labels are drawn from general editorial and engineering-drawing conventions, not from a specific site. The icon set is built from primitive shapes in this repository (section 8 of `motion-and-assets.md`).

## 2. Color tokens

Starting values are the section 9 palette. Two starting values fail AA as **text** on white and were split into a graphic token and a text token. Measured results are in section 3.

| Token | Hex | Role |
|---|---|---|
| `--color-white` | `#FFFFFF` | Primary surface, cards, inputs |
| `--color-canvas` | `#F7F9FC` | Console page background behind white cards, alternate public sections |
| `--color-panel` | `#EEF2F7` | Table headers, code wells on light, neutral status tint |
| `--color-ink` | `#111827` | Primary text, ink borders on fenced plates |
| `--color-ink-2` | `#475569` | Secondary text |
| `--color-ink-3` | `#5B6678` | Meta text, timestamps (still at least 4.5:1 on white, canvas and panel) |
| `--color-ink-disabled` | `#8A94A6` | Disabled controls only (WCAG exempts inactive components; the disabled state is also stated in text or `aria-disabled`) |
| `--color-line` | `#E2E8F0` | Decorative dividers and card borders. Never the only boundary of an input or button. |
| `--color-line-strong` | `#7C8A9E` | Control boundaries (inputs, buttons, chips, seg controls): at least 3:1 |
| `--color-accent` | `#2457E7` | Primary action, links, focus ring |
| `--color-accent-ink` | `#1B46C2` | Hover and text on accent tint |
| `--color-accent-tint` | `#E8EEFD` | Selected row, current nav item, running chip |
| `--color-success` / `-ink` / `-tint` | `#0C8A63` / `#0A7050` / `#E5F4EE` | Base for glyphs and chart lines only; `-ink` for text |
| `--color-warning` / `-ink` / `-tint` | `#A96B08` / `#8A5806` / `#FBF0DC` | Same split |
| `--color-danger` / `-ink` / `-tint` | `#C33444` / `#A82A39` / `#FBE8EA` | Base passes as text on white and canvas; `-ink` is used on tints and panel |
| `--color-neutral` / `-tint` | `#64748B` / `#EEF2F7` | Glyphs for scheduled, queued, canceled; text on neutral uses `--color-ink-2` |
| `--color-focus` | `#2457E7` | 2 px focus ring, 2 px offset, on light surfaces |
| `--color-focus-on-dark` | `#9DB7FF` | Focus ring inside code blocks, prototype bar, footer band |
| `--color-code-*` | bg `#0F172A`; ink `#E2E8F0`; mute `#94A3B8`; key `#9DB7FF`; string `#7FD6B0`; number `#F2C57C`; error `#FF9AA5` | Code block syntax; all at least 4.5:1 on the code background |

60-30-10 distribution on a console page: about 60 percent white and canvas, 30 percent ink and secondary text with line, 10 percent accent plus status.

## 3. Measured contrast

Command (exit 0 means every required pair passes):

```
PATH=/opt/homebrew/opt/node@26/bin:$PATH node docs/design/tools/contrast.mjs
```

Method: WCAG 2.2 relative luminance (sRGB, 0.04045 threshold), ratio `(L1+0.05)/(L2+0.05)`. Required thresholds: 4.5:1 for normal text, 3:1 for non-text UI and graphical objects. No large-text relaxation is used anywhere: every text pair is held to 4.5:1. The raw run is saved at `docs/design/evidence/contrast-output.md`.

<!-- CONTRAST:BEGIN -->
### Required pairs

| Kind | Foreground token | Hex | Background token | Hex | Ratio | Required | Result | Use |
|---|---|---|---|---|---|---|---|---|
| text | `color-ink` | #111827 | `color-white` | #FFFFFF | 17.74 | 4.5:1 | PASS | primary text |
| text | `color-ink` | #111827 | `color-canvas` | #F7F9FC | 16.82 | 4.5:1 | PASS | primary text |
| text | `color-ink` | #111827 | `color-panel` | #EEF2F7 | 15.78 | 4.5:1 | PASS | primary text |
| text | `color-ink-2` | #475569 | `color-white` | #FFFFFF | 7.58 | 4.5:1 | PASS | secondary text |
| text | `color-ink-2` | #475569 | `color-canvas` | #F7F9FC | 7.18 | 4.5:1 | PASS | secondary text |
| text | `color-ink-2` | #475569 | `color-panel` | #EEF2F7 | 6.74 | 4.5:1 | PASS | secondary text |
| text | `color-ink-3` | #5B6678 | `color-white` | #FFFFFF | 5.81 | 4.5:1 | PASS | tertiary/meta text |
| text | `color-ink-3` | #5B6678 | `color-canvas` | #F7F9FC | 5.51 | 4.5:1 | PASS | tertiary/meta text |
| text | `color-ink-3` | #5B6678 | `color-panel` | #EEF2F7 | 5.16 | 4.5:1 | PASS | tertiary/meta text |
| text | `color-accent` | #2457E7 | `color-white` | #FFFFFF | 5.85 | 4.5:1 | PASS | links, accent text |
| text | `color-accent` | #2457E7 | `color-canvas` | #F7F9FC | 5.54 | 4.5:1 | PASS | links, accent text |
| text | `color-accent` | #2457E7 | `color-panel` | #EEF2F7 | 5.20 | 4.5:1 | PASS | links, accent text |
| text | `color-accent` | #2457E7 | `color-accent-tint` | #E8EEFD | 5.03 | 4.5:1 | PASS | links, accent text |
| text | `color-accent-ink` | #1B46C2 | `color-white` | #FFFFFF | 7.77 | 4.5:1 | PASS | hover/active links, chips |
| text | `color-accent-ink` | #1B46C2 | `color-canvas` | #F7F9FC | 7.37 | 4.5:1 | PASS | hover/active links, chips |
| text | `color-accent-ink` | #1B46C2 | `color-panel` | #EEF2F7 | 6.91 | 4.5:1 | PASS | hover/active links, chips |
| text | `color-accent-ink` | #1B46C2 | `color-accent-tint` | #E8EEFD | 6.69 | 4.5:1 | PASS | hover/active links, chips |
| text | `color-success-ink` | #0A7050 | `color-white` | #FFFFFF | 6.09 | 4.5:1 | PASS | success text, chips |
| text | `color-success-ink` | #0A7050 | `color-canvas` | #F7F9FC | 5.78 | 4.5:1 | PASS | success text, chips |
| text | `color-success-ink` | #0A7050 | `color-panel` | #EEF2F7 | 5.42 | 4.5:1 | PASS | success text, chips |
| text | `color-success-ink` | #0A7050 | `color-success-tint` | #E5F4EE | 5.37 | 4.5:1 | PASS | success text, chips |
| text | `color-warning-ink` | #8A5806 | `color-white` | #FFFFFF | 6.04 | 4.5:1 | PASS | warning text, chips |
| text | `color-warning-ink` | #8A5806 | `color-canvas` | #F7F9FC | 5.72 | 4.5:1 | PASS | warning text, chips |
| text | `color-warning-ink` | #8A5806 | `color-panel` | #EEF2F7 | 5.37 | 4.5:1 | PASS | warning text, chips |
| text | `color-warning-ink` | #8A5806 | `color-warning-tint` | #FBF0DC | 5.35 | 4.5:1 | PASS | warning text, chips |
| text | `color-danger-ink` | #A82A39 | `color-white` | #FFFFFF | 6.87 | 4.5:1 | PASS | danger text, chips |
| text | `color-danger-ink` | #A82A39 | `color-canvas` | #F7F9FC | 6.52 | 4.5:1 | PASS | danger text, chips |
| text | `color-danger-ink` | #A82A39 | `color-panel` | #EEF2F7 | 6.11 | 4.5:1 | PASS | danger text, chips |
| text | `color-danger-ink` | #A82A39 | `color-danger-tint` | #FBE8EA | 5.84 | 4.5:1 | PASS | danger text, chips |
| text | `color-danger` | #C33444 | `color-white` | #FFFFFF | 5.39 | 4.5:1 | PASS | danger as text only where listed (white, canvas, tint) |
| text | `color-danger` | #C33444 | `color-canvas` | #F7F9FC | 5.11 | 4.5:1 | PASS | danger as text only where listed (white, canvas, tint) |
| text | `color-danger` | #C33444 | `color-danger-tint` | #FBE8EA | 4.58 | 4.5:1 | PASS | danger as text only where listed (white, canvas, tint) |
| text | `color-ink` | #111827 | `color-accent-tint` | #E8EEFD | 15.27 | 4.5:1 | PASS | body text on tints |
| text | `color-ink` | #111827 | `color-success-tint` | #E5F4EE | 15.63 | 4.5:1 | PASS | body text on tints |
| text | `color-ink` | #111827 | `color-warning-tint` | #FBF0DC | 15.71 | 4.5:1 | PASS | body text on tints |
| text | `color-ink` | #111827 | `color-danger-tint` | #FBE8EA | 15.06 | 4.5:1 | PASS | body text on tints |
| text | `color-ink-2` | #475569 | `color-accent-tint` | #E8EEFD | 6.52 | 4.5:1 | PASS | secondary text on tints |
| text | `color-ink-2` | #475569 | `color-success-tint` | #E5F4EE | 6.68 | 4.5:1 | PASS | secondary text on tints |
| text | `color-ink-2` | #475569 | `color-warning-tint` | #FBF0DC | 6.71 | 4.5:1 | PASS | secondary text on tints |
| text | `color-ink-2` | #475569 | `color-danger-tint` | #FBE8EA | 6.43 | 4.5:1 | PASS | secondary text on tints |
| text | `color-white` | #FFFFFF | `color-accent` | #2457E7 | 5.85 | 4.5:1 | PASS | text on filled buttons |
| text | `color-white` | #FFFFFF | `color-accent-ink` | #1B46C2 | 7.77 | 4.5:1 | PASS | text on filled buttons |
| text | `color-white` | #FFFFFF | `color-danger` | #C33444 | 5.39 | 4.5:1 | PASS | text on filled buttons |
| text | `color-white` | #FFFFFF | `color-ink` | #111827 | 17.74 | 4.5:1 | PASS | text on filled buttons |
| text | `color-code-ink` | #E2E8F0 | `color-code-bg` | #0F172A | 14.48 | 4.5:1 | PASS | code block syntax |
| text | `color-code-mute` | #94A3B8 | `color-code-bg` | #0F172A | 6.96 | 4.5:1 | PASS | code block syntax |
| text | `color-code-key` | #9DB7FF | `color-code-bg` | #0F172A | 9.05 | 4.5:1 | PASS | code block syntax |
| text | `color-code-str` | #7FD6B0 | `color-code-bg` | #0F172A | 10.33 | 4.5:1 | PASS | code block syntax |
| text | `color-code-num` | #F2C57C | `color-code-bg` | #0F172A | 11.10 | 4.5:1 | PASS | code block syntax |
| text | `color-code-err` | #FF9AA5 | `color-code-bg` | #0F172A | 8.86 | 4.5:1 | PASS | code block syntax |
| non-text | `color-line-strong` | #7C8A9E | `color-white` | #FFFFFF | 3.51 | 3:1 | PASS | control boundary |
| non-text | `color-line-strong` | #7C8A9E | `color-canvas` | #F7F9FC | 3.33 | 3:1 | PASS | control boundary |
| non-text | `color-line-strong` | #7C8A9E | `color-panel` | #EEF2F7 | 3.12 | 3:1 | PASS | control boundary |
| non-text | `color-focus` | #2457E7 | `color-white` | #FFFFFF | 5.85 | 3:1 | PASS | focus ring |
| non-text | `color-focus` | #2457E7 | `color-canvas` | #F7F9FC | 5.54 | 3:1 | PASS | focus ring |
| non-text | `color-focus` | #2457E7 | `color-panel` | #EEF2F7 | 5.20 | 3:1 | PASS | focus ring |
| non-text | `color-focus-on-dark` | #9DB7FF | `color-code-bg` | #0F172A | 9.05 | 3:1 | PASS | focus ring on code block |
| non-text | `color-success` | #0C8A63 | `color-white` | #FFFFFF | 4.35 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-success` | #0C8A63 | `color-canvas` | #F7F9FC | 4.12 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-warning` | #A96B08 | `color-white` | #FFFFFF | 4.38 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-warning` | #A96B08 | `color-canvas` | #F7F9FC | 4.15 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-danger` | #C33444 | `color-white` | #FFFFFF | 5.39 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-danger` | #C33444 | `color-canvas` | #F7F9FC | 5.11 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-neutral` | #64748B | `color-white` | #FFFFFF | 4.76 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-neutral` | #64748B | `color-canvas` | #F7F9FC | 4.51 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-accent` | #2457E7 | `color-white` | #FFFFFF | 5.85 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-accent` | #2457E7 | `color-canvas` | #F7F9FC | 5.54 | 3:1 | PASS | status glyph / chart line |
| non-text | `color-success` | #0C8A63 | `color-success-tint` | #E5F4EE | 3.83 | 3:1 | PASS | glyph on its tint |
| non-text | `color-warning` | #A96B08 | `color-warning-tint` | #FBF0DC | 3.88 | 3:1 | PASS | glyph on its tint |
| non-text | `color-danger` | #C33444 | `color-danger-tint` | #FBE8EA | 4.58 | 3:1 | PASS | glyph on its tint |
| non-text | `color-neutral` | #64748B | `color-panel` | #EEF2F7 | 4.23 | 3:1 | PASS | glyph on panel |
| non-text | `color-accent` | #2457E7 | `color-accent-tint` | #E8EEFD | 5.03 | 3:1 | PASS | glyph on its tint |
| non-text | `color-ink-2` | #475569 | `color-white` | #FFFFFF | 7.58 | 3:1 | PASS | chart axis / tick |

### Measured and deliberately rejected for the stated use

| Foreground | Hex | Background | Hex | Ratio | Needs | Result | Decision |
|---|---|---|---|---|---|---|---|
| `color-success` | #0C8A63 | `color-white` | #FFFFFF | 4.35 | 4.5:1 | FAIL | starting token fails as text; use color-success-ink for text, base only for glyphs/bars |
| `color-warning` | #A96B08 | `color-white` | #FFFFFF | 4.38 | 4.5:1 | FAIL | starting token fails as text; use color-warning-ink for text, base only for glyphs/bars |
| `color-neutral` | #64748B | `color-panel` | #EEF2F7 | 4.23 | 4.5:1 | FAIL | fails as text on panel; neutral state text uses color-ink-2 |
| `color-ink-disabled` | #8A94A6 | `color-white` | #FFFFFF | 3.06 | 4.5:1 | FAIL | disabled controls only; WCAG 1.4.3 exempts inactive components, state also stated in text |
| `color-line` | #E2E8F0 | `color-white` | #FFFFFF | 1.23 | 3:1 | FAIL | decorative divider only; never the sole boundary of an input or button |

Pairs checked: 73 required (73 pass, 0 fail); 5 documented rejections.
<!-- CONTRAST:END -->

Reading the rejected pairs: they are not bugs, they are measured exclusions. `--color-success` and `--color-warning` are never set as text on white (4.35 and 4.38, below 4.5): a success or warning message uses the `-ink` token. `--color-neutral` is a glyph color only. `--color-line` is decorative.

Forced colors: status chips, chips, the live indicator and badges gain a one-pixel `CanvasText` border under `forced-colors: active`; focus rings use `Highlight`. Glyph silhouettes and text already carry meaning, so nothing depends on the replaced colors.

## 4. Typography

### 4.1 Fonts (OQ-13 recommendation, owner decision pending)

| Role | Family | License | Source | Weights and files |
|---|---|---|---|---|
| UI and body | **IBM Plex Sans** | SIL Open Font License 1.1 | Upstream repository `github.com/IBM/plex` (release assets); npm mirror `@fontsource/ibm-plex-sans` | 400, 500, 600; Latin subset, WOFF2 |
| Technical values, code, labels | **IBM Plex Mono** | SIL Open Font License 1.1 | `github.com/IBM/plex`; npm mirror `@fontsource/ibm-plex-mono` | 400, 500; Latin subset, WOFF2 |
| Public display titles only | **Newsreader** | SIL Open Font License 1.1 | `github.com/productiontype/Newsreader`; npm mirror `@fontsource-variable/newsreader` | Variable, roman and italic, Latin subset, WOFF2 |

Licence status: the three licences above are recorded from the upstream projects' published licence files as known to the author. **Not re-verified against the live sources in this session** (no network fetch was made): confirm each `OFL.txt` when the files are downloaded, and commit those files next to the fonts. OFL 1.1 permits bundling and self-hosting; it forbids selling the font by itself and requires the licence text to travel with it. No font file is committed in M2.

Self-hosting plan (applies at M3/M6, in `apps/web`, outside this lane's M2 paths):

1. Download the Latin WOFF2 files from the upstream releases or the pinned npm packages; commit them with each family's `OFL.txt` and a `FONTS.md` listing source, version and SHA-256.
2. Serve from the same origin (`/fonts/...`), `font-display: swap`, preload only IBM Plex Sans 400 and 600 (above-the-fold), lazy for mono and display where not used above the fold.
3. Fallback stacks are in `tokens.css` (`--font-sans`, `--font-mono`, `--font-display`). Metric-matched fallbacks (`size-adjust`, `ascent-override`) are **NOT MEASURED**; measure CLS at M7 and add overrides only if layout shift appears.
4. Prototypes in `docs/design/prototypes/` use only the system fallback stacks and no font files, so they load offline and render in the system serif/sans/mono on the reviewer's machine. Screenshots therefore show system faces, not Plex or Newsreader; the type scale, sizes and tracking are the real tokens.

If the owner rejects these families, the replacement must be OFL, Apache-2.0 or equivalent and self-hostable; the tokens, not the markup, change.

### 4.2 Fluid scale (public site and docs)

Fluid between a 320 px and a 1440 px viewport using `clamp()`. Body never drops below 16 px.

| Token | Min to max | Used for |
|---|---|---|
| `--text-step--1` | 13 to 14 px | Captions, table meta on public pages |
| `--text-step-0` | 16 to 17 px | Body, docs prose (line height 1.6, measure about 68 characters) |
| `--text-step-1` | 18 to 21 px | Lede, card titles |
| `--text-step-2` | 21 to 28 px | Docs `h2` |
| `--text-step-3` | 26 to 40 px | Section titles (display serif on public pages) |
| `--text-step-4` | 32 to 56 px | Page titles |
| `--text-step-5` | 40 to 80 px | Home hero only (line height 1.04, tracking -0.028 em) |

### 4.3 Console scale (fixed)

Dense tools need stable sizes: 12 px (mono micro-labels only), 13 px (table meta), 14 px (body and cells), 16 px (inputs, which also prevents iOS zoom-on-focus), then 18 px (section and card titles, 1.29 times body), 20 px (empty-state titles), and 28 px (page title, 2.0 times body); KPI values are 32 px. Type-hierarchy check (computed styles, `docs/design/evidence/type-hierarchy.txt`): page title to body is at least 1.66 times on every prototype at 390 and 1440 px (home hero 4.71 times at 1440), and section heading to body is at least 1.29 times. An earlier build had console section headings at 16 px (1.14 times body, weight only); that was real and is fixed. Tabular figures (`font-variant-numeric: tabular-nums`) on every numeric column and KPI.

## 5. Grid, spacing, shape, depth

| Aspect | Token and value |
|---|---|
| Columns | 12, gutter `clamp(16px, 0.6rem + 1.6vw, 32px)`, outer margin `clamp(16px, 0.4rem + 3vw, 48px)` |
| Max widths | Marketing 1200 px, console 1440 px, prose 680 px |
| Spacing | 4 px base: 4, 8, 12, 16, 24, 32, 48, 64, 96, 128 (`--space-1..10`); public section gap `clamp(48px, 2rem + 5vw, 112px)` |
| Breakpoints | 480, 640 (table to stacked cards below), 768, 1024 (console sidebar docks), 1440 |
| Radii | 2, 4, 6 (default), 10 (dialogs), 999 (chips only) |
| Borders | 1 px. Decorative `--color-line`; interactive `--color-line-strong` |
| Shadows | `--shadow-xs` (a 1 px under-edge on cards), `--shadow-sm`, `--shadow-overlay` (dialogs, drawers, toasts). Nothing else casts a shadow. |
| Icon stroke | 1.5 px at 16 and 20 px, round caps and joins, 16 px grid; sizes 16, 20, 24 |
| Table density | Comfortable rows 44 px (default), compact 32 px. Both above the 24 px WCAG 2.5.8 target floor. A density *preference* is not implemented in 1.0, so only comfortable ships; compact exists for the topology list and the command dialog. |
| Controls | 40 px high (32 px small); touch contexts keep at least 44 px hit area through padding |

## 6. Status encodings (never color alone)

Every status is **text plus a glyph with a distinct silhouette**; color only reinforces. The same glyph and wording appear in tables, chips, legends, the docs concepts page and the demo.

| State | Text | Glyph silhouette (`#g-...` in the sprite) | Color token (glyph) | Chip tint |
|---|---|---|---|---|
| scheduled | Scheduled | Dashed circle with clock hand | neutral | neutral |
| queued | Queued | Rounded square with stacked bars | neutral | neutral |
| running | Running | Circle, right half filled | accent | accent |
| retry_wait | Retry wait | Circular arrow | warning | warning |
| succeeded | Succeeded | Circle with check | success | success |
| failed | Failed | Square with cross | danger | danger |
| canceled | Canceled | Circle with slash | neutral | neutral |
| worker online | Online | Ring with center dot | success | success |
| worker stale | Stale | Triangle with exclamation, dashed node outline | warning | warning |
| worker draining | Draining | Down arrow into a tray | warning | warning |
| worker revoked | Credential revoked | Shield with cross | danger | danger |
| worker deregistered | Deregistered | Circle with dash | neutral | neutral |
| unschedulable (reason, not a state) | Reason text (for example "No capable worker") | Warning triangle in an outlined chip on queued rows | warning | warning |
| masked | Hidden for viewer role | Padlock, dashed outline | ink-2 | panel |
| expired by retention | Expired by retention | Clock | ink-2 | panel |

`blocked` and `skipped` (reserved for 1.1) have no glyph and never appear.

Live indicator (text plus glyph): Live (ring with dot), Reconnecting (circular arrow), Polling (circular arrow, interval in text), Stale (triangle), Offline (crossed signal), Paused (boxed pause), Demo data (filled dot, public demo only).

Severity words follow the same rule: a notice states its meaning in its first words ("Failed", "Stale", "Rejected"), and the icon is secondary.

## 7. Chart encoding

Charts are SVG with a mandatory text alternative (title, description, data table, summary of min, max and last).

| Series | Line | Marker | Direct label |
|---|---|---|---|
| Succeeded | Solid 2.25 px, `--chart-succeeded` | Circle | "succeeded" at a point |
| Failed | Dashed 6 3, `--chart-failed` | Square (white fill) | "failed" |
| Canceled | Dotted 1 3, `--chart-canceled` | Triangle | "canceled" |
| Single-series metrics (queue age p95, run duration p95, attempts started, lease expirations, jobs admitted) | Solid, `--chart-single` | Diamond | Metric name in the chart title |

- Color is never the only channel: dash pattern, marker shape and a direct label also separate series, and a legend repeats all three.
- All chart lines are at least 3:1 on white (measured above). Gridlines use `--color-line`; axes `--color-ink-2` (7.58:1).
- Gaps are drawn as gaps. A bucket with no data is never plotted as zero. Buckets after `as_of` are absent.
- Failure counts share the axis with successes; they are never rescaled to look larger. If failures are invisible at that scale, the table and a separate small chart carry them.
- No animated redraw; a data swap is instant. Stale or offline: opacity 0.45 and the "as of" time written under the chart.
- Tooltips are supplementary; every value is reachable through the table, so no point inspection depends on hover (NFR-A11Y-01).
- Seven-day maximum range, at most 2016 points (the API limit), shown through step presets so a request is never over-limit.

## 8. Motion tokens

Durations and easings are tokens (`--duration-*`, `--ease-*`); the full per-component matrix is `motion-and-assets.md`. `@media (prefers-reduced-motion: reduce)` sets every duration token to 0 ms in `tokens.css`, and components additionally drop translate and scale.

## 9. Tailwind 4 adoption note

Token names follow Tailwind 4 theme namespaces (`--color-*`, `--font-*`, `--text-*`, `--radius-*`, `--shadow-*`, `--ease-*`, `--breakpoint-*`, `--spacing`). Tailwind itself is not installed in M2, so the claim "drop-in `@theme`" is **not verified**: at M3 or M6, import the file and confirm that utilities such as `bg-canvas`, `text-ink-2` and `rounded-md` resolve; if Tailwind requires an `@theme` wrapper, generate it from this file rather than copying values. Breakpoints cannot be read by `@media` from custom properties, so tests and docs treat them as constants.
