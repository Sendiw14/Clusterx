# HANDOFF — TASK-M9-BE-01 (audit fixes: F-12, RV6B-04 runbook note)

- **Author:** backend lane subagent, model `claude-opus-5-5`, requested effort medium. Effective effort and Ultracode:
  UNVERIFIED from inside the subagent (MW-04).
- **Branch:** `m9fix/backend` (worktree `.worktrees/backend`), base main `6222d0c`. Not pushed, not merged.
- **Source findings:** AUDIT-M9-RV-01 F-12 and §4 ruling 1; REVIEW-M6-RV-01 round 2 RV6B-04 and RV6-10.
  Acceptance touched: AC-17/AC-18 (alerting), AC-28 (operations docs).
- **Contract changes:** none. No Go code, SQL, OpenAPI or proto change. The optional F-12 refinement
  (`deadline_exceeded` only when `hard_deadline_at < lease_expires_at` at last renewal) was **not** done: the ruling
  accepts the current precedence.
- **Review owner:** independent reviewer (RV). No integrator-review files touched.

## Commits

| SHA | Content |
|---|---|
| `1754b88` | F-12: state-machine E8 row + `failure_class` note; ADR-0003 Recovery precedence paragraph; `ClusterxLeaseExpiriesRepeated` expr → `sum(clusterx_attempts_finished_5m{state=~"lease_expired\|deadline_exceeded"}) >= 3`, summary updated; promtool tests; incident-runbook triage (crash vs overrun) |
| `0d164e3` | RV6B-04 / RV6-10: `docs/operations/upgrades.md` "Web (Next.js) dependency upgrades" — `didWebSocketSetup` guard check, absolute-form 308, TRACE 500, malformed %-encoding 500, edge method/origin-form rule, raw probe |
| (this) | handoff |

## Changed files

- `docs/architecture/state-machine.md`
- `docs/architecture/adrs/0003-leases-heartbeat-fencing-timing.md`
- `infra/monitoring/alerts.yml`
- `infra/monitoring/alerts_test.yml`
- `docs/operations/incident-runbook.md`
- `docs/operations/upgrades.md`
- `docs/coordination/handoffs/HANDOFF-M9-BE-01.md`

## Commands and exit codes (all executed)

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `promtool check rules alerts.yml` (pinned `quay.io/prometheus/prometheus:v3.16.0@sha256:93320545…` via docker/Colima) | 0 | `SUCCESS: 9 rules found` |
| 2 | `promtool test rules alerts_test.yml` (same image) | 0 | `SUCCESS` |
| 3 | Mutation: old expr `state="lease_expired"` against the new tests (temp copy, deleted) | 1 | `FAILED: alertname: ClusterxLeaseExpiriesRepeated, time: 2m` — tests detect the regression |
| 4 | `GOWORK=off go vet ./... && go test -race -count=1 ./...` in all 5 modules, `CLUSTERX_PG_ADMIN_DSN` → PG 18.7 on `127.0.0.1:55432` (own cluster, `-k ''`, stopped after) | 0 ×5 | all `ok`; chaos 27 s, e2e 17 s, store 10 s, poc 11 s; 0 skips |

Alert tests now cover: 2 `lease_expired` + 1 `deadline_exceeded` fires with value 3 (labels `severity` only, since
`sum` drops `state`); 2 `lease_expired` + 9 `failed` does not fire; 3 `deadline_exceeded` alone fires.
`ClusterxFailureRateHigh` expectation unchanged (56.52 %): the series total is the same.

## Limitations / integration needs

1. **Web `/docs/reliability`** (web lane): the audit's F-12 fix also asks for the precedence sentence there: "if both
   have passed, `deadline_exceeded` wins; with timeout ≤ lease TTL a crash is classified as a deadline". Not in this
   lane's paths.
2. **Edge filter not implemented:** `infra/production/Caddyfile` (outside allowed paths) forwards every method and was
   not changed. `upgrades.md` states the rule (origin-form only; GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS) and says the
   shipped Caddyfile does not filter yet. How Caddy handles an absolute-form target was NOT probed.
3. The alert now also fires on genuine handler overruns (`deadline_exceeded` with live workers); the runbook gives the
   triage. Alert name kept to keep the runbook anchor and dashboards stable.
4. Go test output was not committed as evidence (`docs/testing/**` not in allowed paths); results are in this table.
