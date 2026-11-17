# CLUSTERX — Traceability Matrix

Vocabulary (§0): `proposed → specified → approved → implemented → integrated → tested → verified → released`.
A status is recorded only with evidence. Unrun tests are **NOT RUN**.

Master revision: R2 · adopted 2026-11-17 (OD-01). Contract SHA: _none frozen yet (M1)_.

| ID | Status | Evidence | Next gate |
|---|---|---|---|
| FR-ENG-01..24 | specified | requirements.md | M1 contracts |
| FR-API-01..04 | specified | requirements.md | M1 OpenAPI draft |
| FR-SEC-01..11 | specified | requirements.md | M1 auth ADR |
| FR-RT-01 | specified | requirements.md | M1 realtime contract |
| FR-DEMO-01 | specified | requirements.md | M2 route contract |
| FR-WEB-01..09 | specified | requirements.md | M2 design |
| NFR-* | specified | requirements.md | per release column |
| AC-01..AC-30 | specified | MASTER_PROMPT §11 | tests: NOT RUN |
| MW-01 | in progress | docs/coordination/tasks/ | each task |
| MW-04 | partially verified | docs/product/session-log.md | owner `/status` for Ultracode |
| MW-05 / MW-08 | in effect | OWNER_DECISIONS.md | each gated action |

Rows are updated per milestone gate. M1 will split rows to per-ID once contracts land.
