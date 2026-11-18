# TASK-M3-FE-01 — Web foundation: Next.js app, UI library, data providers, test harness

- Milestone: M3 foundation (web part) — prepares M6/M7. Requirement IDs: FR-WEB-04, FR-WEB-05, FR-WEB-07, FR-DEMO-01 (provider isolation), FR-WEB-09 (BFF skeleton), NFR-A11Y-01, NFR-DEP-01; AC-14 (isolation harness), AC-21/22 (harness).
- Owner: frontend lane `claude-sonnet-5-5`, effort high (requested). Branch `m3/frontend`, worktree `.worktrees/frontend`.
- Contract base: `contract-m1-draft.4`; generated client `packages/sdk-ts` (integrator-owned — import only, never edit). M4 is producing draft.5 (additive); integrator will regenerate.
- Allowed: `apps/web/**`, `packages/ui/**`, `docs/design/**`, `docs/coordination/handoffs/HANDOFF-M3-FE-01.md`.
- Forbidden: root config (`package.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml` — **exception: lockfile changes produced by `pnpm install` for your two packages are allowed; list them in the handoff**), `.github/**`, `api/**`, `packages/sdk-ts/**`, Go code, `docs/product/**`, `docs/architecture/**`.

## Deliverables

1. `apps/web`: Next.js 16 (Active LTS, App Router), React 19.x, TypeScript per dependency matrix (5.9.x unless Next requires otherwise — record), Tailwind CSS 4 with `packages/ui/tokens/tokens.css` as the `@theme` source (verify the fit flagged NOT RUN in M2). ESLint (flat config, current CLI — not `next lint`), strict TS, `next build` with Turbopack (fall back only with an ADR note). Self-hosted fonts per OD-17 (download upstream OFL files, include licence texts, verify licences).
2. Route shells for every 1.0 route in `docs/design/route-inventory.md` — public routes render real layout from M2 designs with honest placeholder copy where docs content is M7; console routes render layout + skeleton/empty/error states. No fake controls: anything not wired yet is absent or disabled with a visible reason. 1.1 routes absent (404).
3. `packages/ui`: accessible components from `component-specs.md` needed by shells (shell/nav/footer, sidebar/topbar/breadcrumbs, buttons, inputs, select, dialog, toast, status badge with icon+text, table + pagination, skeleton, empty/error/offline/stale states, code block + copy, tabs). Keyboard + ARIA per spec; reduced-motion variants; Motion 14.x only where the motion matrix requires.
4. Data-provider seam: one interface used by pages; `LiveProvider` (BFF → generated `@clusterx/sdk-ts`) and `FixtureProvider` (demo only). `/demo` imports ONLY the fixture provider — enforce with an ESLint `no-restricted-imports` boundary and a Playwright network trap that fails on any request leaving the origin or hitting `/api/`/`/bff/`.
5. BFF skeleton (`app/bff/**` route handlers + server-only session module) with interfaces per ADR-0006 but **no OAuth/session implementation yet** (M5): returns typed 501 `not_implemented` envelope. Security headers + CSP baseline in Next config.
6. Test harness: Vitest for components; Playwright (Chromium) E2E smoke for every route shell at 320 and 1440 (no horizontal overflow, landmarks present, no console errors); `@axe-core/playwright` on every route (0 serious/critical violations); demo isolation test.
7. Package scripts in `apps/web/package.json`: `lint`, `typecheck`, `test`, `build`, `e2e`. Integrator wires them into root/CI.

## Acceptance

`pnpm --filter web lint && typecheck && test && build && e2e` all exit 0 locally; axe results saved under `docs/design/evidence/`; screenshots of `/`, `/app/jobs`, `/demo` at 390/1440 matching M2 intent; versions listed for dependency matrix.
