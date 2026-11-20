# Release gates (MASTER §11, §14 M9, §15)

Record of the 1.0 gate. Owner decides GO/NO-GO; reviewer approval is not release authorization.

| Gate | State (2026-11-20) | Evidence / blocker |
|---|---|---|
| Go vet + race, all modules | PASS (local) | AUDIT-M9 G2/G3 |
| Chaos / DB outage / kill -9 | PASS (local) | AC-04, AC-08 |
| Contract lint, SDK + proto reproducible | PASS (local) | AUDIT-M9 G4–G6 |
| Web lint/typecheck/unit/build | PASS (local) | AUDIT-M9 G7 |
| Live E2E (Chromium, mock OAuth) | PASS (local) | 211 passed |
| Cross-browser (Firefox, WebKit, Edge) | NOT RUN | F-08 |
| Manual accessibility (SR, keyboard, zoom) | NOT RUN | F-07 — needs a human tester |
| Performance (public Web Vitals, console budgets) | FAIL / NOT RUN | F-05, F-09; OD-06 |
| Supply chain (govulncheck, pnpm audit, gitleaks) | PASS (local) | post-audit F-04 fix |
| OSV scan, SBOM | NOT RUN locally | CI job defined |
| GitHub CI on exact commit | NOT RUN | OD-11/OD-20 (stay local) |
| Clean-checkout reproduction | PASS (Linux arm64 VM, macOS under `$HOME`) | AC-26; amd64 NOT RUN |
| Reference host: TLS, auth bootstrap, backup/restore | blocked-by-owner | OD-04, AC-27 |
| Real GitHub OAuth + first operator | blocked-by-owner | OD-03 |
| SLO / RPO / RTO / console budgets frozen | blocked-by-owner | OD-06 |
| Independent review of M7, M8, M9 web fixes | NOT RUN | F-19 |
| Public claims match code | FAIL until F-05/F-06 fixed | AC-28 |

**Current recommendation: NO-GO for 1.0** (AUDIT-M9-RV-01). No P0; engine correctness and security pass locally.
