// @vitest-environment node
import { createServer as createHttp, type Server } from "node:http";
import { connect, createServer as createTcp, type Socket } from "node:net";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { exportJWK, generateKeyPair } from "jose";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { resetConfigForTests, loadConfig } from "@/server/config";
import { sealSession } from "@/server/cookies";
import { MAX_SOCKETS_PER_SESSION, MAX_SOCKETS_PER_USER, MAX_SOCKETS_TOTAL, attachStreamRelay, reserveSlot, slotsInUse } from "@/server/stream-relay";

describe("reserveSlot (RV6-08)", () => {
  it("reserves synchronously, so concurrent upgrades cannot pass a check-then-act race", () => {
    const got = Array.from({ length: 8 }, () => reserveSlot("sid-a", "u-a"));
    expect(got.filter(Boolean)).toHaveLength(MAX_SOCKETS_PER_SESSION);
    got.forEach((r) => r?.());
  });
  it("release is idempotent and frees exactly one slot", () => {
    const base = slotsInUse();
    const r = reserveSlot("sid-b", "u-b")!;
    expect(slotsInUse()).toBe(base + 1);
    r();
    r();
    expect(slotsInUse()).toBe(base);
  });
  it("has a per-process ceiling across sessions", () => {
    const held: (() => void)[] = [];
    for (let i = 0; i < MAX_SOCKETS_TOTAL * 2; i++) {
      const n = Math.floor(i / MAX_SOCKETS_PER_SESSION);
      const r = reserveSlot(`sid-many-${n}`, `u-many-${n}`);
      if (r) held.push(r);
    }
    expect(held).toHaveLength(MAX_SOCKETS_TOTAL - slotsInUse() + held.length);
    expect(slotsInUse()).toBe(MAX_SOCKETS_TOTAL);
    expect(reserveSlot("sid-extra", "u-extra")).toBeNull();
    held.forEach((r) => r());
    expect(slotsInUse()).toBe(0);
  });
});

describe("per-user cap across sessions (RV6B-05)", () => {
  it("one user with many sessions cannot take more than MAX_SOCKETS_PER_USER, and other users are unaffected", () => {
    const held: (() => void)[] = [];
    for (let i = 0; i < 10 * MAX_SOCKETS_PER_USER; i++) {
      const r = reserveSlot(`sid-user-${i}`, "u-greedy");
      if (r) held.push(r);
    }
    expect(held).toHaveLength(MAX_SOCKETS_PER_USER);
    expect(MAX_SOCKETS_PER_USER).toBeLessThan(MAX_SOCKETS_TOTAL);
    const other = reserveSlot("sid-other", "u-other");
    expect(other).not.toBeNull();
    held[0]!();
    const again = reserveSlot("sid-user-again", "u-greedy");
    expect(again, "a released slot is reusable").not.toBeNull();
    other?.();
    again?.();
    held.slice(1).forEach((r) => r());
    expect(slotsInUse()).toBe(0);
  });
});

describe("half-closed client (RV6B-05)", () => {
  let web: Server;
  let hung: ReturnType<typeof createTcp>;
  const held: Socket[] = [];
  let origin = "";
  let cookie = "";

  beforeAll(async () => {
    const kp = await generateKeyPair("EdDSA", { extractable: true });
    const keyFile = join(mkdtempSync(join(tmpdir(), "cx-relay-")), "key.jwk.json");
    writeFileSync(keyFile, JSON.stringify({ ...(await exportJWK(kp.privateKey)), kid: "t" }), { mode: 0o600 });
    // Stand-in for Go that accepts the TCP connection and never answers the WebSocket handshake.
    hung = createTcp((s) => { held.push(s); s.on("error", () => {}); });
    await new Promise<void>((r) => hung.listen(0, "127.0.0.1", r));
    web = createHttp((_q, res) => res.end("ok"));
    attachStreamRelay(web, (_q, sock) => sock.destroy());
    await new Promise<void>((r) => web.listen(0, "127.0.0.1", r));
    origin = `http://localhost:${(web.address() as { port: number }).port}`;
    Object.assign(process.env, { CLUSTERX_ENV: "test", CLUSTERX_APP_ORIGIN: origin, CLUSTERX_API_URL: `http://127.0.0.1:${(hung.address() as { port: number }).port}`, CLUSTERX_BFF_KEY_FILE: keyFile, CLUSTERX_WEB_SESSION_KEY: Buffer.alloc(32, 5).toString("base64"), CLUSTERX_GITHUB_CLIENT_ID: "i", CLUSTERX_GITHUB_CLIENT_SECRET: "s" });
    resetConfigForTests();
    const c = loadConfig();
    if (!c.ok) throw new Error(c.problems.join());
    cookie = sealSession(c.config, { sid: "sid-half", uid: "u-half", login: "x", role: "viewer", exp: Math.floor(Date.now() / 1000) + 3600 }).split(";")[0]!;
  });
  afterAll(() => {
    web.close();
    hung.close();
    held.forEach((s) => s.destroy());
  });

  it("releases its slot and the upstream dial promptly when the client sends FIN after its upgrade request", async () => {
    const base = slotsInUse();
    const s = connect((web.address() as { port: number }).port, "127.0.0.1", () => {
      s.end(`GET /bff/stream HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\nOrigin: ${origin}\r\nCookie: ${cookie}\r\n\r\n`);
    });
    s.on("error", () => {});
    await new Promise((r) => setTimeout(r, 150)); // long enough for the relay to have reserved the slot and dialled
    await expect.poll(slotsInUse, { timeout: 1500, interval: 50, message: "the 10 s handshake timeout must not be what frees it" }).toBe(base);
    await expect.poll(() => held.every((h) => h.destroyed || h.readableEnded || h.closed), { timeout: 1500, interval: 50, message: "upstream socket released" }).toBe(true);
  });
});
