# CLUSTERX — Owner decision register

| ID | Decision | Proposed default | Required by | Status |
|---|---|---|---|---|
| OD-01 | Adopt R2 master specification | Use as canonical docs/product/MASTER_PROMPT.md | M0 | **APPROVED 2026-11-17** (owner, chat) |
| OD-02 | License | MIT or Apache-2.0 (owner selects) | Publication | PENDING OWNER |
| OD-03 | OAuth provider and first operator | GitHub OAuth allowlisted numeric IDs, no default password | Real authenticated deployment | PENDING OWNER |
| OD-04 | Hosting/provider, budget, domain | Local-only until approved | External deployment | PENDING OWNER |
| OD-05 | Model entitlement and fallback | Sonnet 5.5 high/Opus 5.5 medium, optional Ultracode | AI sessions | **PARTIAL** — models verified; effort/Ultracode UNVERIFIED (see session-log.md) |
| OD-06 | Numeric backend SLOs & RPO/RTO | Freeze after pilot measurements | Release candidate | REQUIRES EXPERIMENT |
| OD-07 | Exact dependency compatibility | Pin tested versions after clean build | M3 | REQUIRES EXPERIMENT |
| OD-08 | Local repo init + initial commit (no push) | git init in ~/Documents/Clusterx | M0 | **APPROVED 2026-11-17** |
| OD-09 | Local toolchain install | brew Go 1.27 + PostgreSQL 18 | M1 | **APPROVED 2026-11-17**; Docker: owner installs |
| OD-10 | Lane mode | Subagents per worktree under integrator session | M1 | **APPROVED 2026-11-17** |
| OD-11 | Push to GitHub `Sendiw14/Clusterx` | Not authorized | Any push | NOT AUTHORIZED |
