import { test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

/** M7 evidence screenshots (docs/design/screenshots/m7). Opt-in: `SCREENSHOTS_M7=1 pnpm e2e`. Reduced motion shows final states. */
const OUT = resolve(dirname(fileURLToPath(import.meta.url)), "../../../docs/design/screenshots/m7");
test.skip(!process.env.SCREENSHOTS_M7, "set SCREENSHOTS_M7=1 to regenerate the M7 screenshots");
test.use({ reducedMotion: "reduce" });

const demo = (screen: string, extra?: (p: Page) => Promise<void>) => async (p: Page) => {
  await p.getByRole("group", { name: "Demo screen" }).getByRole("button", { name: screen }).click();
  await extra?.(p);
};
const SHOTS: { name: string; path: string; steps?: (p: Page) => Promise<void> }[] = [
  { name: "home", path: "/" },
  { name: "architecture", path: "/architecture" },
  { name: "docs", path: "/docs" },
  { name: "docs-quickstart", path: "/docs/quickstart" },
  { name: "docs-concepts", path: "/docs/concepts" },
  { name: "docs-api", path: "/docs/api" },
  { name: "docs-workers", path: "/docs/workers" },
  { name: "docs-reliability", path: "/docs/reliability" },
  { name: "docs-deployment", path: "/docs/deployment" },
  { name: "docs-security", path: "/docs/security" },
  { name: "docs-search", path: "/docs/concepts", steps: async (p) => { const b = p.getByRole("combobox", { name: "Search the documentation" }).first(); if (await b.isVisible()) { await b.fill("lease"); } } },
  { name: "changelog", path: "/changelog" },
  { name: "demo-overview", path: "/demo" },
  { name: "demo-jobs", path: "/demo", steps: demo("Jobs") },
  { name: "demo-job-failed", path: "/demo", steps: demo("Jobs", async (p) => { await p.getByRole("button", { name: "demo-job-0003" }).first().click(); }) },
  { name: "demo-job-retry", path: "/demo", steps: demo("Jobs", async (p) => { await p.getByRole("button", { name: "demo-job-0013" }).first().click(); }) },
  { name: "demo-workers", path: "/demo", steps: demo("Workers") },
  { name: "demo-recovery", path: "/demo", steps: demo("Recovery replay", async (p) => { await p.getByRole("button", { name: "Show all" }).click(); }) },
];

for (const width of [390, 1440]) {
  for (const s of SHOTS) {
    test(`${s.name} @ ${width}`, async ({ page }) => {
      mkdirSync(OUT, { recursive: true });
      await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
      await page.goto(s.path, { waitUntil: "networkidle" });
      await s.steps?.(page);
      await page.waitForLoadState("networkidle");
      await page.evaluate(() => document.fonts.ready);
      await page.waitForTimeout(400);
      const h = await page.evaluate(() => document.documentElement.scrollHeight);
      // Chromium caps a screenshot near 16k px: the long API page keeps its first 9000 px.
      await page.screenshot({ path: resolve(OUT, `${s.name}-${width}.png`), fullPage: true, ...(h > 9000 ? { clip: { x: 0, y: 0, width, height: 9000 } } : {}) });
    });
  }
}
