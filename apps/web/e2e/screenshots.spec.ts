import { test } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

/** Evidence screenshots (docs/design/screenshots/m3). Opt-in: `SCREENSHOTS=1 pnpm e2e`. Reduced motion shows final states. */
const OUT = resolve(dirname(fileURLToPath(import.meta.url)), "../../../docs/design/screenshots/m3");
test.skip(!process.env.SCREENSHOTS, "set SCREENSHOTS=1 to regenerate evidence screenshots");
test.use({ reducedMotion: "reduce" });

const SHOTS: { name: string; path: string; steps?: (p: import("@playwright/test").Page) => Promise<void> }[] = [
  { name: "home", path: "/" },
  { name: "jobs", path: "/app/jobs" },
  { name: "jobs-failed-filter", path: "/app/jobs?status=failed&priority_min=5" },
  { name: "demo", path: "/demo" },
  { name: "demo-jobs", path: "/demo", steps: async (p) => { await p.getByRole("group", { name: "Demo screen" }).getByRole("button", { name: "Jobs" }).click(); } },
  { name: "demo-job-viewer", path: "/demo", steps: async (p) => {
    await p.getByRole("group", { name: "Demo screen" }).getByRole("button", { name: "Jobs" }).click();
    await p.getByRole("radio", { name: "View as viewer" }).click();
    await p.getByRole("button", { name: "demo-job-0003" }).first().click();
  } },
  { name: "login", path: "/login?error=no_entitlement" },
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
      await page.screenshot({ path: resolve(OUT, `${s.name}-${width}.png`), fullPage: true });
    });
  }
}
