# TASK-M1-FE-01 — Consumer requirements and route inventory (frontend M1 input)

- Scope/release/milestone: M1 (consumer side of contract-first protocol §1.3 step 2); seeds M2
- Requirement IDs: FR-WEB-01..09, FR-RT-01, FR-DEMO-01, NFR-A11Y-01, NFR-RESP-01; AC-14, AC-15, AC-16, AC-21; MW-01
- Owner/model/effort: frontend lane — `claude-sonnet-5-5`, effort high (requested via Agent param)
- Branch/worktree: `m1/frontend` at `.worktrees/frontend`
- Contract base: master R2 @ M0 commit
- Dependencies: none for this ticket. Follow-up TASK-M1-FE-02 signs off backend drafts.
- Allowed files: `docs/design/route-inventory.md`, `docs/design/consumer-requirements.md`, `docs/coordination/handoffs/HANDOFF-M1-FE-01.md`
- Forbidden: everything else, including `api/**`, `apps/**`, `packages/**`, `docs/architecture/**`, `docs/product/**`
- Deliverables:
  1. `route-inventory.md`: every 1.0 public + console route from §8 with the §8 contract fields (purpose, persona, sections, data source, actions, RBAC/field scope, responsive, interaction/motion, loading/empty/error/success/offline, a11y, E2E IDs). 1.1 routes listed as gated/absent.
  2. `consumer-requirements.md`: what the UI needs from the API per route — fields, filters, sort keys, pagination, versions for reconciliation, error codes it must render distinctly (401/403/404/409/413/422/429/503, session expiry, unschedulable reason, expired data), viewer-masked fields, realtime hint shape. No invented endpoints: list needs, not designs.
- Acceptance: every §8 route present; every state category filled; demo route's data source is fixtures with no control-plane calls.
- Reviewer: integrator (Opus) validates data semantics.
