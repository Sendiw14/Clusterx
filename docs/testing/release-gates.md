# Release gates (MASTER §11, §14 M9, §15)

The owner decides GO/NO-GO; reviewer approval is not release authorization. State as of 2026-11-20, main after
AUDIT-M9-RV-03 fixes. Details per AC: `docs/product/traceability.md`.

| Gate | State | Evidence / blocker |
|---|---|---|
| Go vet + race, all modules (incl. chaos, e2e) | PASS (local) | integrator runs on main; AUDIT-M9-RV-03 |
| Contract lint, SDK + proto reproducible | PASS (local) | `pnpm contract:lint`, `pnpm sdk:check`, `generate.sh` |
| Web lint/typecheck/unit/build | PASS (local) | 136 unit tests |
| Live E2E — Chromium, Firefox, WebKit (mock OAuth) | PASS (local) | 213 passed per engine; one unexplained WebKit axe flake not reproduced in 78 reruns |
| Automated accessibility (axe, 3 engines) | PASS (local) | `docs/design/evidence/axe-*` |
| **Manual accessibility (screen reader, keyboard, zoom)** | **NOT RUN — needs a human** | F-07, AC-22 |
| Public performance (lab Lighthouse, static pages) | PASS (lab) | `lighthouse-m9.md`; field data unavailable |
| Console budgets + bundle size | MEASURED, not frozen | `docs/testing/evidence/web/*`; **OD-06** |
| Supply chain (govulncheck, pnpm audit, OSV, gitleaks, SBOM) | PASS (local) | `docs/testing/evidence/m9/supply-chain.md` |
| **GitHub CI on the exact commit** | **NOT RUN** | **OD-11/OD-20** (stay local) |
| Clean-checkout reproduction | PASS on Linux arm64 VM (audit); macOS run from a checkout under `$HOME` only | AC-26; amd64 NOT RUN |
| Backup/restore, upgrade, alert drills | PASS (local) | M8 evidence; AUDIT-M9-RV-03 R4 |
| Production example (TLS, isolation, retention, fail-closed) | PASS (local VM) | `infra/production/verify-local-proof.sh` |
| **Reference host (amd64, DNS/ACME, firewall)** | **blocked-by-owner** | **OD-04**, AC-27 |
| **Real GitHub OAuth + first operator** | **blocked-by-owner** | **OD-03** |
| **SLO / RPO / RTO / console budgets frozen** | **blocked-by-owner** | **OD-06** |
| Independent reviews | DONE for M1, M5, M6, M7/M8/M9 | reviews/*; latest fixes not re-audited |
| Public claims match code | fixed after RV-03 (not re-audited) | RV10-01/02/04/06 |

**Recommendation: NO-GO for 1.0** until the bold rows are closed. Remaining blockers are a human accessibility pass
and owner decisions/actions (OD-03, OD-04, OD-06, a CI run via OD-11).
