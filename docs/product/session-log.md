# CLUSTERX — Session Configuration Log (MW-04)

## 2026-11-17 — integrator session

| Check | Result | Evidence |
|---|---|---|
| `claude --version` | 2.1.319 (≥ 2.1.284 required for Ultracode with non-xhigh effort) | shell output |
| Integrator/lead model | `claude-opus-5-5` | session system context |
| Integrator effort | Set by host session; not observable via tool — **UNVERIFIED** (owner: `/effort`) | — |
| Sonnet lane entitlement | `claude-sonnet-5-5` confirmed — probe subagent, `model=sonnet`, `effort=high` | probe reply `MODEL=claude-sonnet-5-5` |
| Opus lane entitlement | `claude-opus-5-5` confirmed — probe subagent, `model=opus`, `effort=medium` | probe reply `MODEL=claude-opus-5-5` |
| Per-subagent effort | Requested via Agent `effort` param (high / medium / high-reviewer). Applied by harness; subagent cannot self-report effort — **REQUESTED, not independently observed** | Agent call params |
| Ultracode | Project `.claude/settings.json` requests `"ultracode": true`. Activation **UNVERIFIED** from this session (owner: `/status`) | — |
| Multi-agent delegation | Available (Agent tool, background subagents, worktree isolation) | probes completed |
| Lane execution mode | Owner chose: subagents in this session, one git worktree per lane; Opus-high read-only reviewer | owner answer 2026-11-17 |

### Fallback rules in effect
- No model relabeling. If a subagent call returns a different model, stop that lane and report.
- Effort/Ultracode stay marked UNVERIFIED until owner records `/effort` and `/status` output here.

## Toolchain (2026-11-17)

| Tool | Found | Required | Action |
|---|---|---|---|
| git | 2.54.0 | any | ok |
| Go | missing | 1.27.x | owner-approved `brew install go` |
| PostgreSQL | 14.20 | 18.7 | owner-approved `brew install postgresql@18` |
| Node | 24.11.1 | 26.x LTS | upgrade before M3 (not needed M0/M1) |
| pnpm | 9.15.9 | lockfile-pinned | review at M3 |
| protoc | 29.3 | reproducible codegen (buf preferred) | M3 |
| Docker | missing | Compose | **owner installs before M3** |
| gh | missing | optional | owner choice |
