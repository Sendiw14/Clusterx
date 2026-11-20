import { appendFileSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
export const AXE_LOG = resolve(here, "../test-results/axe.ndjson");
const EVIDENCE = resolve(here, "../../../docs/design/evidence");
const EVIDENCE_OTHER = resolve(here, "../../../docs/testing/evidence/web");

export interface AxeRecord { project?: string; route: string; width: number; serious: number; critical: number; moderate: number; minor: number; violations: { id: string; impact: string | null | undefined; nodes: number; help: string }[] }

export function recordAxe(r: AxeRecord) {
  mkdirSync(dirname(AXE_LOG), { recursive: true });
  appendFileSync(AXE_LOG, JSON.stringify(r) + "\n");
}
export function resetAxeLog() {
  rmSync(AXE_LOG, { force: true });
}
/** Aggregates every worker's records into the evidence files after the run. Chromium keeps the M6 files; other browsers write `axe-<project>.json` next to the web test evidence. */
export function writeAxeEvidence() {
  if (!existsSync(AXE_LOG)) return;
  const every = readFileSync(AXE_LOG, "utf8").trim().split("\n").filter(Boolean).map((l) => JSON.parse(l) as AxeRecord);
  for (const project of new Set(every.map((r) => r.project ?? "chromium"))) writeProject(project, every.filter((r) => (r.project ?? "chromium") === project));
}

function writeProject(project: string, all: AxeRecord[]) {
  const sorted = all.sort((a, b) => a.route.localeCompare(b.route) || a.width - b.width);
  const tot = (k: "critical" | "serious" | "moderate" | "minor") => sorted.reduce((n, r) => n + r[k], 0);
  const chromium = project === "chromium";
  const lines = [
    "# axe-core results, M6 (live console on real data) and the public site, docs and demo" + (chromium ? "" : ` (${project})`),
    "",
    `Run by \`pnpm --filter web e2e\` (Playwright ${chromium ? "Chromium" : project}, \`@axe-core/playwright\`, tags wcag2a, wcag2aa, wcag21a, wcag21aa, wcag22aa).`,
    "Automated checks cover only part of WCAG: no conformance is claimed (manual screen-reader and keyboard walkthroughs are NOT RUN).",
    "",
    `Scans: ${sorted.length} (public routes and demo as in M7; every console route on live data as operator at 1440 and 320 and as viewer at 1440 and 390, job detail succeeded and failed, worker detail, the forbidden state, and the retry, create-token, one-time-reveal and jump-to dialogs open). Totals: critical ${tot("critical")}, serious ${tot("serious")}, moderate ${tot("moderate")}, minor ${tot("minor")}.`,
    "",
    "| Scan | Width | Critical | Serious | Moderate | Minor | Rules (nodes) |",
    "|---|---|---|---|---|---|---|",
    ...sorted.map((r) => `| \`${r.route}\` | ${r.width} | ${r.critical} | ${r.serious} | ${r.moderate} | ${r.minor} | ${r.violations.map((v) => `${v.id} (${v.nodes})`).join(", ") || "none"} |`),
    "",
  ];
  const dir = chromium ? EVIDENCE : EVIDENCE_OTHER;
  const base = chromium ? "axe-m6" : `axe-${project}`;
  mkdirSync(dir, { recursive: true });
  writeFileSync(resolve(dir, `${base}.md`), lines.join("\n"));
  writeFileSync(resolve(dir, `${base}.json`), JSON.stringify(sorted.map((r) => ({ ...r, project: undefined })), null, 2) + "\n");
}
