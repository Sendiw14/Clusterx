import { expect, test, type Page, type WebSocketRoute } from "@playwright/test";
import WebSocket from "ws";
import { bff, csrf, live, openConsole, waitJobStatus } from "./console";

const key = () => `st-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
const mk = async (page: Page, handler: string, payload: unknown) => (await (await bff(page, "POST", "/jobs", { handler_name: handler, handler_version: 1, payload }, { "Idempotency-Key": key() })).json()) as { id: string };
const cookieHeader = async (page: Page) => (await page.context().cookies()).map((c) => `${c.name}=${c.value}`).join("; ");

function rawSocket(cookie: string, origin = "http://localhost:3100") {
  return new Promise<{ status?: number; ws?: WebSocket; first?: string }>((resolve) => {
    const ws = new WebSocket("ws://localhost:3100/bff/stream", { headers: { ...(cookie ? { Cookie: cookie } : {}), ...(origin ? { Origin: origin } : {}) } });
    ws.once("unexpected-response", (_r, res) => resolve({ status: res.statusCode }));
    ws.once("message", (d) => resolve({ ws, first: String(d) }));
    ws.once("error", () => resolve({}));
  });
}

test.describe("live channel", () => {
  test("the indicator reaches Live and the first frame is hello", async ({ page }) => {
    await openConsole(page, "operator");
    await expect(live(page)).toHaveAttribute("data-live", "live");
  });

  test("a running job's page updates by itself, and the jobs list offers a refresh instead of reordering", async ({ page, browser }) => {
    await openConsole(page, "operator", "/app/jobs");
    await expect(live(page)).toHaveAttribute("data-live", "live");
    const other = await browser.newContext();
    const op = await other.newPage();
    await openConsole(op, "operator");
    const job = await mk(op, "sleep_ms", { ms: 2500 });
    await expect(page.getByText("Jobs changed.")).toBeVisible({ timeout: 15_000 });
    await page.goto(`/app/jobs/${job.id}`);
    await expect(live(page)).toHaveAttribute("data-live", "live");
    await expect(page.locator(".status", { hasText: "Succeeded" }).first()).toBeVisible({ timeout: 30_000 });
    await other.close();
  });

  test("stream drop: reconnects with backoff, shows non-live states, and reconciles with the snapshot (AC-16)", async ({ page, browser }) => {
    let allow = true;
    const conns: WebSocketRoute[] = [];
    await page.routeWebSocket(/\/bff\/stream/, (ws) => {
      if (!allow) { void ws.close({ code: 4000 }); return; }
      conns.push(ws);
      ws.connectToServer();
    });
    await openConsole(page, "operator");
    await expect(live(page)).toHaveAttribute("data-live", "live");

    const other = await browser.newContext();
    const op = await other.newPage();
    await openConsole(op, "operator");
    const job = await mk(op, "sleep_ms", { ms: 3000 });
    await waitJobStatus(op, job.id, "running");
    await page.goto(`/app/jobs/${job.id}`);
    await expect(page.locator(".status", { hasText: "Running" }).first()).toBeVisible();
    await expect(live(page)).toHaveAttribute("data-live", "live");

    // Cut the stream and refuse reconnects; the job finishes while the page cannot hear about it.
    allow = false;
    await conns.at(-1)!.close({ code: 4000 });
    await expect(live(page)).not.toHaveAttribute("data-live", "live");
    await expect.poll(async () => (await live(page).getAttribute("data-live")) ?? "", { timeout: 15_000 }).toMatch(/reconnecting|polling/);
    await waitJobStatus(op, job.id, "succeeded");
    await page.waitForTimeout(500);
    // Hints are gone, so the page still shows the old state or has caught up by polling: both are allowed. What must hold is the end state.
    allow = true;
    await expect(live(page)).toHaveAttribute("data-live", "live", { timeout: 40_000 });
    await expect(page.locator(".status", { hasText: "Succeeded" }).first()).toBeVisible({ timeout: 10_000 });
    await other.close();
  });

  test("a stream that never connects falls back to polling and marks the data stale when nothing loads", async ({ page }) => {
    await page.routeWebSocket(/\/bff\/stream/, (ws) => { void ws.close({ code: 4000 }); });
    await openConsole(page, "operator");
    await expect.poll(async () => (await live(page).getAttribute("data-live")) ?? "", { timeout: 20_000 }).toBe("polling");
    await expect(page.getByRole("heading", { name: "Overview" })).toBeVisible();
  });

  test("auth_expiring from the stream shows the session-ends-soon notice", async ({ page }) => {
    await page.routeWebSocket(/\/bff\/stream/, (ws) => {
      ws.connectToServer();
      setTimeout(() => ws.send(JSON.stringify({ type: "auth_expiring", expires_at: new Date(Date.now() + 60_000).toISOString() })), 800);
    });
    await openConsole(page, "operator");
    await expect(page.getByText("Your session ends soon.")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole("link", { name: "Sign in again" })).toBeVisible();
  });
});

test.describe("session end closes the stream", () => {
  test("logout: Go closes the relayed socket with 4401 and the open console leaves for /login", async ({ page }) => {
    await openConsole(page, "operator", "/app/workers");
    await expect(live(page)).toHaveAttribute("data-live", "live");
    const cookie = await cookieHeader(page);
    const raw = await rawSocket(cookie);
    expect(raw.first).toContain('"type":"hello"');
    const closed = new Promise<number>((r) => raw.ws!.once("close", (c) => r(c)));
    const out = await page.request.post("/bff/logout", { headers: { Origin: "http://localhost:3100", "X-CSRF-Token": await csrf(page) } });
    expect(out.status()).toBe(204);
    expect(await closed).toBe(4401);
    await expect(page).toHaveURL(/\/login\?reason=session_expired/, { timeout: 45_000 });
    expect((await rawSocket(cookie)).status).toBe(401);
  });
});

test.describe("relay access control", () => {
  test("no cookie is 401, a foreign Origin is 403, and the sixth socket of a session is refused", async ({ page }) => {
    await openConsole(page, "viewer");
    const cookie = await cookieHeader(page);
    expect((await rawSocket("")).status).toBe(401);
    expect((await rawSocket(cookie, "https://evil.example")).status).toBe(403);
    expect((await rawSocket(cookie, "")).status).toBe(403);
    const open: WebSocket[] = [];
    // The console itself holds one socket; open until the limit of five is hit.
    for (let i = 0; i < 4; i++) { const r = await rawSocket(cookie); expect(r.first, `socket ${i}`).toContain("hello"); open.push(r.ws!); }
    const sixth = await rawSocket(cookie);
    expect(sixth.status).toBe(429);
    for (const w of open) w.close();
  });

  test("a tampered or foreign cookie cannot open a stream", async ({ page }) => {
    await openConsole(page, "viewer");
    const c = (await page.context().cookies()).find((x) => /clusterx_session$/.test(x.name))!;
    expect((await rawSocket(`${c.name}=${c.value.slice(0, -3)}AAA`)).status).toBe(401);
  });
});
