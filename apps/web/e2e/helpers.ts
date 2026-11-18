import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";
import { recordAxe, type AxeRecord } from "./axe-report";

export const PUBLIC_ROUTES = ["/", "/architecture", "/docs", "/docs/quickstart", "/docs/concepts", "/docs/api", "/docs/workers", "/docs/reliability", "/docs/deployment", "/docs/security", "/changelog", "/login", "/demo"];
export const CONSOLE_ROUTES = ["/app", "/app/jobs", "/app/jobs/new", "/app/jobs/demo-job-0001", "/app/workers", "/app/workers/demo-worker-1", "/app/observability", "/app/activity", "/app/settings", "/app/settings/tokens", "/app/settings/preferences"];
export const ALL_ROUTES = [...PUBLIC_ROUTES, ...CONSOLE_ROUTES];
export const GATED_ROUTES = ["/app/workflows", "/app/workflows/x", "/app/workflow-runs/x"];


/**
 * Collects console errors and page errors. The BFF skeleton answers 501 on purpose (M5 implements it), and Chromium
 * logs every non-2xx subresource as a console error. Only that exact case, for /bff/ URLs, is allowed.
 */
export function watchConsole(page: Page): string[] {
  const errors: string[] = [];
  page.on("console", (m) => {
    if (m.type() !== "error") return;
    const url = m.location().url;
    if (url.includes("/bff/") && /status of 501/.test(m.text())) return;
    errors.push(`${m.text()} @ ${url}`);
  });
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  return errors;
}

export async function gotoReady(page: Page, path: string) {
  const res = await page.goto(path, { waitUntil: "networkidle" });
  return res;
}


export async function runAxe(page: Page, route: string, width: number): Promise<AxeRecord> {
  const r = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  const count = (i: string) => r.violations.filter((v) => v.impact === i).length;
  const rec: AxeRecord = {
    route, width,
    critical: count("critical"), serious: count("serious"), moderate: count("moderate"), minor: count("minor"),
    violations: r.violations.map((v) => ({ id: v.id, impact: v.impact, nodes: v.nodes.length, help: v.help })),
  };
  recordAxe(rec);
  return rec;
}

export async function noHorizontalOverflow(page: Page) {
  const o = await page.evaluate(() => ({ sw: document.documentElement.scrollWidth, cw: document.documentElement.clientWidth }));
  expect(o.sw, `horizontal overflow: scrollWidth ${o.sw} > clientWidth ${o.cw}`).toBeLessThanOrEqual(o.cw);
}
