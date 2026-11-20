import { expect, test, type Page } from "@playwright/test";
import { PUBLIC_ROUTES, runAxe, watchConsole } from "./helpers";

const WIDTHS = [320, 390, 768, 1024, 1440, 1920];

test.describe("public pages: no horizontal overflow at every width", () => {
  for (const w of WIDTHS) {
    test(`overflow @ ${w}`, async ({ page }) => {
      await page.setViewportSize({ width: w, height: 900 });
      for (const r of PUBLIC_ROUTES) {
        await page.goto(r, { waitUntil: "networkidle" });
        const o = await page.evaluate(() => ({ sw: document.documentElement.scrollWidth, cw: document.documentElement.clientWidth }));
        expect(o.sw, `${r} @ ${w}: scrollWidth ${o.sw} > ${o.cw}`).toBeLessThanOrEqual(o.cw);
      }
    });
  }
});

test.describe("public pages: axe at 390 and 1024 (the 320 and 1440 runs are in routes.spec)", () => {
  for (const w of [390, 1024]) {
    test(`axe @ ${w}`, async ({ page }) => {
      await page.setViewportSize({ width: w, height: 900 });
      for (const r of PUBLIC_ROUTES) {
        await page.goto(r, { waitUntil: "networkidle" });
        const rec = await runAxe(page, r, w);
        expect(rec.serious + rec.critical, `${r} @ ${w}: ${JSON.stringify(rec.violations)}`).toBe(0);
      }
    });
  }
});

async function collectLinks(page: Page): Promise<string[]> {
  return page.locator("a[href]").evaluateAll((as) => as.map((a) => a.getAttribute("href")!).filter((h) => h.startsWith("/") && !h.startsWith("//")));
}

test("link checker: every internal link on every public page resolves, and every #fragment exists on its target", async ({ page, request }) => {
  const targets = new Map<string, Set<string>>(); // path -> fragments wanted
  for (const r of PUBLIC_ROUTES) {
    await page.goto(r, { waitUntil: "networkidle" });
    for (const h of await collectLinks(page)) {
      const [p, frag] = h.split("#");
      if (!targets.has(p!)) targets.set(p!, new Set());
      if (frag) targets.get(p!)!.add(frag);
    }
  }
  expect(targets.size).toBeGreaterThan(10);
  for (const [p, frags] of targets) {
    if (p === "/bff/session") continue;
    const res = await request.get(p || "/");
    expect(res.status(), p).toBe(200);
    if (frags.size) {
      const html = await res.text();
      for (const f of frags) expect(html.includes(`id="${f}"`), `${p}#${f}`).toBe(true);
    }
  }
});

test("home: Atlas Play toggles by keyboard, Step and Reset work, nothing autoplays under reduced motion", async ({ browser }) => {
  const ctx = await browser.newContext({ reducedMotion: "reduce", viewport: { width: 1440, height: 900 } });
  const p = await ctx.newPage();
  await p.goto("/", { waitUntil: "networkidle" });
  await p.locator(".atlas-card").scrollIntoViewIfNeeded();
  await p.waitForTimeout(800);
  await expect(p.getByText("Step 0 of 8").first()).toBeVisible(); // did not auto-play
  const play = p.getByRole("button", { name: "Play" });
  await play.focus();
  await p.keyboard.press("Enter");
  await expect(p.getByRole("button", { name: "Pause" })).toHaveAttribute("aria-pressed", "true");
  await p.keyboard.press("Enter");
  await expect(p.getByRole("button", { name: "Play" })).toHaveAttribute("aria-pressed", "false");
  await p.getByRole("button", { name: "Step" }).click();
  await expect(p.getByText(/Step \d of 8 · /).first()).toBeVisible();
  await p.getByRole("button", { name: "Reset" }).click();
  await expect(p.getByText("Step 0 of 8").first()).toBeVisible();
  await ctx.close();
});

