# CLUSTERX reviewed multi-model setup — R2

These are templates, not changes to your GitHub repository. The master is in `docs/product/MASTER_PROMPT.md` and contains **all** product and model-ownership rules. The audit is supporting evidence only.

1. Read `docs/product/READINESS_AUDIT.md`, review owner-dependent decisions and adopt R2 before considering it approved.
2. Recheck `Sendiw14/Clusterx` HEAD and branch permissions. Do not overwrite existing files. Copy this kit after owner permission and an initial authorized commit.
3. Check Claude Code: `claude --version`; inside each session `/model`, `/effort`, `/status`, and ultracode/workflow state. Preferred Sonnet high, Opus medium. The `.claude/settings.json` requests Ultracode on where supported but **does not grant access or set per-session effort**.
4. Run frontend and backend in separate worktrees after an initial commit. Verify model/effort *in each session*; do not assume an agent file changes the currently running main session.
5. Complete M0/M1 contracts/ADRs before parallel implementation. Frontend may use labeled fixtures while backend builds; release requires actual integrated E2E.
6. Use a TASK and HANDOFF file for every assigned branch/PR; owner/integrator merges contracts first and generated client second. Never commit secrets.
7. Never treat a subagent review as a passing automated test or human release authorization.
