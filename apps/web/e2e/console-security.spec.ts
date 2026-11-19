import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { bff, csrf, openConsole } from "./console";
import { KEY_FILE, WEB_DIR } from "./stack/stack";

test.describe("CSRF on mutating BFF routes (ADR-0006 section 3)", () => {
  const body = { handler_name: "hash_sha256", handler_version: 1, payload: { data: "x" } };

  test("rejects a missing Origin, a foreign Origin, a missing token and a token of another session", async ({ page, browser }) => {
    await openConsole(page, "operator");
    const token = await csrf(page);
    const post = (headers: Record<string, string>) => page.request.fetch("/bff/api/v1/jobs", { method: "POST", headers: { "Content-Type": "application/json", ...headers }, data: JSON.stringify(body) });
    const cases: Record<string, string>[] = [{}, { "X-CSRF-Token": token }, { Origin: "http://localhost:3100" }, { Origin: "https://evil.example", "X-CSRF-Token": token }, { Origin: "http://localhost:3100", "X-CSRF-Token": "AAAA" }];
    for (const h of cases) {
      const r = await post(h);
      expect(r.status(), JSON.stringify(h)).toBe(403);
      expect((await r.json()).code).toBe("csrf_failed");
      expect((await r.json()).request_id).toMatch(/^req_/);
    }
    const other = await browser.newContext();
    const p2 = await other.newPage();
    await openConsole(p2, "operator");
    const r = await post({ Origin: "http://localhost:3100", "X-CSRF-Token": await csrf(p2) });
    expect(r.status()).toBe(403);
    await other.close();
    // A valid request still works (so the above are not blanket failures).
    expect((await bff(page, "POST", "/jobs", body, { "Idempotency-Key": `csrf-ok-${Date.now()}` })).status()).toBe(201);
  });

  test("logout is CSRF-protected too, and GET routes cause no side effects", async ({ page }) => {
    await openConsole(page, "operator");
    expect((await page.request.post("/bff/logout")).status()).toBe(403);
    expect((await page.request.get("/bff/logout")).status()).toBe(405);
    expect((await page.request.get("/bff/session")).status()).toBe(200);
  });

  test("the proxy never exposes session operations or reaches arbitrary paths", async ({ page }) => {
    await openConsole(page, "operator");
    for (const p of ["/sessions", "/sessions/current", "/stream"]) expect((await bff(page, "GET", p)).status(), p).toBe(404);
    expect((await bff(page, "POST", "/sessions", { github_login: "x" })).status()).toBe(404);
    expect((await page.request.get("/bff/api/v1/../../health/live")).status()).not.toBe(200);
  });
});

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((f) => { const p = join(dir, f); return statSync(p).isDirectory() ? files(p) : [p]; });
}

test.describe("secrets stay out of the client (FR-SEC)", () => {
  test("the client bundle and served pages contain no private key material, cxs_/cxt_/cxw_ secrets or OAuth secrets", async ({ page }) => {
    const jwk = JSON.parse(readFileSync(KEY_FILE, "utf8")) as { d: string; x: string };
    const forbidden = [jwk.d, "cxs_", "cxt_cx", "cxw_cx", "BEGIN PRIVATE KEY", "e2e-client-secret-not-real", "CLUSTERX_WEB_SESSION_KEY", "CLUSTERX_BFF_KEY_FILE", "bff-private"];
    const client = files(join(WEB_DIR, ".next/static")).filter((f) => /\.(js|css|html|json|map)$/.test(f));
    expect(client.length).toBeGreaterThan(5);
    for (const f of client) {
      const text = readFileSync(f, "utf8");
      for (const needle of forbidden) expect(text.includes(needle), `${needle} in ${f}`).toBe(false);
    }
    // The jose signing code is server-only: no client chunk imports EdDSA key import/sign.
    expect(client.some((f) => /importJWK|SignJWT/.test(readFileSync(f, "utf8")))).toBe(false);
    // Rendered pages (login, console) carry the CSRF token but no secrets.
    await openConsole(page, "operator");
    for (const path of ["/login", "/app", "/app/settings/tokens"]) {
      const html = await (await page.request.get(path)).text();
      for (const needle of forbidden) expect(html.includes(needle), `${needle} in ${path}`).toBe(false);
    }
  });
});
