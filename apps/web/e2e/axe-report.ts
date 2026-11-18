import { appendFileSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
export const AXE_LOG = resolve(here, "../test-results/axe.ndjson");
const EVIDENCE = resolve(here, "../../../docs/design/evidence");

export interface AxeRecord { route: string; width: number; serious: number; critical: number; moderate: number; minor: number; violations: { id: string; impact: string | null | undefined; nodes: number; help: string }[] }

export function recordAxe(r: AxeRecord) {
  mkdirSync(dirname(AXE_LOG), { recursive: true });
  appendFileSync(AXE_LOG, JSON.stringify(r) + "\n");
}
export function resetAxeLog() {
  rmSync(AXE_LOG, { force: true });
}
/** Aggregates every worker's records into the evidence files after the run. */
export function writeAxeEvidence() {
  if (!existsSync(AXE_LOG)) return;
  const all = readFileSync(AXE_LOG, "utf8").trim().split("\n").filter(Boolean).map((l) => JSON.parse(l) as AxeRecord);
  const sorted = all.sort((a, b) => a.route.localeCompare(b.route) || a.width - b.width);
  const tot = (k: "critical" | "serious" | "moderate" | "minor") => sorted.reduce((n, r) => n + r[k], 0);
  const lines = [
    "# axe-core results, M3 web foundation",
    "",
    "Run by `pnpm --filter web e2e` (Playwright Chromium, `@axe-core/playwright`, tags wcag2a, wcag2aa, wcag21a, wcag21aa, wcag22aa).",
    "Automated checks cover only part of WCAG: no conformance is claimed (manual screen-reader and keyboard walkthroughs are NOT RUN).",
    "",
    `Scans: ${sorted.length} (24 routes x widths 320 and 1440, demo screens in both views, open mobile menu). Totals: critical ${tot("critical")}, serious ${tot("serious")}, moderate ${tot("moderate")}, minor ${tot("minor")}.`,
    "",
    "| Scan | Width | Critical | Serious | Moderate | Minor | Rules (nodes) |",
    "|---|---|---|---|---|---|---|",
    ...sorted.map((r) => `| \`${r.route}\` | ${r.width} | ${r.critical} | ${r.serious} | ${r.moderate} | ${r.minor} | ${r.violations.map((v) => `${v.id} (${v.nodes})`).join(", ") || "none"} |`),
    "",
  ];
  mkdirSync(EVIDENCE, { recursive: true });
  writeFileSync(resolve(EVIDENCE, "axe-m3.md"), lines.join("\n"));
  writeFileSync(resolve(EVIDENCE, "axe-m3.json"), JSON.stringify(sorted, null, 2) + "\n");
}
