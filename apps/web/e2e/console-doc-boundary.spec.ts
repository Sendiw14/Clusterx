import { expect, test, type Page } from "@playwright/test";
import { openConsole } from "./console";
import { PUBLIC_ROUTES } from "./helpers";

/** RV6B-01 (REVIEW-M6-RV-01 round 2): a public page never hands over to the console or /login inside its own, looser-CSP document. */

const scriptSrc = (csp: string) => csp.split(";").map((d) => d.trim()).find((d) => d.startsWith("script-src")) ?? "";

/** Injects an inline event handler and reports whether the page's CSP let it run. */
const inlineHandler = (page: Page) =>
  page.evaluate(async () => {
    const w = window as unknown as { __x?: number };
    delete w.__x;
    const i = document.createElement("img");
    i.setAttribute("onerror", "window.__x=1");
    i.src = "data:image/png;base64,AAAA";
    document.body.append(i);
    await new Promise((r) => setTimeout(r, 400));
    return w.__x === 1 ? "ran" : "blocked";
  });

test.describe("RV6B-01: public to console or login is a full document load under the nonce CSP", () => {
  const cases: [string, string, string, string][] = [
    ["404 page Console button", "/this-page-does-not-exist", 'main a[href="/app"]', "/app"],
    ["header Sign in", "/docs", 'header .pub-nav a[href="/login"]', "/login"],
    ["footer Sign in", "/changelog", 'footer a[href="/login"]', "/login"],
    ["demo header Sign in", "/demo", 'header .pub-nav a[href="/login"]', "/login"],
  ];
  for (const [name, from, link, to] of cases) {
    test(`${name}: new document, CSP without 'unsafe-inline', injected inline handler blocked`, async ({ page }) => {
      await openConsole(page, "operator");
      await page.goto(from);
      await page.evaluate(() => { (window as unknown as { __doc: number }).__doc = 1; });
      const docs: string[] = [];
      page.on("response", (r) => { if (r.request().isNavigationRequest() && new URL(r.url()).pathname === to) docs.push(r.headers()["content-security-policy"] ?? ""); });
      await page.locator(link).first().click();
      await page.waitForURL((u) => u.pathname === to);
      await expect.poll(() => page.evaluate(() => (window as unknown as { __doc?: number }).__doc), { message: "the click must load a new document" }).toBeUndefined();
      expect(docs).toHaveLength(1);
      expect(scriptSrc(docs[0]!)).toContain("'nonce-");
      expect(scriptSrc(docs[0]!)).not.toContain("'unsafe-inline'");
      expect(await inlineHandler(page)).toBe("blocked");
    });
  }

  test("a console-internal navigation stays a client-side navigation", async ({ page }) => {
    await openConsole(page, "operator");
    await page.evaluate(() => { (window as unknown as { __doc: number }).__doc = 1; });
    await page.locator('aside a[href="/app/jobs"]').first().click();
    await page.waitForURL((u) => u.pathname === "/app/jobs");
    expect(await page.evaluate(() => (window as unknown as { __doc?: number }).__doc)).toBe(1);
  });
});

test.describe("every link from a public page into the console or /login loads a new document", () => {
  const into = /^\/(?:app|login|auth)(?:[/?#]|$)/i;
  const pages = [...PUBLIC_ROUTES.filter((r) => r !== "/login"), "/does-not-exist"];
  for (const route of pages) {
    test(`${route}`, async ({ page }) => {
      await page.goto(route);
      const hrefs = [...new Set(await page.locator("a[href]").evaluateAll((as) => as.map((a) => a.getAttribute("href") ?? "")))].filter((h) => into.test(h));
      expect(hrefs.length, "the header and footer always link to sign-in").toBeGreaterThan(0);
      for (const href of hrefs) {
        await page.goto(route);
        await page.evaluate(() => { (window as unknown as { __doc: number }).__doc = 1; });
        await page.locator(`a[href="${href}"]:visible`).first().click();
        await page.waitForURL((u) => u.pathname !== route);
        await expect.poll(() => page.evaluate(() => (window as unknown as { __doc?: number }).__doc), { message: `${route} -> ${href} must load a new document` }).toBeUndefined();
      }
    });
  }

  test("the mobile menu drawer link too", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 800 });
    await page.goto("/docs");
    await page.evaluate(() => { (window as unknown as { __doc: number }).__doc = 1; });
    await page.getByRole("button", { name: "Menu", exact: true }).click();
    await page.getByRole("dialog").getByRole("link", { name: "Sign in" }).click();
    await page.waitForURL((u) => u.pathname === "/login");
    await expect.poll(() => page.evaluate(() => (window as unknown as { __doc?: number }).__doc)).toBeUndefined();
  });
});
