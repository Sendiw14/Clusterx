import { expect, test } from "@playwright/test";
import { PUBLIC_ROUTES, GATED_ROUTES, gotoReady, noHorizontalOverflow, runAxe, watchConsole } from "./helpers";

const WIDTHS = [320, 1440];
// Console routes with live data (job and worker detail pages included) are scanned in console-a11y.spec.ts.

test.describe("route shells", () => {
  for (const width of WIDTHS) {
    for (const route of PUBLIC_ROUTES) {
      test(`${route} @ ${width}`, async ({ page }) => {
        await page.setViewportSize({ width, height: 900 });
        const errors = watchConsole(page);
        const res = await gotoReady(page, route);
        expect(res?.status(), "HTTP status").toBe(200);

        // landmarks and structure (E2E-*-01)
        await expect(page.getByRole("main")).toHaveCount(1);
        await expect(page.getByRole("banner")).toHaveCount(1);
        await expect(page.locator("h1")).toHaveCount(1);
        expect(await page.title()).toMatch(/: CLUSTERX$/);
        await expect(page.getByRole("link", { name: "Skip to content" })).toHaveCount(1);
        if (route.startsWith("/app")) await expect(page.locator("nav[aria-label=Console]").first()).toBeAttached();
        else await expect(page.getByRole("contentinfo")).toHaveCount(1);

        await noHorizontalOverflow(page);

        // axe: zero serious/critical (E2E-*-11)
        const rec = await runAxe(page, route, width);
        expect(rec.serious + rec.critical, JSON.stringify(rec.violations)).toBe(0);

        expect(errors, "console errors").toEqual([]);
      });
    }
  }

  test("1.1 gated routes answer the global 404 with the brand shell", async ({ page }) => {
    for (const r of GATED_ROUTES) {
      const res = await page.goto(r);
      expect(res?.status(), r).toBe(404);
      await expect(page.getByRole("heading", { level: 1 })).toContainText("does not exist");
      await expect(page.getByRole("banner")).toHaveCount(1);
    }
  });

  test("unknown URL is a 404 and names no stack trace", async ({ page }) => {
    const res = await page.goto("/definitely/not/here");
    expect(res?.status()).toBe(404);
    expect(await page.content()).not.toMatch(/at \w+ \(/);
  });

});
