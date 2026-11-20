# CLUSTERX — Owner decision register

| ID | Decision | Proposed default | Required by | Status |
|---|---|---|---|---|
| OD-01 | Adopt R2 master specification | Use as canonical docs/product/MASTER_PROMPT.md | M0 | **APPROVED 2026-11-17** (owner, chat) |
| OD-02 | License | Apache-2.0 | Publication | **APPROVED 2026-11-18** — Apache-2.0 (LICENSE added) |
| OD-03 | OAuth provider and first operator | GitHub OAuth allowlisted numeric IDs, no default password | Real authenticated deployment | PENDING OWNER |
| OD-04 | Hosting/provider, budget, domain | Local-only until approved | External deployment | PENDING OWNER |
| OD-05 | Model entitlement and fallback | Sonnet 5.5 high/Opus 5.5 medium, optional Ultracode | AI sessions | **PARTIAL** — models verified; effort/Ultracode UNVERIFIED (see session-log.md) |
| OD-06 | Numeric backend SLOs & RPO/RTO | Freeze after pilot measurements | Release candidate | REQUIRES EXPERIMENT |
| OD-07 | Exact dependency compatibility | Pin tested versions after clean build | M3 | REQUIRES EXPERIMENT |
| OD-08 | Local repo init + initial commit (no push) | git init in ~/Documents/Clusterx | M0 | **APPROVED 2026-11-17** |
| OD-09 | Local toolchain install | brew Go 1.27 + PostgreSQL 18 | M1 | **APPROVED 2026-11-17**; Docker: owner installs |
| OD-10 | Lane mode | Subagents per worktree under integrator session | M1 | **APPROVED 2026-11-17** |
| OD-11 | Push to GitHub `Sendiw14/Clusterx` | Not authorized | Any push | NOT AUTHORIZED |
| OD-12 | Accept M1 exit + freeze contract draft.4 (`c48b62f`) | Accept; reviewer PASS, 0 P0/P1 open | M2/M4 start | **APPROVED 2026-11-18** (owner, chat); tag `contract-m1-draft.4` |
| OD-13 | Viewer read access to `/audit` (OQ-03) | Yes, actor/detail masked | M5 | **APPROVED default 2026-11-18** |
| OD-14 | Session lifetimes | idle 2 h, absolute 12 h | M5 | **APPROVED default 2026-11-18** |
| OD-15 | Container runtime | Colima + docker CLI + compose via brew; Node 26 LTS via brew | M3 | **APPROVED 2026-11-18** |
| OD-16 | Build whole project M2–M9 locally | Milestone gates, no push/deploy/spend | — | **APPROVED 2026-11-18** (owner: "do the whole project") |
| OD-17 | Fonts (OQ-13) | IBM Plex Sans, IBM Plex Mono, Newsreader (all OFL 1.1, self-hosted); serif display on public titles only | M3-FE | **APPROVED default 2026-11-18** (integrator under OD-16; owner may override) — re-verify licence files on first download |
| OD-18 | M2 design defaults | `/login` noindex; topology default = List, graph on request; cluster threshold 12 workers (revisit after M6 budget); `/demo` tour scope set at M7 | M6/M7 | **APPROVED default 2026-11-18** (integrator under OD-16) |
| OD-19 | Vulnerability reporting channel (SECURITY.md) | GitHub private vulnerability reporting on Sendiw14/Clusterx | M9 | **APPROVED 2026-11-20** (owner) — owner enables it in repo settings at publication |
| OD-20 | GitHub publication at M9 | Stay local; owner pushes later or instructs | M9 | **APPROVED 2026-11-20** (owner): stay local — OD-11 still NOT AUTHORIZED |
