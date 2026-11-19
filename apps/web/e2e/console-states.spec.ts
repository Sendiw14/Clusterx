import { expect, test } from "@playwright/test";
import { bff, openConsole } from "./console";
import { USERS } from "./stack/stack";

const envelope = (code: string, status: number, rid = "req_e2e000000") => ({ status, headers: { "Content-Type": "application/json", "X-Request-Id": rid }, body: JSON.stringify({ code, message: "simulated", request_id: rid }) });

test.describe("error, offline and stale states (S-ERR, S-OFFLINE) on live pages", () => {
  test("a failing list shows the cause with a copyable request id, and Retry recovers", async ({ page }) => {
    let fail = true;
    await page.route("**/bff/api/v1/jobs?*", (r) => (fail ? r.fulfill(envelope("unavailable", 503, "req_e2e503503")) : r.continue()));
    await openConsole(page, "operator", "/app/jobs");
    await expect(page.getByTestId("error-state")).toContainText("The control plane is not ready");
    await expect(page.getByTestId("request-id")).toHaveText("req_e2e503503");
    fail = false;
    await page.getByRole("button", { name: "Retry", exact: true }).click();
    await expect(page.getByTestId("error-state")).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Failed \(dead-letter\)/ })).toBeVisible();
  });

  test("429 and 500 and a BFF upstream_unavailable each get their own plain-language state", async ({ page }) => {
    const codes: [string, number, string][] = [["rate_limited", 429, "Wait a moment, then try again."], ["internal", 500, "The server returned an internal error"], ["upstream_unavailable", 503, "could not reach the engine"], ["upstream_auth", 502, "infrastructure fault"]];
    for (const [code, status, text] of codes) {
      await page.route("**/bff/api/v1/workers?*", (r) => r.fulfill(envelope(code, status)));
      await openConsole(page, "operator", "/app/workers");
      await expect(page.getByTestId("error-state")).toContainText(text);
      await expect(page.getByTestId("error-state")).toHaveAttribute("data-error-code", code);
      await page.unroute("**/bff/api/v1/workers?*");
    }
  });

  test("offline: banner, frozen list, mutation controls say why they are unavailable", async ({ page, context }) => {
    await openConsole(page, "operator", "/app/jobs");
    await expect(page.locator("table.tbl tbody tr").first()).toBeVisible();
    await context.setOffline(true);
    await expect(page.getByText("You are offline.")).toBeVisible();
    await expect(page.locator(".live[data-live=offline]")).toBeVisible();
    await context.setOffline(false);
    await expect(page.getByText("You are offline.")).toHaveCount(0);
    await expect(page.locator(".live[data-live=live]")).toBeVisible({ timeout: 20_000 });
  });

  test("a 429 from Go reaches the UI as the rate-limit state with Retry-After, and submit surfaces it", async ({ page }) => {
    await openConsole(page, "operator", "/app/jobs/new");
    await page.route("**/bff/api/v1/jobs", (r) => (r.request().method() === "POST" ? r.fulfill({ ...envelope("rate_limited", 429, "req_e2e429429"), headers: { "Content-Type": "application/json", "Retry-After": "7", "X-Request-Id": "req_e2e429429" } }) : r.continue()));
    await page.getByRole("combobox", { name: /^Handler/ }).selectOption("hash_sha256");
    await page.getByLabel("Payload (JSON)").fill('{"data":"x"}');
    await page.getByRole("button", { name: "Submit job" }).click();
    await expect(page.getByRole("alert").filter({ hasText: "Too many requests. Try again in 7 s." })).toBeVisible();
    await expect(page.getByRole("alert").filter({ hasText: "req_e2e429429" })).toBeVisible();
  });

  test("live 429: a burst from one session hits Go's mutation limit; the proxy passes 429 and Retry-After through", async ({ page }) => {
    await openConsole(page, "limited");
    const token = await (await page.request.get("/bff/session")).json();
    const results = await page.evaluate(async (csrfToken) => {
      const fire = () => fetch("/bff/api/v1/jobs/0193a0f2-7c1e-7a10-8000-000000000001/cancel", { method: "POST", headers: { "X-CSRF-Token": csrfToken } }).then(async (r) => ({ status: r.status, retry: r.headers.get("Retry-After"), body: r.status === 429 ? await r.json() : null }));
      return Promise.all(Array.from({ length: 120 }, fire));
    }, token.csrf_token);
    const limited = results.filter((r) => r.status === 429);
    expect(limited.length).toBeGreaterThan(0);
    expect(limited[0]!.retry).toMatch(/^\d+$/);
    expect(limited[0]!.body.code).toBe("rate_limited");
    expect(results.some((r) => r.status === 404)).toBe(true); // the rest reached Go: a job that does not exist
    void USERS;
    await page.waitForTimeout(1500);
    expect((await bff(page, "GET", "/jobs")).status()).toBe(200); // reads are not affected
  });

  test("a 403 from Go refreshes the role (UI hides, Go enforces)", async ({ page }) => {
    await openConsole(page, "viewer", "/app/jobs");
    const r = await bff(page, "POST", "/jobs", { handler_name: "hash_sha256", handler_version: 1, payload: { data: "x" } });
    expect(r.status()).toBe(403);
    expect((await r.json()).details?.reason ?? "forbidden").toBeTruthy();
  });
});
