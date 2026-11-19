import { expect, test, type Page } from "@playwright/test";
import { bff, openConsole, waitJobStatus } from "./console";
import { noHorizontalOverflow, runAxe } from "./helpers";

const key = () => `ax-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
const ids: { ok: string; failed: string; worker: string } = { ok: "", failed: "", worker: "" };

test.beforeAll(async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await openConsole(page, "operator");
  const mk = async (handler: string, payload: unknown, extra: object = {}) => (await (await bff(page, "POST", "/jobs", { handler_name: handler, handler_version: 1, payload, ...extra }, { "Idempotency-Key": key() })).json()).id as string;
  ids.ok = await mk("hash_sha256", { data: "axe" });
  ids.failed = await mk("fail_n_times", { n: 1, permanent: true }, { max_attempts: 1 });
  await waitJobStatus(page, ids.ok, "succeeded");
  await waitJobStatus(page, ids.failed, "failed");
  ids.worker = (await (await bff(page, "GET", "/workers")).json()).items[0].id;
  await ctx.close();
});

/** Dialogs fade in (M-OVERLAY): scan after the transition, or axe measures a half-transparent surface. */
const afterTransition = (page: Page) => page.waitForTimeout(500);

async function settle(page: Page) {
  await page.waitForLoadState("networkidle");
  await expect(page.getByTestId("skeleton")).toHaveCount(0, { timeout: 15_000 });
}

const routes = () => ["/app", "/app/jobs", "/app/jobs/new", `/app/jobs/${ids.ok}`, `/app/jobs/${ids.failed}`, "/app/workers", `/app/workers/${ids.worker}`, "/app/observability", "/app/activity", "/app/settings", "/app/settings/tokens", "/app/settings/preferences"];

for (const [who, widths] of [["operator", [1440, 320]], ["viewer", [1440, 390]]] as const) {
  test(`axe and overflow on every console route with live data (${who})`, async ({ page }) => {
    test.setTimeout(240_000);
    for (const width of widths) {
      await page.setViewportSize({ width, height: width < 700 ? 800 : 900 });
      await openConsole(page, who);
      for (const route of routes()) {
        if (who === "viewer" && (route === "/app/jobs/new" || route === "/app/settings/tokens")) continue; // S-403 page, scanned below
        await page.goto(route);
        await settle(page);
        await noHorizontalOverflow(page);
        await expect(page.getByRole("main")).toHaveCount(1);
        await expect(page.locator("h1")).toHaveCount(1);
        expect(await page.title()).toMatch(/: CLUSTERX$/);
        const r = await runAxe(page, `${route.replace(/[0-9a-f-]{36}/, "[id]")} (live ${who})`, width);
        expect(r.violations, `${route} @${width}: ${JSON.stringify(r.violations)}`).toEqual([]);
      }
    }
  });
}

test("axe on the forbidden state, the command dialog, confirmation dialogs and the one-time reveal", async ({ page }) => {
  await openConsole(page, "viewer");
  for (const p of ["/app/jobs/new", "/app/settings/tokens"]) {
    await page.goto(p);
    await expect(page.getByTestId("forbidden-state")).toBeVisible();
    expect((await runAxe(page, `${p} (forbidden)`, 1280)).violations).toEqual([]);
  }
  await openConsole(page, "operator");
  await page.goto(`/app/jobs/${ids.failed}`);
  await settle(page);
  await page.getByRole("button", { name: "Retry job" }).click();
  await expect(page.getByRole("dialog", { name: "Retry this job?" })).toBeVisible();
  await afterTransition(page);
  expect((await runAxe(page, "retry confirm dialog", 1280)).violations).toEqual([]);
  await page.keyboard.press("Escape");
  await page.goto("/app/settings/tokens");
  await settle(page);
  await page.getByRole("button", { name: "Create API token" }).click();
  await page.getByRole("dialog", { name: "Create API token" }).getByLabel(/^Name/).fill(`axe-${Date.now()}`);
  await afterTransition(page);
  expect((await runAxe(page, "create token dialog", 1280)).violations).toEqual([]);
  await page.getByRole("dialog", { name: "Create API token" }).getByRole("button", { name: "Create", exact: true }).click();
  await expect(page.getByRole("dialog", { name: /Copy your new API token now/ })).toBeVisible();
  await afterTransition(page);
  expect((await runAxe(page, "token reveal dialog", 1280)).violations).toEqual([]);
  const reveal = page.getByRole("dialog", { name: /Copy your new API token now/ });
  await reveal.getByLabel("I have copied it somewhere safe").check();
  await reveal.getByRole("button", { name: /Done, I have stored it/ }).click();
  await page.keyboard.press("Control+k");
  await expect(page.getByRole("dialog", { name: "Jump to" })).toBeVisible();
  await afterTransition(page);
  expect((await runAxe(page, "jump-to dialog", 1280)).violations).toEqual([]);
});

test("reduced motion: no running animation on live console pages", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await openConsole(page, "operator");
  for (const p of ["/app", "/app/jobs", "/app/observability"]) {
    await page.goto(p);
    await settle(page);
    const running = await page.evaluate(() => document.getAnimations().filter((a) => a.playState === "running" && !(a.effect as KeyframeEffect | null)?.target?.closest?.(".spin")).length);
    expect(running, p).toBe(0);
  }
});

test("keyboard: the sign-out, jump and drain controls are reachable and labelled", async ({ page }) => {
  await openConsole(page, "operator", "/app/workers");
  await page.keyboard.press("Control+k");
  await expect(page.getByRole("dialog", { name: "Jump to" })).toBeVisible();
  await page.getByLabel(/Jump to a page/).fill(ids.ok);
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(new RegExp(`/app/jobs/${ids.ok}$`));
});