test("home: mobile menu opens, traps focus and returns it; Atlas diagram is replaced by the step list at 390", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 800 });
  await page.goto("/", { waitUntil: "networkidle" });
  await expect(page.locator(".atlas-stage")).toBeHidden();
  await expect(page.getByRole("heading", { name: "The eight steps" })).toBeVisible();
  const btn = page.getByRole("button", { name: "Menu" });
  await btn.click();
  await expect(page.getByRole("dialog", { name: "Site menu" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(btn).toBeFocused();
});

test.describe("reduced motion on the new public pages", () => {
  for (const r of ["/", "/architecture", "/docs", "/docs/api", "/changelog", "/demo"]) {
    test(`no running animation after load: ${r}`, async ({ browser }) => {
      const ctx = await browser.newContext({ reducedMotion: "reduce", viewport: { width: 390, height: 800 } });
      const p = await ctx.newPage();
      await p.goto(r, { waitUntil: "networkidle" });
      await p.waitForTimeout(300);
      expect(await p.evaluate(() => document.getAnimations().filter((a) => a.playState === "running").length)).toBe(0);
      await ctx.close();
    });
  }
});

test("no scroll hijacking: wheel scrolls the document by the native amount", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 800 });
  await page.goto("/", { waitUntil: "networkidle" });
  await page.mouse.move(700, 400);
  await page.mouse.wheel(0, 300);
  await page.waitForTimeout(400);
  const y = await page.evaluate(() => window.scrollY);
  expect(y).toBeGreaterThan(200);
  expect(y).toBeLessThan(400);
});

test.describe("architecture and changelog", () => {
  test("architecture: diagram has a title, description and the same facts as text; build status is stated", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/architecture", { waitUntil: "networkidle" });
    const svg = page.locator("svg#arch-svg");
    await expect(svg).toHaveAttribute("role", "img");
    expect(await svg.locator("desc").textContent()).toContain("never to the database");
    await expect(page.getByRole("table", { name: /Connections table|Every connection/ })).toBeVisible();
    const t = await page.locator("main").innerText();
    expect(t, "every layer shown is built; no layer is labelled specified-only").not.toMatch(/Specified, not built|NOT BUILT/i);
    expect(t).toMatch(/tested in Chromium/);
    expect(t).toMatch(/Implemented, tested/);
    expect(t).toMatch(/not a high-availability design/i);
    await expect(page.locator("main").getByText("docs/architecture/adrs/0005-trust-boundaries.md")).toBeVisible();
  });

  test("changelog: only merged milestones, no invented version numbers or release dates", async ({ page }) => {
    await page.goto("/changelog", { waitUntil: "networkidle" });
    await expect(page.getByRole("heading", { name: "Unreleased (pre-1.0)" })).toBeVisible();
    const heads = await page.locator("main h2, main h3").allTextContents();
    for (const h of heads) expect(h.replace("pre-1.0", ""), h).not.toMatch(/\bv\d+\.\d+|\b\d+\.\d+\.\d+/);
    const t = await page.locator("main").innerText();
    expect(t).toMatch(/No releases yet/);
    expect(t).not.toMatch(/released on|Released:/i);
    for (const m of ["M1", "M2", "M3", "M4", "M5", "M6", "M7"]) expect(t).toContain(m);
    expect(t, "M8 is not merged").not.toMatch(/M8: /);
  });
});

test("honesty: no testimonials, customer logos, uptime figures or 'live' demo labels on public pages", async ({ page }) => {
  const errors = watchConsole(page);
  for (const r of ["/", "/architecture", "/changelog", "/demo", "/docs", ...["quickstart", "concepts", "workers", "api", "reliability", "deployment", "security"].map((s) => `/docs/${s}`)]) {
    await page.goto(r, { waitUntil: "networkidle" });
    const t = await page.locator("main").innerText();
    expect(t, r).not.toMatch(/testimonial|trusted by|our customers|\b99\.\d+ ?%|uptime guarantee|production[- ]ready|battle[- ]tested|SOC ?2/i);
    expect(t, r).not.toMatch(/guarantees? exactly[- ]once|provides? exactly[- ]once/i);
  }
  expect(errors).toEqual([]);
});

test("stale copy: no public page says the console or the Next.js server is unbuilt or in development", async ({ page }) => {
  for (const r of ["/", "/architecture", "/docs", "/docs/security", "/docs/api", "/docs/quickstart", "/changelog", "/login", "/demo"]) {
    await page.goto(r);
    const t = await page.locator("body").innerText();
    expect(t, r).not.toMatch(/Specified, not built|NOT BUILT YET|still in development|No session implementation|until it ships/i);
  }
});
