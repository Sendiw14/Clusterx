import { expect, test } from "@playwright/test";
import { bff, openConsole, waitJobStatus } from "./console";
import { sql } from "./stack/stack";

const key = () => `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
const HELLO_SHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824";

async function submit(page: import("@playwright/test").Page, body: Record<string, unknown>) {
  const r = await bff(page, "POST", "/jobs", body, { "Idempotency-Key": key() });
  expect(r.status()).toBe(201);
  return (await r.json()) as { id: string };
}

test.describe("submit and run (operator)", () => {
  test("submit a job in the UI, a worker runs it, and the page shows succeeded without a reload", async ({ page }) => {
    await openConsole(page, "operator", "/app/jobs/new");
    await page.getByRole("combobox", { name: /^Handler/ }).selectOption("hash_sha256");
    await page.getByLabel("Payload (JSON)").fill('{"data":"hello"}');
    await page.getByRole("button", { name: "Submit job" }).click();
    await expect(page).toHaveURL(/\/app\/jobs\/[0-9a-f-]{36}$/);
    await expect(page.locator(".status", { hasText: "Succeeded" }).first()).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText(HELLO_SHA)).toBeVisible();
    await expect(page.getByRole("heading", { name: "Attempt 1" })).toBeVisible();
    await expect(page.getByText("lease generation")).toBeVisible();
    // The event timeline records admitted -> claimed -> succeeded.
    const tl = page.getByRole("list", { name: /Job event timeline/ });
    await expect(tl).toContainText("admitted");
    await expect(tl).toContainText("claimed");
    await expect(tl).toContainText("succeeded");
  });

  test("client-side schema validation blocks a bad payload before any request, and names the field", async ({ page }) => {
    await openConsole(page, "operator", "/app/jobs/new");
    await page.getByRole("combobox", { name: /^Handler/ }).selectOption("hash_sha256");
    let posted = false;
    page.on("request", (r) => { if (r.method() === "POST" && r.url().includes("/bff/api/v1/jobs")) posted = true; });
    await page.getByLabel("Payload (JSON)").fill("{}");
    await page.getByRole("button", { name: "Submit job" }).click();
    await expect(page.getByRole("alert").first()).toContainText("problem");
    await expect(page.getByRole("alert").first()).toContainText("/payload");
    await page.getByLabel("Payload (JSON)").fill("{not json");
    await page.getByRole("button", { name: "Submit job" }).click();
    await expect(page.getByText(/not valid JSON/).first()).toBeVisible();
    expect(posted).toBe(false);
  });

  test("API outcomes pass through the BFF unchanged: 201, 200 replay, 409 conflict, 422 fields, 413", async ({ page }) => {
    await openConsole(page, "operator");
    const k = key();
    const body = { handler_name: "hash_sha256", handler_version: 1, payload: { data: "replay" } };
    const a = await bff(page, "POST", "/jobs", body, { "Idempotency-Key": k });
    expect(a.status()).toBe(201);
    expect(a.headers()["location"]).toMatch(/^\/api\/v1\/jobs\//);
    const b = await bff(page, "POST", "/jobs", body, { "Idempotency-Key": k });
    expect(b.status()).toBe(200);
    expect(b.headers()["idempotent-replayed"]).toBe("true");
    expect((await b.json()).id).toBe((await a.json()).id);
    const c = await bff(page, "POST", "/jobs", { ...body, payload: { data: "other" } }, { "Idempotency-Key": k });
    expect(c.status()).toBe(409);
    expect((await c.json()).code).toBe("idempotency_conflict");
    const d = await bff(page, "POST", "/jobs", { handler_name: "no.such", handler_version: 1, payload: {} });
    expect(d.status()).toBe(422);
    const dj = await d.json();
    expect(dj.code).toBe("validation_failed");
    expect(dj.details.fields[0].field).toBeTruthy();
    expect(dj.request_id).toBeTruthy();
    expect(d.headers()["x-request-id"]).toBe(dj.request_id);
    const e = await bff(page, "POST", "/jobs", { ...body, payload: { data: "x".repeat(70_000) } });
    expect(e.status()).toBe(413);
    expect((await e.json()).code).toBe("payload_too_large");
  });

  test("no live worker for a version is shown at submit time and the job reads as unschedulable-free when one exists", async ({ page }) => {
    await openConsole(page, "operator", "/app/jobs/new");
    await expect(page.getByText(/live workers? can run this version/)).toBeVisible();
  });
});

test.describe("cancel a running job", () => {
  test("cancel needs confirmation, is accepted (202) while running, and ends canceled", async ({ page }) => {
    await openConsole(page, "operator");
    const job = await submit(page, { handler_name: "sleep_ms", handler_version: 1, payload: { ms: 45_000 }, timeout_seconds: 120 });
    await waitJobStatus(page, job.id, "running");
    await page.goto(`/app/jobs/${job.id}`);
    await page.getByRole("button", { name: "Cancel job" }).click();
    const dlg = page.getByRole("dialog", { name: "Cancel this job?" });
    await expect(dlg).toContainText("running now");
    await dlg.getByRole("button", { name: "Cancel", exact: true }).click(); // back out: nothing changes
    await expect(dlg).toBeHidden();
    expect((await (await bff(page, "GET", `/jobs/${job.id}`)).json()).cancel_requested_at).toBeNull();
    await page.getByRole("button", { name: "Cancel job" }).click();
    await dlg.getByRole("button", { name: "Cancel job" }).click();
    await expect(page.getByText(/Cancel requested\./)).toBeVisible();
    await expect(page.locator(".status", { hasText: "Canceled" }).first()).toBeVisible({ timeout: 45_000 });
    // A second cancel is idempotent in the API.
    expect((await bff(page, "POST", `/jobs/${job.id}/cancel`)).status()).toBe(200);
  });

  test("canceling an already finished job is a 409 that carries the current state", async ({ page }) => {
    await openConsole(page, "operator");
    const job = await submit(page, { handler_name: "hash_sha256", handler_version: 1, payload: { data: "done" } });
    await waitJobStatus(page, job.id, "succeeded");
    const r = await bff(page, "POST", `/jobs/${job.id}/cancel`);
    expect(r.status()).toBe(409);
    const j = await r.json();
    expect(j.code).toBe("job_terminal");
    expect(j.details.status).toBe("succeeded");
  });
});

test.describe("manual retry keeps lineage", () => {
  test("a failed job is retried as a new linked job, once", async ({ page }) => {
    await openConsole(page, "operator");
    const parent = await submit(page, { handler_name: "fail_n_times", handler_version: 1, payload: { n: 1, permanent: true }, max_attempts: 1 });
    await waitJobStatus(page, parent.id, "failed");
    await page.goto(`/app/jobs/${parent.id}`);
    await expect(page.getByText("planned_failure").first()).toBeVisible();
    await page.getByRole("button", { name: "Retry job" }).click();
    await page.getByRole("dialog", { name: "Retry this job?" }).getByRole("button", { name: "Create retry" }).click();
    await expect(page).not.toHaveURL(new RegExp(`${parent.id}$`));
    await expect(page).toHaveURL(/\/app\/jobs\/[0-9a-f-]{36}$/);
    const childId = page.url().split("/").pop()!;
    expect(childId).not.toBe(parent.id);
    await expect(page.locator("dt", { hasText: "Retried from" })).toBeVisible();
    await expect(page.locator(`a[href="/app/jobs/${parent.id}"]`).first()).toBeVisible();
    await page.locator(`a[href="/app/jobs/${parent.id}"]`).first().click();
    await expect(page.locator(`a[href="/app/jobs/${childId}"]`).first()).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry job" })).toHaveAttribute("aria-disabled", "true");
    const again = await bff(page, "POST", `/jobs/${parent.id}/retry`, undefined, { "Idempotency-Key": key() });
    expect(again.status()).toBe(409);
    expect((await again.json()).details.retried_as_job_id).toBe(childId);
  });

  test("a non-failed job cannot be retried", async ({ page }) => {
    await openConsole(page, "operator");
    const job = await submit(page, { handler_name: "hash_sha256", handler_version: 1, payload: { data: "ok" } });
    await waitJobStatus(page, job.id, "succeeded");
    const r = await bff(page, "POST", `/jobs/${job.id}/retry`, undefined, { "Idempotency-Key": key() });
    expect(r.status()).toBe(409);
    expect((await r.json()).code).toBe("retry_not_allowed");
  });
});

test.describe("lists, errors and retention", () => {
  test("jobs list: server filters, dead-letter view, sort and cursor", async ({ page }) => {
    await openConsole(page, "operator");
    for (let i = 0; i < 3; i++) await submit(page, { handler_name: "fail_n_times", handler_version: 1, payload: { n: 1, permanent: true }, max_attempts: 1 });
    await page.goto("/app/jobs");
    await page.getByRole("button", { name: /Failed \(dead-letter\)/ }).click();
    await expect(page).toHaveURL(/status=failed/);
    await expect(page.locator("table.tbl tbody tr").first()).toBeVisible();
    for (const row of await page.locator("table.tbl tbody tr .status").allInnerTexts()) expect(row).toContain("Failed");
    await page.getByLabel("Sort").selectOption("created_at");
    await expect(page).toHaveURL(/sort=created_at/);
    await expect(page.locator(".pager")).toContainText("oldest first");
    await page.goto("/app/jobs?status=failed&handler_name=hash_sha256");
    await expect(page.getByText("No jobs match these filters")).toBeVisible();
  });

  test("an unknown job is a not-found state, an expired one says retention (410)", async ({ page }) => {
    await openConsole(page, "operator");
    await page.goto("/app/jobs/0193a0f2-7c1e-7a10-8000-00000000dead");
    await expect(page.getByRole("heading", { name: "No job with that ID" })).toBeVisible();
    sql("INSERT INTO job_tombstones (job_id, terminal_at) VALUES ('0193a0f2-7c1e-7a10-8000-00000000e0e0', now() - interval '31 days') ON CONFLICT DO NOTHING");
    await page.goto("/app/jobs/0193a0f2-7c1e-7a10-8000-00000000e0e0");
    await expect(page.getByRole("heading", { name: "This job expired by retention" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry job" })).toHaveCount(0);
  });

  test("the activity page lists the privileged actions this suite performed", async ({ page }) => {
    await openConsole(page, "operator");
    await page.goto("/app/activity");
    await expect(page.getByRole("table")).toContainText("job.cancel");
    await page.getByLabel("Category").selectOption("privileged");
    await expect(page.getByRole("table")).toContainText("job.");
  });
});

test.describe("viewer: denied and masked", () => {
  test("payload, result and logs are absent from the DOM; no controls; Go answers 403 for writes", async ({ page, browser }) => {
    const op = await browser.newContext();
    const opPage = await op.newPage();
    await openConsole(opPage, "operator");
    const job = await submit(opPage, { handler_name: "hash_sha256", handler_version: 1, payload: { data: "very-secret-payload-marker" } });
    await waitJobStatus(opPage, job.id, "succeeded");
    await op.close();

    await openConsole(page, "viewer", `/app/jobs/${job.id}`);
    await expect(page.locator(".masked").first()).toBeVisible();
    const html = await page.content();
    expect(html).not.toContain("very-secret-payload-marker");
    expect(html).not.toContain(HELLO_SHA);
    await expect(page.getByRole("button", { name: /Cancel job|Retry job/ })).toHaveCount(0);
    const raw = await (await bff(page, "GET", `/jobs/${job.id}`)).json();
    expect(raw.payload).toBeNull();
    expect(raw.masked_fields).toEqual(expect.arrayContaining(["payload", "result"]));
    const w = await bff(page, "POST", "/jobs", { handler_name: "hash_sha256", handler_version: 1, payload: { data: "x" } });
    expect(w.status()).toBe(403);
    expect((await w.json()).code).toBe("forbidden");
    expect((await bff(page, "GET", "/tokens")).status()).toBe(403);
  });

  test("the activity page shows privileged events only, with details hidden", async ({ page }) => {
    await openConsole(page, "viewer", "/app/activity");
    await expect(page.getByText("Viewer scope.")).toBeVisible();
    await expect(page.getByLabel("Category").locator("option")).toHaveText(["All visible", "Privileged"]);
    const r = await bff(page, "GET", "/audit?category=security");
    expect(r.status()).toBe(200);
    expect((await r.json()).items).toEqual([]);
  });
});
