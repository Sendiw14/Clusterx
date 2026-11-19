import { expect, test } from "@playwright/test";
import { openConsole } from "./console";
import { cookieOf, raw } from "./raw";

/** RV6-09: nonce-based CSP on the console, from REVIEW-M6-RV-01. */
test.describe("RV6-09: console responses carry a nonce CSP without 'unsafe-inline' in script-src", () => {
  const scriptSrc = (csp: string) => csp.split(";").map((d) => d.trim()).find((d) => d.startsWith("script-src")) ?? "";

  test("/app, /login and /auth/* use a fresh nonce with strict-dynamic; no inline-script allowance", async ({ page }) => {
    await openConsole(page, "operator");
    const cookie = await cookieOf(page);
    const nonces = new Set<string>();
    for (const p of ["/app", "/app/jobs", "/login", "/auth/end?reason=session_expired"]) {
      const r = await raw("GET", p, p.startsWith("/auth") ? {} : { Cookie: cookie });
      const csp = String(r.headers["content-security-policy"] ?? "");
      const s = scriptSrc(csp);
      expect(s, p).not.toContain("'unsafe-inline'");
      expect(s, p).toContain("'strict-dynamic'");
      const m = /'nonce-([A-Za-z0-9+/=_-]{16,})'/.exec(s);
      expect(m, `${p}: ${csp}`).not.toBeNull();
      nonces.add(m![1]!);
      expect(csp, p).toContain("frame-ancestors 'none'");
      expect(csp, p).toContain("object-src 'none'");
    }
    expect(nonces.size, "nonce is fresh per response").toBe(4);
  });

  test("scripts in the console HTML carry the response nonce, and the browser reports no CSP violation", async ({ page }) => {
    const violations: string[] = [];
    await page.addInitScript(() => document.addEventListener("securitypolicyviolation", (e) => (window as unknown as { __v: string[] }).__v?.push(`${e.violatedDirective} ${e.blockedURI}`)));
    await page.addInitScript(() => { (window as unknown as { __v: string[] }).__v = []; });
    page.on("console", (m) => { if (/Content Security Policy/i.test(m.text())) violations.push(m.text().slice(0, 200)); });
    await openConsole(page, "operator");
    for (const p of ["/app", "/app/jobs", "/app/workers", "/app/settings/tokens"]) {
      await page.goto(p);
      await expect(page.locator("main")).toBeVisible();
      const html = await page.content();
      const nonce = /'nonce-([^']+)'/.exec((await (await page.request.get(p)).headers())["content-security-policy"] ?? "")?.[1];
      expect(nonce, p).toBeTruthy();
      // Inline <script> elements exist (Next's flight payload) and every one has a nonce attribute.
      const inline = await page.$$eval("script:not([src])", (els) => els.map((e) => ({ nonce: e.nonce || e.getAttribute("nonce") })));
      expect(inline.length, p).toBeGreaterThan(0);
      for (const s of inline) expect(s.nonce, p).toBeTruthy();
      expect(html).not.toContain("javascript:");
    }
    expect(violations).toEqual([]);
    expect(await page.evaluate(() => (window as unknown as { __v: string[] }).__v ?? [])).toEqual([]);
  });

  test("public pages keep their static policy and still render", async ({ page }) => {
    const r = await page.request.get("/docs");
    expect(r.status()).toBe(200);
    expect(r.headers()["content-security-policy"]).toContain("default-src 'self'");
    await page.goto("/");
    await expect(page.locator("h1").first()).toBeVisible();
  });
});
