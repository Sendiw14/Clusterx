import { expect, test, type Page, type Request } from "@playwright/test";
import { runAxe, watchConsole } from "./helpers";

/**
 * AC-14 isolation harness (E2E-DEMO-20/21/22/23/24/25/26): during a full tour of /demo every request is a same-origin
 * static asset, nothing touches /api/ or /bff/, no WebSocket opens, no cookie is set and no Authorization header is sent.
 */
async function tour(page: Page) {
  await page.goto("/demo", { waitUntil: "networkidle" });
  const screen = page.getByRole("group", { name: "Demo screen" });
  for (const view of ["operator", "viewer"]) {
    await page.getByRole("radio", { name: `View as ${view}` }).click();
    for (const name of ["Overview", "Jobs", "Workers", "Recovery replay", "Config"]) {
      await screen.getByRole("button", { name }).click();
      await page.waitForLoadState("networkidle");
    }
    await screen.getByRole("button", { name: "Jobs" }).click();
    await page.getByRole("button", { name: "demo-job-0002" }).first().click();
    await expect(page.getByRole("heading", { name: "Attempts" })).toBeVisible();
    await page.getByRole("button", { name: "Back to jobs" }).click();
  }
}

test("network trap: same-origin static only, no /api, no /bff, no sockets, no cookies, no auth header", async ({ page, context }) => {
  const origin = new URL(page.context().browser() ? "http://localhost:3100" : "http://localhost:3100").origin;
  const bad: string[] = [];
  const seen: string[] = [];
  const check = (r: Request) => {
    const u = new URL(r.url());
    seen.push(`${r.method()} ${u.pathname}`);
    if (u.origin !== origin) bad.push(`cross-origin ${r.url()}`);
    if (/^\/(api|bff)\//.test(u.pathname)) bad.push(`forbidden path ${u.pathname}`);
    if (r.method() !== "GET" && r.method() !== "HEAD") bad.push(`non-GET ${r.method()} ${u.pathname}`);
    if (r.headers()["authorization"]) bad.push(`authorization header on ${u.pathname}`);
    if (r.resourceType() === "websocket" || r.resourceType() === "eventsource") bad.push(`${r.resourceType()} ${u.pathname}`);
  };
  page.on("request", check);
  page.on("websocket", (ws) => bad.push(`websocket ${ws.url()}`));
  const errors = watchConsole(page);
  await tour(page);
  expect(seen.length).toBeGreaterThan(5);
  expect(bad, bad.join("\n")).toEqual([]);
  expect(await context.cookies(), "no cookies set").toEqual([]);
  expect(await page.evaluate(() => document.cookie)).toBe("");
  expect(errors).toEqual([]);
});

test("banner on every demo screen, nothing says Live, no mutation controls, synthetic IDs only", async ({ page }) => {
  await page.goto("/demo", { waitUntil: "networkidle" });
  const banner = page.getByRole("region", { name: "Demo notice" });
  const screen = page.getByRole("group", { name: "Demo screen" });
  for (const name of ["Overview", "Jobs", "Workers", "Recovery replay", "Config"]) {
    await screen.getByRole("button", { name }).click();
    await expect(banner).toBeVisible();
    await expect(banner).toContainText("not live");
    await expect(banner.getByRole("button")).toHaveCount(0); // not dismissible
    const texts = await page.locator("body *").evaluateAll((els) => els.filter((e) => e.children.length === 0).map((e) => (e.textContent ?? "").trim()));
    expect(texts.filter((t) => /^(live|connected)$/i.test(t))).toEqual([]);
    await expect(page.getByRole("button", { name: /^(cancel|retry|drain|create|revoke|delete|submit)/i })).toHaveCount(0);
    await expect(page.getByRole("link", { name: /^(cancel|retry|drain|create|revoke|submit)/i })).toHaveCount(0);
  }
  await expect(page.getByText("Demo data", { exact: true })).toBeVisible();
  await screen.getByRole("button", { name: "Jobs" }).click();
  const ids = await page.locator("button.mono, a.mono, span.mono").allTextContents();
  const jobIds = ids.filter((t) => /job/.test(t));
  expect(jobIds.length).toBeGreaterThan(3);
  for (const id of jobIds) expect(id).toMatch(/^demo-/);
});

test("viewer view removes payload, result, error and log text from the DOM (DOM canary)", async ({ page }) => {
  await page.goto("/demo", { waitUntil: "networkidle" });
  const screen = page.getByRole("group", { name: "Demo screen" });
  await screen.getByRole("button", { name: "Jobs" }).click();
  await page.getByRole("button", { name: "demo-job-0003" }).first().click();
  await expect(page.getByText("demo://assets/3")).toBeVisible(); // operator sees the payload
  await expect(page.getByText("upstream timed out").first()).toBeVisible();
  await page.getByRole("button", { name: "Back to jobs" }).click();
  await page.getByRole("radio", { name: "View as viewer" }).click();
  await page.getByRole("button", { name: "demo-job-0003" }).first().click();
  await expect(page.getByText("Hidden for viewer role").first()).toBeVisible();
  const html = await page.content();
  for (const canary of ["demo://assets/3", "upstream timed out", "POST demo://upstream", "Handler reported upstream_timeout"]) expect(html, canary).not.toContain(canary);
});

test("axe on each demo screen, both views", async ({ page }) => {
  await page.goto("/demo", { waitUntil: "networkidle" });
  const screen = page.getByRole("group", { name: "Demo screen" });
  for (const view of ["operator", "viewer"]) {
    await page.getByRole("radio", { name: `View as ${view}` }).click();
    for (const name of ["Overview", "Jobs", "Workers", "Recovery replay", "Config"]) {
      await screen.getByRole("button", { name }).click();
      await page.waitForLoadState("networkidle");
      const r = await runAxe(page, `/demo [${view}/${name}]`, 1280);
      expect(r.serious + r.critical, JSON.stringify(r.violations)).toBe(0);
    }
    await screen.getByRole("button", { name: "Jobs" }).click();
    await page.getByRole("button", { name: "demo-job-0002" }).first().click();
    const r = await runAxe(page, `/demo [${view}/job detail]`, 1280);
    expect(r.serious + r.critical, JSON.stringify(r.violations)).toBe(0);
    await page.getByRole("button", { name: "Back to jobs" }).click();
  }
});

test("demo jobs filter and dead-letter preset work client-side", async ({ page }) => {
  await page.goto("/demo", { waitUntil: "networkidle" });
  await page.getByRole("group", { name: "Demo screen" }).getByRole("button", { name: "Jobs" }).click();
  await page.getByRole("button", { name: "Failed (dead-letter)" }).click();
  await expect(page.getByRole("button", { name: "Failed (dead-letter)" })).toHaveAttribute("aria-pressed", "true");
  const rows = page.locator("table.tbl tbody tr");
  await expect(rows).toHaveCount(2);
  await expect(page.getByRole("button", { name: "Remove Status: Failed filter" })).toBeVisible();
  await page.getByLabel("Priority at least").selectOption("9");
  await expect(page.getByRole("heading", { name: "No jobs match these filters" })).toBeVisible();
  await page.getByRole("button", { name: "Clear all filters" }).click();
  await expect(rows.first()).toBeVisible();
});

test("the trap has teeth: the same check flags the console's BFF calls", async ({ page }) => {
  const hits: string[] = [];
  page.on("request", (r) => { if (/^\/(api|bff)\//.test(new URL(r.url()).pathname)) hits.push(new URL(r.url()).pathname); });
  await page.goto("/app/jobs", { waitUntil: "networkidle" });
  expect(hits.some((h) => h.startsWith("/bff/api/v1/jobs"))).toBe(true);
});

test("recovery replay: keyboard Step reaches the scripted late-report rejection, Reset returns, nothing network", async ({ page }) => {
  const bad: string[] = [];
  page.on("request", (r) => { const p = new URL(r.url()).pathname; if (/^\/(api|bff)\//.test(p) || r.method() !== "GET") bad.push(p); });
  await page.goto("/demo", { waitUntil: "networkidle" });
  await page.getByRole("group", { name: "Demo screen" }).getByRole("button", { name: "Recovery replay" }).click();
  const step = page.getByRole("button", { name: "Step", exact: true });
  await expect(page.getByText("Press Step to begin.")).toBeVisible();
  await step.focus();
  for (let i = 0; i < 7; i++) await page.keyboard.press("Enter");
  await expect(page.getByText(/Step 7 of 7: Late report from worker A: rejected/)).toBeVisible();
  await expect(page.getByText("Scripted, not a fixture record.")).toBeVisible();
  await expect(step).toBeDisabled();
  await page.getByRole("button", { name: "Reset" }).click();
  await expect(page.getByText("Press Step to begin.")).toBeVisible();
  await page.getByRole("button", { name: "Show all" }).click();
  await expect(page.getByText("Step 7 of 7", { exact: false }).first()).toBeVisible();
  expect(bad).toEqual([]);
});

test("failed job shows its retry lineage and the link works both ways", async ({ page }) => {
  await page.goto("/demo", { waitUntil: "networkidle" });
  await page.getByRole("group", { name: "Demo screen" }).getByRole("button", { name: "Jobs" }).click();
  await page.getByRole("button", { name: "demo-job-0003" }).first().click();
  await expect(page.getByText("Retried as")).toBeVisible();
  await page.getByRole("button", { name: "demo-job-0013" }).first().click();
  await expect(page.getByText("Retried from")).toBeVisible();
  await expect(page.getByRole("button", { name: "demo-job-0003" }).first()).toBeVisible();
});
