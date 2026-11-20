import { request as httpRequest } from "node:http";
import { execFileSync } from "node:child_process";
import { expect, test } from "@playwright/test";
import { openConsole } from "./console";
import { HOST, PORT, cookieOf, mutating, raw } from "./raw";

/** RV6-01 (positive allowlist, encoded separators) and RV6-02 (body cap enforced while reading), from REVIEW-M6-RV-01. */
test.describe("RV6-01: the proxy is a positive allowlist and encoded separators never reach Go", () => {
  const encoded = [
    "/bff/api/v1/x%2F..%2F..%2F..%2Fhealth%2Fready",
    "/bff/api/v1/x%2F..%2Fstream",
    "/bff/api/v1/x%2F..%2Fsessions%2Fcurrent",
    "/bff/api/v1/jobs%2F..%2Fme",
    "/bff/api/v1/jobs%2f..%2fme",
    "/bff/api/v1/jobs%5C..%5Cme",
    "/bff/api/v1/jobs/%2e%2e/sessions/current",
    "/bff/api/v1/jobs/%2E%2E/%2e%2e/health/ready",
    "/bff/api/v1/jobs%3Fx=1",
    "/bff/api/v1/jobs%23frag",
    "/bff/api/v1/%2e%2e/health/ready",
    "/bff/api/v1/jobs/..%2fsessions%2fcurrent",
  ];

  test("every encoded probe is 404 not_found_route, and the session survives", async ({ page }) => {
    await openConsole(page, "operator");
    const cookie = await cookieOf(page);
    for (const p of encoded) {
      const r = await raw("GET", p, { Cookie: cookie });
      expect(r.status, p).toBe(404);
      // Dot segments the HTTP stack itself normalises can land outside /bff/api/v1: that is Next's own HTML 404, still not Go.
      if (r.body.startsWith("{")) expect(JSON.parse(r.body).code, p).toBe("not_found_route");
    }
    // The reviewer's destructive probe: DELETE of sessions/current through an encoded slash. It must not log the user out.
    const d = await raw("DELETE", "/bff/api/v1/x%2F..%2Fsessions%2Fcurrent", await mutating(page));
    expect(d.status).toBe(404);
    expect((await raw("GET", "/bff/session", { Cookie: cookie })).status).toBe(200);
  });

  test("a path that is not an OpenAPI route the console uses is 404; the routes it does use still work", async ({ page }) => {
    await openConsole(page, "operator");
    const cookie = await cookieOf(page);
    for (const p of ["/bff/api/v1/sessions/current", "/bff/api/v1/stream", "/bff/api/v1/unknown", "/bff/api/v1/jobs/a/b/c", "/bff/api/v1/health/ready"]) {
      expect((await raw("GET", p, { Cookie: cookie })).status, p).toBe(404);
    }
    for (const p of ["/bff/api/v1/jobs?limit=1", "/bff/api/v1/me", "/bff/api/v1/handlers", "/bff/api/v1/config", "/bff/api/v1/workers?limit=5", "/bff/api/v1/metrics/overview"]) {
      expect((await raw("GET", p, { Cookie: cookie })).status, p).toBe(200);
    }
  });
});

test.describe("RV6-02: the 1 MiB body cap is enforced while reading", () => {
  // Both big-body tests measure the one web server's RSS; run in parallel, one test's 256 MiB buffers show up in the other's peak (seen as a 103 to 127 MiB flake against the 96 MiB limit).
  test.describe.configure({ mode: "serial" });
  const rssKiB = () => {
    const pid = execFileSync("lsof", ["-t", "-nP", `-iTCP:${PORT}`, "-sTCP:LISTEN"], { encoding: "utf8" }).trim().split("\n")[0];
    return Number(execFileSync("ps", ["-o", "rss=", "-p", pid!], { encoding: "utf8" }).trim());
  };

  /** Streams `total` bytes in 1 MiB writes with backpressure; stops writing once the server has answered. */
  function stream(path: string, headers: Record<string, string>, total: number): Promise<{ status: number; sent: number }> {
    return new Promise((res) => {
      let sent = 0;
      let status = 0;
      let done = false;
      const finish = () => { if (!done) { done = true; res({ status, sent }); } };
      const r = httpRequest({ host: HOST, port: PORT, method: "POST", path, headers }, (m) => { status = m.statusCode ?? 0; m.resume(); m.on("end", finish); });
      r.on("error", finish);
      const chunk = Buffer.alloc(1024 * 1024, 0x61);
      const pump = () => {
        while (!done && status === 0 && sent < total) {
          sent += chunk.length;
          if (!r.write(chunk)) return void r.once("drain", pump);
        }
        if (!done && status === 0) r.end();
      };
      r.on("response", () => r.destroy());
      pump();
      setTimeout(finish, 20_000);
    });
  }

  test("Content-Length over the cap is refused before anything is read (no memory growth)", async ({ page }) => {
    await openConsole(page, "viewer");
    const h = { ...(await mutating(page)), "Content-Type": "application/json" };
    const before = rssKiB();
    let peak = before;
    const t = setInterval(() => { try { peak = Math.max(peak, rssKiB()); } catch { /* process gone */ } }, 25);
    const big = Buffer.alloc(256 * 1024 * 1024, 0x61);
    const r = await raw("POST", "/bff/api/v1/jobs", { ...h, "Content-Length": String(big.length) }, big).catch((e: NodeJS.ErrnoException) => ({ status: -1, body: e.code ?? "", headers: {} }));
    clearInterval(t);
    // Either the 413 arrives, or the server closed the socket while we were still writing: both mean it did not buffer.
    expect([413, -1]).toContain(r.status);
    expect((peak - before) / 1024, `rss grew ${((peak - before) / 1024).toFixed(0)} MiB`).toBeLessThan(96);
  });

  test("a chunked body with no Content-Length is cut off at the cap", async ({ page }) => {
    await openConsole(page, "viewer");
    const h = { ...(await mutating(page)), "Content-Type": "application/json", "Transfer-Encoding": "chunked" };
    const before = rssKiB();
    let peak = before;
    const t = setInterval(() => { try { peak = Math.max(peak, rssKiB()); } catch { /* process gone */ } }, 25);
    const r = await stream("/bff/api/v1/jobs", h, 256 * 1024 * 1024);
    clearInterval(t);
    expect(r.status).toBe(413);
    expect(r.sent / (1024 * 1024), "MiB written before the server answered").toBeLessThan(64);
    expect((peak - before) / 1024, `rss grew ${((peak - before) / 1024).toFixed(0)} MiB`).toBeLessThan(96);
  });

  test("a body under the cap still passes", async ({ page }) => {
    await openConsole(page, "operator");
    const h = { ...(await mutating(page)), "Content-Type": "application/json", "Idempotency-Key": `cap-ok-${Date.now()}` };
    const r = await raw("POST", "/bff/api/v1/jobs", h, Buffer.from(JSON.stringify({ handler_name: "hash_sha256", handler_version: 1, payload: { data: "x" } })));
    expect(r.status).toBe(201);
  });
});

