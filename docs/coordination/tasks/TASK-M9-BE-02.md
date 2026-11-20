# TASK-M9-BE-02 — REVIEW-M9-RV-02 ops/backend fixes

- Owner: backend lane `claude-opus-5-5`, effort medium. Branch `m9fix2/backend` from main. Allowed: Go code, `infra/**`, `scripts/**`, `docs/operations/**`, `docs/security/**`, handoff.

| Item | Required outcome |
|---|---|
| RV9-02 | Production example runs retention (scheduled service or control-plane loop with the retention role); prove one pass deletes expired rows in a local run. |
| RV9-04 | Caddy rejects methods outside GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS (405) before upstream; probe TRACE → 405. |
| RV9-05 | Bound concurrent `/metrics` scrapes (e.g. 1 in flight, others 429/serve cached ≤ scrape interval). Test. |
| RV9-06 | gRPC and metrics listeners not reachable from the edge network (separate networks); probe. |
| RV9-07 | Caddy nonroot (cap NET_BIND_SERVICE or high ports) — or document why not. |
| RV9-08 | Mutation rate limit 0 rejected in production config (fail closed); test. |
| RV9-09 | Backup drill never copies password files into evidence; cleanup on failure (trap). |
| RV9-11 | Alert when node-exporter target is down (absent/up==0); promtool test. |

Acceptance: Go vet + race green; promtool check/test green; production example re-verified locally (`infra/production/verify-local-proof.sh`) exit 0 with scoped project name and `down -v`; handoff `HANDOFF-M9-BE-02.md`.
