# CLUSTERX — Shared Agent Operating Instructions

**Canonical contract:** `docs/product/MASTER_PROMPT.md` (Revision R2). This is the only source of product scope, architecture constraints, acceptance and model ownership. Read Section 1 of that file before editing. Do not re-interpret requirements from old prompts, chat summaries, or task tickets. Approved ADRs/contracts refine the master; report conflicts.

**Owners:** Sonnet 5.5 high owns web/design; Opus 5.5 medium owns Go/SQL/API/security/architecture; Opus 5.5 high independent read-only reviewer handles critical races/security. Confirm model+effort locally; Ultracode requires compatible Claude Code/workflow access. Default to two separate worktrees, NOT a shared editing workspace.

**Do not:** edit another lane's owned files; invent API endpoints; hand-edit generated SDKs; claim mock demo data is live; bypass CI or authorization; expose credentials; commit/push/publish/deploy/incur cost without user authorization; claim a test ran if it didn't.

**Every ticket must:** cite requirement and acceptance IDs, source-contract commit, allowed paths, dependencies, tests, review owner. At handoff report actual commits, changed files, commands/exits, evidence, errors, limitations, and integration needs. Critical architecture/security conflicts block dependent work until resolved.
