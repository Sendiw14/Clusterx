# TASK-M2-FE-01 — M2 design system, route designs, Execution Atlas, motion

- Milestone: M2 design. Requirement IDs: FR-WEB-01..08, FR-DEMO-01, NFR-A11Y-01, NFR-RESP-01; AC-21..AC-23 (design inputs); RV01-10 (deferred to M2 — see REVIEW-M1-RV-01).
- Owner: frontend lane, `claude-sonnet-5-5`, effort high (requested). Branch `m2/frontend`, worktree `.worktrees/frontend`.
- Contract base: tag `contract-m1-draft.4` (frozen). Route inventory + consumer requirements on main.
- Allowed files: `docs/design/**`, `packages/ui/tokens/**` (token source only, no components yet), `docs/coordination/handoffs/HANDOFF-M2-FE-01.md`.
- Forbidden: `apps/**`, `api/**`, `packages/proto/**`, `packages/sdk-*/**`, root config, `docs/product/**`, `docs/architecture/**`.

## Deliverables (§9 "Design artifacts before major frontend work")

1. `docs/design/brand-and-tokens.md` + `packages/ui/tokens/tokens.css` (CSS custom properties, Tailwind 4 `@theme`-compatible): color tokens from §9 with **measured** contrast ratios (compute them; table of pairs with ratio and AA pass/fail), fluid type scale, font choice with license (OQ-13 — must be a free OFL/Apache font, e.g. self-hosted; record license), grid, spacing, radii, borders, shadows, icon stroke, densities, chart encoding, status encodings (never color-only).
2. `docs/design/information-architecture.md`: nav model public/docs/console, mobile menu, breadcrumbs, command search scope.
3. `docs/design/component-specs.md`: every component in §8 shared inventory — anatomy, states (default/hover/focus/active/disabled/loading/error), keyboard, ARIA, responsive, motion ref.
4. `docs/design/states-matrix.md`: route × state × permission (from route-inventory S-* catalogue).
5. `docs/design/execution-atlas.md`: original node/edge grammar for job/queue/attempt/lease/worker/recovery/result; statuses, labels, time context; clustering at 1/10/100 workers; zoom/pan; synchronized table/list equivalent; illustrative vs live marking.
6. `docs/design/motion-and-assets.md`: full motion matrix per component (trigger, from/to, delay, duration, easing, interruption, reversal, repeat-mount, scroll, visibility, reduced-motion fallback); icon/vector asset inventory with licenses.
7. `docs/design/accessibility-plan.md`: WCAG 2.2 AA criterion checklist plan, SR walkthrough scripts, 320 px reflow plan, test widths.
8. **High-fidelity prototypes** — self-contained static HTML (no external network, no CDN, inline CSS using tokens.css copy) in `docs/design/prototypes/`: `home.html` (hero + Atlas), `overview.html` (/app), `jobs.html` (/app/jobs incl. empty/error/stale), `job-detail.html` (attempts, lease, cancel/retry, 410 expired), `workers.html` (topology + list toggle, no-worker state), `login.html`. Each responsive at 320/390/768/1440, reduced-motion honored, labeled "design prototype — illustrative data".
9. Render each prototype in a browser at 390 and 1440 and save screenshots to `docs/design/screenshots/` (use whatever headless tool is available — e.g. `npx playwright` — record command; if none available, NOT RUN).

## Acceptance

Every §8 route maps to an IA entry and a states-matrix row; contrast table shows all text pairs ≥4.5:1 (or documented large-text 3:1); no color-only status; prototypes load offline; originality — no copied third-party graphics/wording.
Reviewer: integrator (data semantics) + independent visual/a11y check later.
