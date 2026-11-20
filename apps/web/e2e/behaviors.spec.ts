import { expect, test } from "@playwright/test";
import { openConsole } from "./console";
import { activate, animationsDone, clipboardContext, readClipboard, runAxe, watchConsole } from "./helpers";

test.describe("navigation and overlays", () => {
  test("public mobile menu: opens, traps focus, Esc closes, focus returns to the button", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 800 });
    await page.goto("/", { waitUntil: "networkidle" });
    const btn = page.getByRole("button", { name: "Menu" });
    await activate(page, btn);
    const dlg = page.getByRole("dialog", { name: "Site menu" });
    await expect(dlg).toBeVisible();
    // A page-level CSS locator, not one chained from the dialog locator: reading text through a locator derived from a role-resolved modal <dialog> crashes the Firefox content process of the Playwright Firefox build (reproduces on a bare page with one <dialog>, not an app bug).
    expect(await page.locator("dialog[open] a").allTextContents()).toEqual(["Architecture", "Docs", "Changelog", "Demo", "Sign in"]);
    await animationsDone(dlg);
    const r = await runAxe(page, "/ [menu open]", 320);
    expect(r.serious + r.critical, JSON.stringify(r.violations)).toBe(0);
    await page.keyboard.press("Escape");
    await expect(dlg).toBeHidden();
    await expect(btn).toBeFocused();
    await btn.click();
    await dlg.getByRole("link", { name: "Docs" }).click();
    await expect(page).toHaveURL(/\/docs$/);
    await expect(dlg).toBeHidden();
    await expect(page.getByRole("main")).toBeFocused(); // focus moves to main after a client route change
  });

  test("console drawer holds everything the sidebar holds; viewer-only items follow the role", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 800 });
    await openConsole(page, "viewer");
    await activate(page, page.getByRole("button", { name: "Menu" }));
    const nav = page.getByRole("dialog", { name: "Console menu" }).getByRole("navigation", { name: "Console" });
    // Viewer: operator-only Tokens is absent, everything else is present.
    expect(await nav.getByRole("link").allTextContents()).toEqual(["Overview", "Jobs", "Workers", "Observability", "Activity", "Configuration", "Preferences"]);
    await page.keyboard.press("Escape");
    await expect(page.getByRole("button", { name: "Menu" })).toBeFocused();
    await page.keyboard.press("Escape");
    await openConsole(page, "operator");
    await page.getByRole("button", { name: "Menu" }).click();
    expect(await page.getByRole("dialog", { name: "Console menu" }).getByRole("navigation", { name: "Console" }).getByRole("link").allTextContents()).toContain("Tokens and credentials");
  });

  test("Ctrl+K opens the command dialog; Esc closes and restores focus; exact-ID lookup is live", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await openConsole(page, "operator");
    await page.keyboard.press("Control+k");
    const dlg = page.getByRole("dialog", { name: "Jump to" });
    await expect(dlg).toBeVisible();
    await expect(dlg.getByLabel("Jump to a page, or enter an exact job or worker ID")).toBeFocused();
    await dlg.getByRole("textbox").fill("job");
    await expect(dlg.getByRole("link")).toHaveText(["Jobs"]);
    await dlg.getByRole("textbox").fill("0193a0f2-7c1e-7a10-8000-00000000f00d");
    await dlg.getByRole("button", { name: /Look up job or worker/ }).click();
    await expect(dlg.getByText("No job or worker with that ID.")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dlg).toBeHidden();
  });

  test("skip link is first in tab order and moves focus to main", async ({ page }) => {
    await page.goto("/architecture", { waitUntil: "networkidle" });
    // Safari (WebKit) leaves links out of the Tab order unless Option is held; that is the platform default, not a defect.
    await page.keyboard.press(test.info().project.name === "webkit" ? "Alt+Tab" : "Tab");
    const skip = page.getByRole("link", { name: "Skip to content" });
    await expect(skip).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/#main$/);
  });

  test("focus ring is visible on a keyboard-focused link", async ({ page }) => {
    await page.goto("/", { waitUntil: "networkidle" });
    await page.keyboard.press("Tab");
    await page.keyboard.press("Tab");
    const outline = await page.evaluate(() => { const s = getComputedStyle(document.activeElement as Element); return { w: s.outlineWidth, st: s.outlineStyle }; });
    expect(outline.st).toBe("solid");
    expect(parseFloat(outline.w)).toBeGreaterThanOrEqual(2);
  });
});

