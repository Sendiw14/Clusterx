# TASK-M7-FE-01 — Public site, documentation, isolated demo (craft pass)

- Milestone: M7 public craft (run before M6 because M6 needs M5 sessions). Requirement IDs: FR-WEB-01, FR-WEB-05..07, FR-DEMO-01, NFR-A11Y-01, NFR-RESP-01, NFR-PERF-01; AC-14, AC-21, AC-22, AC-23, AC-24 (lab), AC-28 (docs truthfulness), AC-29.
- Owner: frontend lane `claude-sonnet-5-5`, effort high (requested). Branch `m7/frontend`, worktree `.worktrees/frontend`. Base: main `ad54425`.
- Allowed: `apps/web/**` (public + docs + demo routes, shared components), `packages/ui/**`, `docs/design/**`, `docs/coordination/handoffs/HANDOFF-M7-FE-01.md`. Lockfile changes from `pnpm install` for these packages only.
- Forbidden: Go code, `api/**`, `packages/sdk-ts/**`, root config, `.github/**`, `infra/**`, `docs/product/**`, `docs/architecture/**` (read only), console `/app/**` pages beyond shared components (M6), `/login` and `app/bff/**` (M5/M6).

## Truth sources (docs must match shipped code — AC-28)

- Engine behaviour: `docs/architecture/state-machine.md`, ADRs 0001–0006, `docs/architecture/api-contracts.md`, `api/openapi.yaml` (draft.5; draft.6 lands with M5 — write the API reference generated from the spec at build time, not hand-copied).
- Quickstart commands: `infra/compose/bootstrap.sh`, `.env.example`, `docs/coordination/handoffs/HANDOFF-M4-BE-01.md` (executed curl guide). **Only document commands the integrator has executed** — they are listed in that handoff and in this ticket below; anything else is marked "not yet verified".
- Integrator-verified 2026-11-18 on Colima: `cp .env.example .env` (fill 3 values with `openssl rand -hex 24`), `./infra/compose/bootstrap.sh` → exit 0, 3 live workers; `POST /api/v1/jobs` with `hash_sha256` and `fail_n_times` → all succeeded; operator token in `infra/compose/.secrets.env`.
- Worker SDK: `packages/sdk-go/*.go`, `examples/basic-jobs/basicjobs.go`.
- Limitations: MASTER §16 "Known project limitations" — must appear verbatim in substance on `/docs/reliability` and the home FAQ.

## Deliverables

1. `/` final: editorial hero, interactive illustrative Execution Atlas (labeled), capability pillars, architecture steps, worker-failure story, console preview (labeled as preview until M6 screenshots replace it), quickstart, limitations/FAQ, CTAs, footer. Faithful to M2 prototypes; elevate craft.
2. `/architecture`: layered topology diagram (accessible SVG + text alternative), job/lease path, trade-offs, failure domains, limitations, source links (repo-relative).
3. Docs: `/docs` index with client-side search over a build-time index (no external service), sidebar + on-this-page; `/docs/quickstart`, `/concepts`, `/api` (generated from `api/openapi.yaml` at build: operations, auth, errors, examples, request IDs, version policy), `/workers`, `/reliability`, `/deployment` (local compose verified; production section clearly "M8 — not yet verified"), `/security`. Copy buttons on code blocks.
4. `/changelog`: real history only — "Unreleased (pre-1.0)" with milestones actually merged; no fictitious versions.
5. `/demo`: complete fixture-based read-only console tour (watermarked, "illustrative — not live telemetry"), scope decided per OD-18: overview, jobs list, job detail (incl. failed + retry lineage), workers, worker-failure recovery replay. Fixture provider only; isolation tests extended.
6. Motion per motion matrix; reduced-motion verified; no scroll hijack.
7. Tests: E2E for every public/docs/demo route and interaction (search, copy, Atlas step/play/reset via keyboard, mobile menu), axe 0 serious/critical, 320/390/768/1024/1440/1920 overflow checks, link checker (all internal links resolve), demo network trap, Lighthouse/lab Web Vitals for `/`, `/docs`, `/demo` (record LCP/CLS/TBT lab numbers; field = unavailable).

## Acceptance

`pnpm --filter web lint typecheck test build e2e` all exit 0 on the branch; screenshots at 390/1440 of every public page in `docs/design/screenshots/m7/`; handoff lists every docs claim that is "not yet verified".
