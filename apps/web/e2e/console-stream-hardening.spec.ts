import { connect } from "node:net";
import { expect, test } from "@playwright/test";
import WebSocket from "ws";
import { openConsole } from "./console";
import { HOST, ORIGIN, PORT, cookieOf, raw } from "./raw";

/** RV6-03 (no upstream socket leak) and RV6-08 (slot reserved before the dial), from REVIEW-M6-RV-01. */
test.describe("RV6-03 / RV6-08: the stream relay never leaks upstream sockets and reserves its slot before dialing", () => {
  const key = "dGhlIHNhbXBsZSBub25jZQ==";
  const upgrade = (cookie: string, version = 13) => `GET /bff/stream HTTP/1.1\r\nHost: localhost:${PORT}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: ${key}\r\nSec-WebSocket-Version: ${version}\r\nOrigin: ${ORIGIN}\r\nCookie: ${cookie}\r\n\r\n`;

  /** Valid upgrade whose TCP connection is destroyed the moment it is written (a tab closed while the BFF dials Go). */
  const aborted = (cookie: string) => new Promise<void>((res) => {
    const s = connect(PORT, HOST, () => { s.write(upgrade(cookie)); setImmediate(() => { s.destroy(); res(); }); });
    s.on("error", () => res());
  });
  /** Handshake `ws` rejects after the BFF has already dialed Go (bad version). */
  const malformed = (cookie: string) => new Promise<string>((res) => {
    const s = connect(PORT, HOST, () => s.write(upgrade(cookie, 12)));
    let out = "";
    s.on("data", (d) => (out += d.toString()));
    s.on("close", () => res(out.split("\r\n")[0] ?? ""));
    s.on("error", () => res("error"));
    setTimeout(() => s.destroy(), 3000);
  });

  function socket(cookie: string, holdMs = 1500): Promise<string> {
    return new Promise((res) => {
      const ws = new WebSocket(`ws://localhost:${PORT}/bff/stream`, { headers: { Origin: ORIGIN, Cookie: cookie } });
      let first = "";
      ws.on("unexpected-response", (_q, r) => res(`refused ${r.statusCode}`));
      ws.on("message", (d) => { if (!first) first = d.toString(); });
      ws.on("open", () => setTimeout(() => { res(first.includes('"hello"') ? "open" : `open without hello (${first.slice(0, 40)})`); ws.close(); }, holdMs));
      ws.on("close", (code) => res(`closed ${code}`));
      ws.on("error", (e) => res(`error ${e.message}`));
    });
  }

  test("six aborted and six malformed upgrades, then a working socket", async ({ page }) => {
    await openConsole(page, "viewer");
    const cookie = await cookieOf(page);
    const pause = (ms: number) => new Promise((r) => setTimeout(r, ms));
    await page.goto("about:blank"); // the open console holds one of the session's five slots itself
    await pause(500);
    // Slots are reserved before the dial and freed when the browser socket closes, so give each attempt time to settle.
    for (let i = 0; i < 6; i++) { await aborted(cookie); await pause(250); }
    for (let i = 0; i < 6; i++) { expect(await malformed(cookie)).toMatch(/^HTTP\/1\.1 400/); await pause(250); }
    await pause(500);
    expect(await socket(cookie)).toBe("open");
    // The BFF is still serving HTTP afterwards (an unhandled upstream error would have killed the process).
    expect((await raw("GET", "/bff/session", { Cookie: cookie })).status).toBe(200);
  });

  test("eight concurrent upgrades: five open, the rest are refused by the BFF itself (429)", async ({ page }) => {
    await openConsole(page, "operator");
    const cookie = await cookieOf(page);
    await page.goto("about:blank"); // the open console holds one of the session's five sockets itself
    await new Promise((r) => setTimeout(r, 800));
    const res = await Promise.all(Array.from({ length: 8 }, () => socket(cookie, 2500)));
    expect(res.filter((r) => r === "open"), JSON.stringify(res)).toHaveLength(5);
    expect(res.filter((r) => r === "refused 429"), JSON.stringify(res)).toHaveLength(3);
  });
});

