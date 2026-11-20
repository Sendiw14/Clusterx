#!/usr/bin/env bash
# Supply-chain checks (MASTER_PROMPT §7, AC-25), identical locally and in .github/workflows/security.yml.
#   scripts/security/scan.sh <out-dir> [govulncheck|pnpm|osv|gitleaks|sbom ...]   (default: all)
# Tools on PATH, pinned (install: scripts/security/install-tools.sh). Exit 1 if any check finds a problem;
# every report is written to <out-dir> whatever the outcome (never conceal a scan).
set -uo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
OUT=$(mkdir -p "$1" && cd "$1" && pwd); shift
checks=("$@"); [ ${#checks[@]} -gt 0 ] || checks=(govulncheck pnpm osv gitleaks sbom)
rc=0
for c in "${checks[@]}"; do
  case $c in
  govulncheck)
    # Symbol-level reachability for every Go module of the workspace.
    for m in apps/control-plane apps/worker packages/sdk-go packages/proto examples/basic-jobs; do
      f="$OUT/govulncheck-$(echo "$m" | tr / -).txt"
      (cd "$ROOT/$m" && GOWORK=off govulncheck -show verbose ./...) > "$f" 2>&1; e=$?
      echo "govulncheck $m exit=$e"; [ $e -eq 0 ] || rc=1
    done ;;
  pnpm)
    # npm advisories for the locked pnpm workspace (fails on any advisory of moderate or higher).
    (cd "$ROOT" && pnpm audit --audit-level moderate --json) > "$OUT/pnpm-audit.json" 2> "$OUT/pnpm-audit.stderr.txt"; e=$?
    echo "pnpm audit exit=$e"; [ $e -eq 0 ] || rc=1 ;;
  osv)
    # pnpm lockfile and Go modules against OSV (includes npm advisories).
    tree=$(mktemp -d); git -C "$ROOT" archive HEAD | tar -x -C "$tree"   # committed tree only
    osv-scanner scan source --recursive --format table "$tree" > "$OUT/osv-scanner.txt" 2>&1; e=$?; rm -rf "$tree"
    echo "osv-scanner exit=$e"; [ $e -eq 0 ] || rc=1 ;;
  gitleaks)
    # Whole git history plus the working tree; findings are redacted in the report.
    (cd "$ROOT" && gitleaks git --redact --no-banner --report-format json --report-path "$OUT/gitleaks-history.json" .) > "$OUT/gitleaks-history.txt" 2>&1; e=$?
    echo "gitleaks history exit=$e"; [ $e -eq 0 ] || rc=1
    # Scan the committed tree only (git archive), so gitignored local secrets (.env, .next, worktrees) don't count.
    tree=$(mktemp -d); git -C "$ROOT" archive HEAD | tar -x -C "$tree"; cp "$ROOT/.gitleaksignore" "$tree/" 2>/dev/null || true
    (cd "$tree" && gitleaks dir --redact --no-banner --report-format json --report-path "$OUT/gitleaks-tree.json" .) > "$OUT/gitleaks-tree.txt" 2>&1; e=$?; rm -rf "$tree"
    echo "gitleaks tree exit=$e"; [ $e -eq 0 ] || rc=1 ;;
  sbom)
    # SPDX SBOM of each locally built image named in CLUSTERX_SBOM_IMAGES (space separated) plus the source tree.
    tree=$(mktemp -d); git -C "$ROOT" archive HEAD | tar -x -C "$tree"
    syft scan "dir:$tree" -q -o spdx-json="$OUT/sbom-source.spdx.json"; e=$?; rm -rf "$tree"
    echo "syft source exit=$e"; [ $e -eq 0 ] || rc=1
    for img in ${CLUSTERX_SBOM_IMAGES:-}; do
      f="$OUT/sbom-$(echo "$img" | tr '/:' '--').spdx.json"
      syft scan "docker:$img" -q -o spdx-json="$f"; e=$?
      echo "syft $img exit=$e packages=$(python3 -c 'import json,sys;print(len(json.load(open(sys.argv[1]))["packages"]))' "$f" 2>/dev/null)"; [ $e -eq 0 ] || rc=1
    done ;;
  *) echo "unknown check $c" >&2; rc=2 ;;
  esac
done
exit $rc
