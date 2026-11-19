import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import { bff, chooseIdentity, openConsole, signIn, waitJobStatus } from "./console";

/**
 * M6 evidence (docs/design/screenshots/m6) and the real console capture used on `/` (public/images/console-jobs.png).
 * Opt-in: `CAPTURE_M6=1 pnpm exec playwright test e2e/screenshots-m6.spec.ts`. Data is real: it comes from the live E2E
 * stack (own PostgreSQL, real control plane and worker). Secrets in dialogs are masked in the image.
 */
const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const OUT = resolve(ROOT, "docs/design/screenshots/m6");
const PUBLIC = resolve(ROOT, "apps/web/public/images");
test.skip(!process.env.CAPTURE_M6, "set CAPTURE_M6=1 to regenerate the M6 screenshots and the home capture");
test.use({ reducedMotion: "reduce" });
test.describe.configure({ mode: "serial" });

const key = () => `shot-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
const ids = { ok: "", failed: "", running: "", worker: "" };

async function settle(page: Page) {
  await page.waitForLoadState("networkidle");
  await expect(page.getByTestId("skeleton")).toHaveCount(0, { timeout: 15_000 });
  await page.waitForTimeout(300);
}
const shot = async (page: Page, name: string, width: number, opts: { mask?: ReturnType<Page["locator"]>[] } = {}) => {
  mkdirSync(OUT, { recursive: true });
  await page.setViewportSize({ width, height: width < 700 ? 844 : 900 });
  await settle(page);
  await page.screenshot({ path: resolve(OUT, `${name}-${width}.png`), fullPage: width < 700, mask: opts.mask, maskColor: "#1a1a1a" });
};

test("seed live data", async ({ page }) => {
  await openConsole(page, "operator");
  const mk = async (handler: string, payload: unknown, extra: object = {}) => (await (await bff(page, "POST", "/jobs", { handler_name: handler, handler_version: 1, payload, ...extra }, { "Idempotency-Key": key() })).json()).id as string;
  ids.ok = await mk("hash_sha256", { data: "console capture" });
  await mk("hash_sha256", { data: "second" });
  ids.failed = await mk("fail_n_times", { n: 1, permanent: true }, { max_attempts: 1 });
  await mk("fail_n_times", { n: 2 }, { max_attempts: 3 });
  ids.running = await mk("sleep_ms", { ms: 40_000 }, { timeout_seconds: 120 });
  await waitJobStatus(page, ids.ok, "succeeded");
  await waitJobStatus(page, ids.failed, "failed");
  await waitJobStatus(page, ids.running, "running");
  ids.worker = (await (await bff(page, "GET", "/workers")).json()).items[0].id;
});

test("console capture for the home page", async ({ page }) => {
  await openConsole(page, "operator", "/app/jobs");
  await page.setViewportSize({ width: 1280, height: 900 });
  await settle(page);
  const changed = page.getByRole("button", { name: "Refresh list" });
  if (await changed.isVisible().catch(() => false)) { await changed.click(); await settle(page); }
  mkdirSync(PUBLIC, { recursive: true });
  await page.locator("section[aria-labelledby=jl-h]").screenshot({ path: resolve(PUBLIC, "console-jobs.png") });
});

test("console routes on live data", async ({ page }) => {
  test.setTimeout(240_000);
  await openConsole(page, "operator");
  const routes: [string, string][] = [["overview", "/app"], ["jobs", "/app/jobs"], ["jobs-failed", "/app/jobs?status=failed"], ["job-new", "/app/jobs/new"], ["job-succeeded", `/app/jobs/${ids.ok}`], ["job-failed", `/app/jobs/${ids.failed}`], ["job-running", `/app/jobs/${ids.running}`], ["workers", "/app/workers"], ["worker-detail", `/app/workers/${ids.worker}`], ["observability", "/app/observability"], ["activity", "/app/activity"], ["settings", "/app/settings"], ["tokens", "/app/settings/tokens"], ["preferences", "/app/settings/preferences"]];
  for (const width of [1440, 390]) {
    for (const [name, path] of routes) {
      await page.goto(path);
      await shot(page, name, width);
    }
  }
});

test("dialogs, viewer, and error states", async ({ page, browser }) => {
  await openConsole(page, "operator", `/app/jobs/${ids.failed}`);
  await page.getByRole("button", { name: "Retry job" }).click();
  await shot(page, "dialog-retry", 1440);
  await page.keyboard.press("Escape");
  await page.goto("/app/settings/tokens");
  await settle(page);
  await page.getByRole("button", { name: "Create API token" }).click();
  await page.getByRole("dialog", { name: "Create API token" }).getByLabel(/^Name/).fill("ci-submitter");
  await shot(page, "dialog-create-token", 1440);
  await page.getByRole("dialog", { name: "Create API token" }).getByRole("button", { name: "Create", exact: true }).click();
  const reveal = page.getByRole("dialog", { name: /Copy your new API token now/ });
  await expect(reveal).toBeVisible();
  await shot(page, "dialog-token-reveal", 1440, { mask: [reveal.getByTestId("secret-value")] });
  await reveal.getByLabel("I have copied it somewhere safe").check();
  await reveal.getByRole("button", { name: /Done, I have stored it/ }).click();

  const v = await browser.newContext({ reducedMotion: "reduce" });
  const vp = await v.newPage();
  await openConsole(vp, "viewer", `/app/jobs/${ids.ok}`);
  await shot(vp, "viewer-job-masked", 1440);
  await vp.goto("/app/settings/tokens");
  await shot(vp, "viewer-forbidden", 1440);
  await v.close();

  const e = await browser.newContext({ reducedMotion: "reduce" });
  const ep = await e.newPage();
  await ep.route("**/bff/api/v1/jobs?*", (r) => r.fulfill({ status: 503, headers: { "Content-Type": "application/json", "Retry-After": "2" }, body: JSON.stringify({ code: "unavailable", message: "Not ready.", request_id: "req_sim503" }) }));
  await openConsole(ep, "operator", "/app/jobs");
  await shot(ep, "state-error-503-simulated", 1440);
  await e.close();

  const s = await browser.newContext({ reducedMotion: "reduce" });
  const sp = await s.newPage();
  await chooseIdentity(s, "stranger");
  await sp.goto("/auth/github");
  await shot(sp, "login-no-entitlement", 390);
  await s.close();
  void signIn;
});
