# TASK-M9-BE-01 — Audit fixes (backend/ops)

- Owner: backend lane `claude-opus-5-5`, effort medium. Branch `m9fix/backend` from main. Allowed: Go code, `infra/monitoring/**`, `docs/architecture/**`, `docs/operations/**`, handoff.

| Item | Required outcome |
|---|---|
| F-12 | Document in state-machine/ADR-0003 that when attempt timeout == lease TTL a crashed attempt is classified `deadline_exceeded`; make `ClusterxLeaseExpiriesRepeated` (or a sibling alert) count crash-type outcomes (`lease_expired` + `deadline_exceeded`); update rule tests and runbook. |
| RV6B-04 note | Add Next.js edge behaviours (absolute-form redirect, TRACE 500, malformed %-encoding 500) and the `didWebSocketSetup` guard to the upgrade runbook. |

Acceptance: Go vet + race green; `promtool test rules` green; handoff `HANDOFF-M9-BE-01.md`.
