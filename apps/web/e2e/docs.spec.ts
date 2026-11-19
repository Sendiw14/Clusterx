import { readFileSync } from "node:fs";
import path from "node:path";
import { expect, test } from "@playwright/test";
import { runAxe } from "./helpers";

const DOCS = ["quickstart", "concepts", "workers", "api", "reliability", "deployment", "security"];
const SPEC_VERSION = /version: (1\.0\.0-draft\.\d+)/.exec(readFileSync(path.join(process.cwd(), "..", "..", "api", "openapi.yaml"), "utf8"))![1]!;

test.describe("docs framework", () => {
  for (const slug of DOCS) {
    test(`/docs/${slug}: sidebar current page, on-this-page, prev/next, anchors resolve`, async ({ page }) => {
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(`/docs/${slug}`, { waitUntil: "networkidle" });
      await expect(page.getByRole("navigation", { name: "Documentation" }).locator("a[aria-current=page]")).toHaveCount(1);
      const otp = page.getByRole("navigation", { name: "On this page", exact: true });
      await expect(otp).toBeVisible();
      const hrefs = await otp.locator("a").evaluateAll((as) => as.map((a) => a.getAttribute("href")!));
      expect(hrefs.length).toBeGreaterThan(2);
      for (const h of hrefs) expect(await page.locator(`[id="${h.slice(1)}"]`).count(), h).toBe(1);
      await expect(page.getByRole("navigation", { name: "Previous and next page" })).toBeVisible();
    });
  }

  test("on-this-page marks the section in view", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 700 });
    await page.goto("/docs/concepts", { waitUntil: "networkidle" });
    await page.evaluate(() => document.getElementById("fencing")!.scrollIntoView());
    await expect(page.getByRole("navigation", { name: "On this page", exact: true }).locator("a[aria-current=location]")).toHaveCount(1);
  });

  test("search: finds pages and API operations, keyboard only, / focuses, Esc clears", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/docs", { waitUntil: "networkidle" });
    await page.keyboard.press("/");
    const box = page.getByRole("combobox", { name: "Search the documentation" }).first();
    await expect(box).toBeFocused();
    await box.fill("fencing");
    const list = page.getByRole("listbox", { name: "Search results" });
    await expect(list).toBeVisible();
    await expect(list.getByRole("option").first()).toContainText("Fencing");
    await expect(page.getByRole("status").filter({ hasText: /results?$/ }).first()).toBeAttached();
    await page.keyboard.press("Escape");
    await expect(box).toHaveValue("");
    await box.fill("createJob");
    await expect(list.getByRole("option").first()).toContainText("createJob");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowUp");
    await page.keyboard.press("Enter");
    await page.waitForURL(/\/docs\/api#createJob/);
    await expect(page.locator("#createJob")).toBeVisible();
  });

  test("search: empty result message, no results never claims a hit; axe with the popup open", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/docs/concepts", { waitUntil: "networkidle" });
    const box = page.getByRole("combobox", { name: "Search the documentation" }).first();
    await box.fill("zzqqxx");
    await expect(page.getByText(/No results for/)).toBeVisible();
    await box.fill("lease");
    await expect(page.getByRole("listbox", { name: "Search results" })).toBeVisible();
    const r = await runAxe(page, "/docs/concepts [search open]", 1440);
    expect(r.serious + r.critical, JSON.stringify(r.violations)).toBe(0);
  });

  test("search index is a same-origin static file built from the docs", async ({ request }) => {
    const res = await request.get("/docs/search-index.json");
    expect(res.status()).toBe(200);
    const idx = (await res.json()) as { u: string; h: string }[];
    expect(idx.length).toBeGreaterThan(100);
    expect(idx.some((e) => e.u === "/docs/api#createJob")).toBe(true);
  });

  test("code blocks have a copy button that copies the exact text and announces it", async ({ browser }) => {
    const ctx = await browser.newContext({ permissions: ["clipboard-read", "clipboard-write"], viewport: { width: 1440, height: 900 } });
    const p = await ctx.newPage();
    await p.goto("/docs/quickstart", { waitUntil: "networkidle" });
    const pre = p.locator("pre[aria-label^='Code: 2. Bootstrap the stack']");
    const code = await pre.innerText();
    expect(code).toContain("./infra/compose/bootstrap.sh");
    await p.getByRole("button", { name: /^Copy: Code: 2\. Bootstrap the stack/ }).click();
    expect(await p.evaluate(() => navigator.clipboard.readText())).toBe(code);
    await expect(p.getByRole("button", { name: /^Copy: Code: 2\. Bootstrap/ })).toContainText("Copied");
    await ctx.close();
  });

  test("mobile: Docs menu drawer holds search and navigation, traps focus, Esc restores focus", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 800 });
    await page.goto("/docs/reliability", { waitUntil: "networkidle" });
    const btn = page.getByRole("button", { name: "Docs menu" });
    await btn.click();
    const dlg = page.getByRole("dialog", { name: "Docs menu" });
    await expect(dlg).toBeVisible();
    await expect(dlg.getByRole("combobox")).toBeVisible();
    await expect(dlg.getByRole("link", { name: "Quickstart" })).toBeVisible();
    await dlg.getByRole("combobox").fill("backoff");
    await expect(dlg.getByRole("listbox")).toBeVisible();
    await page.keyboard.press("Escape");
    await page.keyboard.press("Escape");
    await expect(dlg).toBeHidden();
    await expect(btn).toBeFocused();
    // the inline on-this-page list is reachable on narrow screens
    await page.getByText("On this page", { exact: true }).first().click();
    await expect(page.getByRole("navigation", { name: "On this page (inline)" })).toBeVisible();
  });
});

test.describe("API reference (generated)", () => {
  test("shows the spec version, every operation, served status and the draft warning", async ({ page }) => {
    await page.goto("/docs/api", { waitUntil: "networkidle" });
    const text = await page.locator("main").innerText();
    expect(text).toContain(SPEC_VERSION);
    expect(text).toMatch(/draft/i);
    const spec = readFileSync(path.join(process.cwd(), "..", "..", "api", "openapi.yaml"), "utf8");
    const ids = [...spec.matchAll(/operationId: (\w+)/g)].map((m) => m[1]!);
    for (const id of ids) expect(await page.locator(`section#${id}`).count(), id).toBe(1);
    await expect(page.locator("section#createJob").getByText("Served in this build")).toBeVisible();
    await expect(page.locator("section#cancelJob").getByText(/Not served yet/)).toBeVisible();
    await expect(page.locator("section#createJob pre[aria-label^='Example request']").first()).toBeVisible();
    await expect(page.getByRole("heading", { name: "Error codes and statuses" })).toBeVisible();
  });
});

test("quickstart and deployment mark what is not verified; production is not claimed", async ({ page }) => {
  await page.goto("/docs/quickstart", { waitUntil: "networkidle" });
  const qs = await page.locator("main").innerText();
  expect(qs).toMatch(/Not yet verified/);
  expect(qs).toMatch(/Verified\./);
  await page.goto("/docs/deployment", { waitUntil: "networkidle" });
  expect(await page.locator("main").innerText()).toMatch(/not yet verified/i);
  await page.goto("/docs/reliability", { waitUntil: "networkidle" });
  const rel = await page.locator("main").innerText();
  expect(rel).toMatch(/No exactly-once execution/);
  expect(rel).toMatch(/not a throughput claim/);
});
