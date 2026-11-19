import { expect, test } from "@playwright/test";
import { bff, openConsole } from "./console";

test.describe("workers", () => {
  test("list shows live workers and their detail; drain asks first and then the worker reads Draining", async ({ page }) => {
    await openConsole(page, "operator", "/app/workers");
    const row = page.locator("table.tbl tbody tr", { hasText: "e2e-worker-drain" });
    await expect(row).toContainText("Online");
    await expect(page.locator("table.tbl tbody tr", { hasText: "e2e-worker-1" })).toContainText("Online");
    await row.getByRole("link", { name: "e2e-worker-drain" }).click();
    await expect(page.getByRole("heading", { name: "Worker", exact: true }).first()).toBeVisible();
    await expect(page.getByText("sleep_ms@1")).toBeVisible();

    await page.getByRole("button", { name: "Drain worker" }).click();
    const dlg = page.getByRole("dialog", { name: "Drain this worker?" });
    await dlg.getByLabel("Grace period (seconds)").fill("5000");
    await expect(dlg.getByText("Enter a whole number from 0 to 3600.")).toBeVisible();
    await dlg.getByLabel("Grace period (seconds)").fill("30");
    await dlg.getByRole("button", { name: "Drain worker" }).click();
    // An idle drained worker may deregister within a second or two (SDK drain), so either state proves the drain took effect.
    await expect(page.locator(".status", { hasText: /Draining|Deregistered/ }).first()).toBeVisible();
    await expect(page.getByRole("button", { name: "Drain worker" })).toHaveCount(0);

    // Draining again is idempotent in the API (202, original deadline kept), or 409 worker_deregistered once the idle worker has left.
    const id = page.url().split("/").pop()!;
    const again = await bff(page, "POST", `/workers/${id}/drain`, { grace_seconds: 600 });
    expect([202, 409]).toContain(again.status());
    if (again.status() === 409) expect((await again.json()).code).toBe("worker_deregistered");
    // A drained worker takes no new claims: a job still runs on the other worker.
    const job = await (await bff(page, "POST", "/jobs", { handler_name: "hash_sha256", handler_version: 1, payload: { data: "after-drain" } }, { "Idempotency-Key": `drain-${Date.now()}` })).json();
    await expect.poll(async () => (await (await bff(page, "GET", `/jobs/${job.id}`)).json()).status, { timeout: 30_000 }).toBe("succeeded");
  });

  test("viewer sees workers with the hostname masked and no drain control", async ({ page }) => {
    await openConsole(page, "viewer", "/app/workers");
    await page.locator("table.tbl tbody tr", { hasText: "e2e-worker-1" }).getByRole("link").click();
    await expect(page.locator(".masked").first()).toBeVisible();
    await expect(page.getByRole("button", { name: "Drain worker" })).toHaveCount(0);
    await expect(page.getByText("Draining needs the operator role")).toBeVisible();
    expect((await bff(page, "POST", `/workers/${page.url().split("/").pop()}/drain`, {})).status()).toBe(403);
  });

  test("an unknown worker is a not-found state", async ({ page }) => {
    await openConsole(page, "operator", "/app/workers/0193a0f2-7c1e-7a10-8000-00000000beef");
    await expect(page.getByRole("heading", { name: "No worker with that ID" })).toBeVisible();
  });
});
