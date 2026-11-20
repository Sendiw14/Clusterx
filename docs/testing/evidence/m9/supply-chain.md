# Supply-chain scan — 2026-11-20T15:32:01Z, commit e0f36ea, macOS arm64 dev host

Command: CLUSTERX_SBOM_IMAGES="clusterx/control-plane:local clusterx/worker:local" ./scripts/security/scan.sh <out>
Tools: govulncheck v1.9.0, osv-scanner 2.7.1, syft 1.57.2, gitleaks 8.30.1, pnpm 9.15.9

```
govulncheck x5 exit=0; pnpm audit exit=0; osv-scanner exit=0 (No issues found; 555 npm + Go modules);
gitleaks history exit=0, tree exit=0; syft source exit=0 (617 packages); control-plane image 31 packages; worker image 18 packages
```

Raw SBOMs not committed (regenerable). CI run of security.yml: NOT RUN (OD-11/OD-20).

**Update 2026-11-20 (RV11-03):** images rebuilt from main before SBOM generation; both image SBOMs show `golang.org/x/net v0.62.0` (the release bump). control-plane 31 packages, worker 18 packages.