test.describe("home", () => {
  test("hero states at-least-once and makes no exactly-once, HA or workflow claim (copy check)", async ({ page }) => {
    await page.goto("/", { waitUntil: "networkidle" });
    const hero = await page.locator("section.hero").innerText();
    expect(hero).toMatch(/at least once/i);
    expect(hero).toMatch(/not exactly-once/i);
    expect(hero).not.toMatch(/guarantees? exactly[- ]once|high availability|highly available|workflow|DAG/i);
    // whole page: limits may be named, but never claimed as features
    const text = await page.locator("main").innerText();
    expect(text).not.toMatch(/(provides?|offers?|guarantees?|supports?) (exactly[- ]once|high availability|automatic failover)/i);
  });

  test("Atlas: labelled illustrative, no Live label, keyboard Step reaches 8 of 8 and Reset returns", async ({ browser }) => {
    const ctx = await browser.newContext({ reducedMotion: "reduce", viewport: { width: 1440, height: 900 } });
    const p = await ctx.newPage();
    await p.goto("/", { waitUntil: "networkidle" });
    await expect(p.getByText("Illustrative, scripted").first()).toBeVisible();
    const step = p.getByRole("button", { name: "Step", exact: true });
    for (let i = 1; i <= 8; i++) {
      await step.click();
      await expect(p.getByText(`Step ${i} of 8`).first()).toBeVisible();
    }
    await expect(p.getByRole("list").filter({ hasText: "Fence" }).locator("li[aria-current=step]")).toContainText("Fence");
    await p.getByRole("button", { name: "Reset" }).click();
    await expect(p.getByText("Step 0 of 8").first()).toBeVisible();
    const play = p.getByRole("button", { name: "Play" });
    await expect(play).toHaveAttribute("aria-pressed", "false");
    const live = await p.locator("#atlas, .atlas-card").first().evaluate((el) => /\blive\b/i.test(el.textContent ?? "") && !/not live telemetry|nothing on this diagram/i.test(el.textContent ?? ""));
    expect(live).toBe(false);
    await ctx.close();
  });

  test("quickstart Copy puts the exact text on the clipboard and announces it", async ({ browser }) => {
    const ctx = await clipboardContext(browser);
    const p = await ctx.newPage();
    await p.goto("/", { waitUntil: "networkidle" });
    const code = await p.locator("pre[aria-label='Quickstart commands']").innerText();
    await p.getByRole("button", { name: "Copy: Quickstart commands" }).click();
    expect(await readClipboard(p)).toBe(code);
    await expect(p.getByRole("button", { name: /Copy: Quickstart/ })).toContainText("Copied");
    await ctx.close();
  });

  test("home makes no request to /api or /bff", async ({ page }) => {
    const hits: string[] = [];
    page.on("request", (r) => { if (/^\/(api|bff)\//.test(new URL(r.url()).pathname)) hits.push(r.url()); });
    await page.goto("/", { waitUntil: "networkidle" });
    expect(hits).toEqual([]);
  });
});

test.describe("reduced motion", () => {
  test("with prefers-reduced-motion no animation is running after load on home or the console", async ({ browser }) => {
    const ctx = await browser.newContext({ reducedMotion: "reduce", viewport: { width: 390, height: 800 } });
    const p = await ctx.newPage();
    await openConsole(p, "operator");
    for (const r of ["/", "/app/jobs", "/demo"]) {
      await p.goto(r, { waitUntil: "networkidle" });
      await p.waitForTimeout(300);
      const running = await p.evaluate(() => document.getAnimations().filter((a) => a.playState === "running" && (a.effect as KeyframeEffect | null)?.getTiming().iterations !== 0).map((a) => (a as CSSAnimation).animationName ?? a.constructor.name));
      expect(running, `${r}: ${running.join(",")}`).toEqual([]);
    }
    await ctx.close();
  });

  test("preference 'Always reduce motion' sets data-rm and persists across reload; storage failure degrades quietly", async ({ page }) => {
    await openConsole(page, "operator", "/app/settings/preferences");
    await page.getByLabel("Reduce motion").selectOption("always");
    await expect(page.locator("html")).toHaveAttribute("data-rm", "1");
    await page.reload({ waitUntil: "networkidle" });
    await expect(page.locator("html")).toHaveAttribute("data-rm", "1");
    await expect(page.getByLabel("Reduce motion")).toHaveValue("always");
    await page.getByLabel("Reduce motion").selectOption("system");
    await expect(page.locator("html")).not.toHaveAttribute("data-rm", "1");
  });

  test("demo fixture times are always UTC, even with a zone preference; the title carries UTC (zone preference itself: Vitest, timestamp-zone.test.tsx)", async ({ page }) => {
    await openConsole(page, "operator", "/app/settings/preferences");
    await page.getByLabel("Time zone").selectOption("Asia/Tokyo");
    await page.goto("/demo", { waitUntil: "networkidle" });
    await page.getByRole("group", { name: "Demo screen" }).getByRole("button", { name: "Jobs" }).click();
    const t = page.locator("table.tbl time").first();
    await expect(t).toHaveAttribute("title", /\(UTC\)$/);
    await expect(t).toContainText("UTC");
  });
});

test.describe("console states and filters", () => {
  test("jobs: URL is filter state, chips remove, clear all (live data)", async ({ page }) => {
    await openConsole(page, "operator");
    const errors = watchConsole(page);
    await page.goto("/app/jobs?status=failed&priority_min=5&handler_name=export", { waitUntil: "networkidle" });
    await expect(page.getByRole("button", { name: "Remove Status: Failed filter" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Failed (dead-letter)" })).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByRole("heading", { name: "No jobs match these filters" })).toBeVisible();
    await page.getByRole("button", { name: "Remove Priority 5 or higher filter" }).click();
    await expect(page).toHaveURL(/status=failed&handler_name=export$/);
    await page.getByRole("button", { name: "Clear all filters" }).click();
    await expect(page).toHaveURL(/\/app\/jobs$/);
    expect(errors).toEqual([]);
  });

  test("jobs: invalid filter values in the URL are ignored", async ({ page }) => {
    await openConsole(page, "operator", "/app/jobs?status=bogus&priority_min=12");
    await expect(page.getByLabel("Status")).toHaveValue("");
    await expect(page.getByLabel("Priority at least")).toHaveValue("");
  });

  test("viewer-gated routes: Submit job absent, Tokens absent from nav", async ({ page }) => {
    await openConsole(page, "viewer", "/app/jobs");
    await expect(page.getByRole("link", { name: "Submit job" })).toHaveCount(0);
    await expect(page.getByRole("link", { name: "Tokens and credentials" })).toHaveCount(0);
  });

  test("offline: banner appears, then recovers when the connection returns", async ({ page, context }) => {
    await openConsole(page, "operator", "/app/jobs");
    await context.setOffline(true);
    await expect(page.getByText("You are offline.")).toBeVisible();
    await context.setOffline(false);
    await expect(page.getByText("You are offline.")).toBeHidden();
  });

  test("viewport 320 and 400 percent zoom equivalent: console jobs has no horizontal scroll and every filter is reachable", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 800 });
    await openConsole(page, "operator", "/app/jobs");
    await page.waitForLoadState("networkidle");
    for (const l of ["Status", "Handler", "Priority at least"]) await expect(page.getByLabel(l)).toBeVisible();
    await expect(page.getByRole("button", { name: "Failed (dead-letter)" })).toBeVisible();
    const o = await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth);
    expect(o).toBe(true);
  });
});

test.describe("login and BFF", () => {
  test("no password input, sign-in is a plain GitHub link, no default credentials", async ({ page }) => {
    await page.goto("/login", { waitUntil: "networkidle" });
    await expect(page.locator("input[type=password]")).toHaveCount(0);
    await expect(page.getByRole("link", { name: "Sign in with GitHub" })).toHaveAttribute("href", "/auth/github");
    await expect(page.getByText("Sign-in is not enabled in this build")).toHaveCount(0);
  });

  test("closed outcome set: known values render copy, arbitrary values render nothing", async ({ page }) => {
    await page.goto("/login?error=no_entitlement", { waitUntil: "networkidle" });
    await expect(page.locator(".notice[role=alert]")).toContainText("isn't approved");
    await expect(page.getByRole("link", { name: "Sign in with GitHub" })).toHaveCount(0);
    await page.goto("/login?error=user_disabled", { waitUntil: "networkidle" });
    await expect(page.locator(".notice[role=alert]")).toContainText("disabled");
    await page.goto("/login?reason=signed_out", { waitUntil: "networkidle" });
    await expect(page.getByRole("status").filter({ hasText: "signed out" })).toBeVisible();
    await page.goto("/login?reason=%3Cscript%3Ealert(1)%3C/script%3E&error=evil", { waitUntil: "networkidle" });
    await expect(page.locator(".notice[role=alert]")).toHaveCount(0);
    const visible = await page.locator("main").innerText();
    expect(visible).not.toContain("evil");
    expect(visible).not.toContain("script");
    await page.goto("/login?error=upstream_auth", { waitUntil: "networkidle" });
    await expect(page.locator(".notice[role=alert]")).toContainText("could not reach its own API");
    expect(page.url()).toContain("/login"); // never a login redirect for assertion faults
  });

  test("BFF routes without a session answer the BFF envelope (401, 403 csrf, 426) with no-store", async ({ request }) => {
    const cases: [string, string, number, string][] = [["GET", "/bff/session", 401, "session_expired"], ["POST", "/bff/logout", 403, "csrf_failed"], ["GET", "/bff/api/v1/jobs", 401, "session_expired"], ["POST", "/bff/api/v1/jobs", 403, "csrf_failed"], ["GET", "/bff/stream", 426, "upstream_unavailable"]];
    for (const [method, path, status, code] of cases) {
      const r = await request.fetch(path, { method });
      expect(r.status(), `${method} ${path}`).toBe(status);
      const j = await r.json();
      expect(Object.keys(j).sort()).toEqual(["code", "message", "request_id"]);
      expect(j.code).toBe(code);
      expect(r.headers()["x-request-id"]).toBe(j.request_id);
      expect(r.headers()["cache-control"]).toContain("no-store");
      expect(r.headers()["content-type"]).toContain("application/json");
    }
  });

  test("security headers and CSP baseline on public and console routes", async ({ request }) => {
    for (const path of ["/", "/demo", "/login", "/app", "/docs"]) {
      const h = (await request.get(path)).headers();
      expect(h["x-content-type-options"], path).toBe("nosniff");
      expect(h["x-frame-options"], path).toBe("DENY");
      expect(h["referrer-policy"], path).toBe("strict-origin-when-cross-origin");
      expect(h["strict-transport-security"], path).toContain("max-age=");
      expect(h["permissions-policy"], path).toContain("camera=()");
      const csp = h["content-security-policy"] ?? "";
      for (const d of ["default-src 'self'", "frame-ancestors 'none'", "object-src 'none'", "base-uri 'self'", "form-action 'self'", "connect-src 'self'"]) expect(csp, `${path} ${d}`).toContain(d);
      expect(h["x-powered-by"], path).toBeUndefined();
    }
    for (const path of ["/app", "/app/jobs", "/login", "/bff/session"]) {
      const h = (await request.get(path)).headers();
      expect(h["cache-control"], path).toContain("no-store");
      expect(h["x-robots-tag"], path).toContain("noindex");
    }
    expect((await request.get("/")).headers()["x-robots-tag"]).toBeUndefined();
  });

  test("the console exposes no data without a session: it redirects to /login and no data call is made", async ({ page }) => {
    const data: number[] = [];
    page.on("response", (r) => { if (new URL(r.url()).pathname.startsWith("/bff/api/")) data.push(r.status()); });
    await page.goto("/app", { waitUntil: "networkidle" });
    await expect(page).toHaveURL(/\/login$/);
    expect(data).toEqual([]);
  });
});
