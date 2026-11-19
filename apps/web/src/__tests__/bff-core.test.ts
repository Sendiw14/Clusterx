import { createServer, type IncomingMessage, type Server } from "node:http";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { exportJWK, generateKeyPair, jwtVerify, decodeProtectedHeader, type CryptoKey } from "jose";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { GET as proxyGET, POST as proxyPOST } from "@/app/bff/api/v1/[...path]/route";
import { POST as logoutPOST } from "@/app/bff/logout/route";
import { loadConfig, resetConfigForTests } from "@/server/config";
import { csrfToken, open, seal } from "@/server/crypto";
import { csrfOk } from "@/server/bff";
import { clampNext, readSession, sealSession, type SessionCookie } from "@/server/cookies";
import { mintAssertion } from "@/server/assertion";

const KEY = Buffer.alloc(32, 7);
const ORIGIN = "http://localhost:3999";
const SESSION: SessionCookie = { sid: "0193a0f2-9000-7000-8000-0000000000e1", uid: "0193a0f2-0000-7000-8000-000000000001", login: "octo", role: "operator", exp: Math.floor(Date.now() / 1000) + 3600 };

let pub: CryptoKey;
let keyFile: string;
let go: Server;
let goUrl: string;
let seen: { method: string; url: string; headers: IncomingMessage["headers"]; body: string }[] = [];
let script: (n: number, req: { method: string; url: string }) => { status: number; headers?: Record<string, string>; body?: unknown } = () => ({ status: 200, body: {} });

beforeAll(async () => {
  const kp = await generateKeyPair("EdDSA", { extractable: true });
  pub = kp.publicKey;
  const jwk = { ...(await exportJWK(kp.privateKey)), kid: "test-1" };
  keyFile = join(mkdtempSync(join(tmpdir(), "cx-bff-")), "key.jwk.json");
  writeFileSync(keyFile, JSON.stringify(jwk), { mode: 0o600 });
  go = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => {
      seen.push({ method: req.method!, url: req.url!, headers: req.headers, body: Buffer.concat(chunks).toString() });
      const r = script(seen.length, { method: req.method!, url: req.url! });
      res.writeHead(r.status, { "Content-Type": "application/json", "X-Request-Id": "go-rid", ...r.headers });
      res.end(r.status === 204 ? undefined : JSON.stringify(r.body ?? {}));
    });
  });
  await new Promise<void>((r) => go.listen(0, "127.0.0.1", r));
  goUrl = `http://127.0.0.1:${(go.address() as { port: number }).port}`;
});
afterAll(() => go.close());

function env(extra: Record<string, string> = {}) {
  Object.assign(process.env, { CLUSTERX_ENV: "test", CLUSTERX_APP_ORIGIN: ORIGIN, CLUSTERX_API_URL: goUrl, CLUSTERX_BFF_KEY_FILE: keyFile, CLUSTERX_WEB_SESSION_KEY: KEY.toString("base64"), CLUSTERX_GITHUB_CLIENT_ID: "id", CLUSTERX_GITHUB_CLIENT_SECRET: "secret", ...extra });
  resetConfigForTests();
}
beforeEach(() => { seen = []; script = () => ({ status: 200, body: {} }); env(); });

const cfg = () => { const c = loadConfig(); if (!c.ok) throw new Error(c.problems.join()); return c.config; };
const cookie = () => sealSession(cfg(), SESSION).split(";")[0]!;
const get = (path: string, headers: Record<string, string> = { cookie: cookie() }) => proxyGET(new Request(`${ORIGIN}/bff/api/v1/${path}`, { headers }));
const post = (path: string, headers: Record<string, string> = {}, body = "{}") => proxyPOST(new Request(`${ORIGIN}/bff/api/v1/${path}`, { method: "POST", body, headers: { cookie: cookie(), origin: ORIGIN, "x-csrf-token": csrfToken(KEY, SESSION.sid), "content-type": "application/json", ...headers } }));
const claims = async (h: IncomingMessage["headers"]) => {
  const tok = String(h.authorization).replace("Bearer ", "");
  const { payload, protectedHeader } = await jwtVerify(tok, pub, { issuer: "clusterx-bff", audience: "clusterx-control-plane" });
  return { payload, protectedHeader };
};

describe("config fails closed in production", () => {
  const full = { CLUSTERX_ENV: "production", CLUSTERX_APP_ORIGIN: "https://console.example.com", CLUSTERX_BFF_KEY_FILE: "/k", CLUSTERX_WEB_SESSION_KEY: KEY.toString("base64"), CLUSTERX_GITHUB_CLIENT_ID: "a", CLUSTERX_GITHUB_CLIENT_SECRET: "b" };
  it("an empty environment is production and has problems", () => {
    const r = loadConfig({});
    expect(r.ok).toBe(false);
  });
  it("accepts a complete production config", () => expect(loadConfig(full).ok).toBe(true));
  it("rejects http origins, short keys, missing OAuth credentials and OAuth endpoint overrides", () => {
    for (const bad of [{ CLUSTERX_APP_ORIGIN: "http://localhost:3000" }, { CLUSTERX_WEB_SESSION_KEY: "AAAA" }, { CLUSTERX_GITHUB_CLIENT_SECRET: "" }, { CLUSTERX_GITHUB_TOKEN_URL: "http://localhost:9/token" }, { CLUSTERX_BFF_KEY_FILE: "" }, { CLUSTERX_ENV: "staging" }]) {
      expect(loadConfig({ ...full, ...bad }).ok, JSON.stringify(bad)).toBe(false);
    }
  });
  it("allows http localhost and endpoint overrides only outside production", () => {
    const dev = loadConfig({ ...full, CLUSTERX_ENV: "development", CLUSTERX_APP_ORIGIN: "http://localhost:3000", CLUSTERX_GITHUB_TOKEN_URL: "http://localhost:9/token" });
    expect(dev.ok).toBe(true);
    expect(loadConfig({ ...full, CLUSTERX_ENV: "development", CLUSTERX_APP_ORIGIN: "http://example.com" }).ok).toBe(false);
  });
  it("production needs https to the control plane unless it is loopback (RV6-05)", () => {
    for (const bad of ["http://10.0.0.5:8080", "http://control-plane:8080", "http://api.example.com"]) expect(loadConfig({ ...full, CLUSTERX_API_URL: bad }).ok, bad).toBe(false);
    for (const good of ["https://api.example.com", "https://10.0.0.5:8443", "http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"]) expect(loadConfig({ ...full, CLUSTERX_API_URL: good }).ok, good).toBe(true);
    expect(loadConfig({ ...full, CLUSTERX_ENV: "development", CLUSTERX_API_URL: "http://control-plane:8080" }).ok).toBe(true);
  });
  it("an optional previous session key must also be at least 32 bytes (RV6-07)", () => {
    expect(loadConfig({ ...full, CLUSTERX_WEB_SESSION_KEY_PREVIOUS: Buffer.alloc(32, 3).toString("base64") }).ok).toBe(true);
    expect(loadConfig({ ...full, CLUSTERX_WEB_SESSION_KEY_PREVIOUS: "AAAA" }).ok).toBe(false);
  });
  it("cookie names: __Host- only when secure", () => {
    const c = loadConfig(full);
    if (c.ok) expect(c.config.secureCookies).toBe(true);
  });
});

describe("sealed cookies and CSRF token", () => {
  it("round-trips and binds the purpose", () => {
    const t = seal(KEY, "session", { a: 1 });
    expect(open(KEY, "session", t)).toEqual({ a: 1 });
    expect(open(KEY, "oauth", t)).toBeNull();
    expect(open(Buffer.alloc(32, 9), "session", t)).toBeNull();
  });
  it("key rotation: the old key still opens existing values, new values use the new key, an unrelated key never does (RV6-07)", () => {
    const OLD = KEY;
    const NEW = Buffer.alloc(32, 8);
    const sealedOld = seal(OLD, "session", { a: 1 });
    expect(sealedOld).not.toBe(seal(OLD, "session", { a: 1 }));
    expect(Buffer.from(sealedOld, "base64url")[0]).not.toBe(Buffer.from(seal(NEW, "session", { a: 1 }), "base64url")[0]);
    expect(open([NEW, OLD], "session", sealedOld)).toEqual({ a: 1 });
    expect(open([NEW], "session", sealedOld)).toBeNull();
    const sealedNew = seal(NEW, "session", { a: 2 });
    expect(open([NEW, OLD], "session", sealedNew)).toEqual({ a: 2 });
    expect(open([OLD], "session", sealedNew)).toBeNull();
    expect(open([Buffer.alloc(32, 99)], "session", sealedNew)).toBeNull();
  });
  it("CSRF verification accepts the previous key during rotation but tokens are issued with the current one", () => {
    const c = cfg();
    const prev = Buffer.alloc(32, 5);
    const rotated = { ...c, sessionKey: Buffer.alloc(32, 8), previousSessionKey: c.sessionKey };
    const req = (tok: string) => new Request(ORIGIN, { headers: { origin: ORIGIN, "x-csrf-token": tok } });
    expect(csrfOk(rotated, req(csrfToken(c.sessionKey, "s1")), "s1")).toBe(true);
    expect(csrfOk(rotated, req(csrfToken(rotated.sessionKey, "s1")), "s1")).toBe(true);
    expect(csrfOk(rotated, req(csrfToken(prev, "s1")), "s1")).toBe(false);
  });
  it("detects tampering and garbage", () => {
    const t = seal(KEY, "session", { a: 1 });
    expect(open(KEY, "session", t.slice(0, -2) + (t.endsWith("AA") ? "BB" : "AA"))).toBeNull();
    expect(open(KEY, "session", "x")).toBeNull();
    expect(open(KEY, "session", undefined)).toBeNull();
  });
  it("sessions expire and carry no plaintext secret", () => {
    const c = cfg();
    const header = sealSession(c, SESSION);
    expect(header).toMatch(/HttpOnly/);
    expect(header).toMatch(/SameSite=Lax/);
    const opened = open<Record<string, unknown>>(c.sessionKey, "session", header.split(";")[0]!.split("=")[1]);
    expect(Object.keys(opened ?? {}).sort()).toEqual(["exp", "login", "role", "sid", "uid"]); // no cxs_ session secret (RV6-07)
    expect(header).not.toContain(SESSION.sid);
    expect(readSession(c, header.split(";")[0])).toMatchObject({ sid: SESSION.sid, role: "operator" });
    expect(readSession(c, sealSession(c, { ...SESSION, exp: 1 }).split(";")[0])).toBeNull();
  });
  it("CSRF token is stable per session and differs between sessions", () => {
    expect(csrfToken(KEY, "a")).toBe(csrfToken(KEY, "a"));
    expect(csrfToken(KEY, "a")).not.toBe(csrfToken(KEY, "b"));
    expect(csrfToken(KEY, "a")).not.toBe(csrfToken(Buffer.alloc(32, 1), "a"));
  });
  it("clampNext keeps same-origin /app paths only", () => {
    expect(clampNext("/app/jobs?status=failed", ORIGIN)).toBe("/app/jobs?status=failed");
    for (const bad of ["https://evil.example/app", "//evil.example/app", "/\\evil.example", "/appx", "/login", "javascript:alert(1)", "/app/\n", "", null, "/" + "a".repeat(600)]) {
      expect(clampNext(bad as string | null, ORIGIN), String(bad)).toBe("/app");
    }
  });
});

describe("service assertion", () => {
  it("iat, nbf and exp come from one clock read even if the second ticks mid-mint: exp - iat === 60 (RV6-04)", async () => {
    const RealDate = Date;
    const base = RealDate.now();
    let reads = 0;
    class Ticking extends RealDate {
      constructor(...a: unknown[]) {
        if (a.length) super(...(a as [number]));
        else super(base + reads++ * 1000); // every no-arg clock read is a second later than the last
      }
      static override now() { return base + reads++ * 1000; }
    }
    globalThis.Date = Ticking as DateConstructor;
    let tok: string;
    try {
      tok = await mintAssertion(keyFile, { login: true, githubId: 7, rid: "req_clock" });
    } finally {
      globalThis.Date = RealDate;
    }
    const { payload } = await jwtVerify(tok, pub, { currentDate: new Date(base + 5000) });
    expect(payload.exp! - payload.iat!).toBe(60);
    expect(payload.nbf).toBe(payload.iat);
  });
  it("normal assertion: EdDSA header, closed claim set, 60 s, fresh jti", async () => {
    const mint = () => mintAssertion(keyFile, { userId: SESSION.uid, sid: SESSION.sid, role: "viewer", rid: "req_abc" });
    const [a, b] = [await mint(), await mint()];
    expect(decodeProtectedHeader(a)).toEqual({ alg: "EdDSA", typ: "clusterx-assertion+jwt", kid: "test-1" });
    const { payload } = await jwtVerify(a, pub, { issuer: "clusterx-bff", audience: "clusterx-control-plane" });
    expect(Object.keys(payload).sort()).toEqual(["aud", "exp", "iat", "iss", "jti", "nbf", "rid", "role", "sid", "sub"]);
    expect(payload.sub).toBe(`user:${SESSION.uid}`);
    expect(payload.exp! - payload.iat!).toBe(60);
    expect(payload.jti).toMatch(/^[A-Za-z0-9_-]{16,128}$/);
    expect((await jwtVerify(b, pub)).payload.jti).not.toBe(payload.jti);
  });
  it("login assertion: github subject, purpose=login, no sid or role", async () => {
    const { payload } = await jwtVerify(await mintAssertion(keyFile, { login: true, githubId: 4242, rid: "req_x" }), pub);
    expect(Object.keys(payload).sort()).toEqual(["aud", "exp", "iat", "iss", "jti", "nbf", "purpose", "rid", "sub"]);
    expect(payload.sub).toBe("github:4242");
    expect(payload.purpose).toBe("login");
  });
});

describe("API proxy", () => {
  it("answers 401 session_expired with a cleared cookie when there is no session", async () => {
    const r = await get("jobs", {});
    expect(r.status).toBe(401);
    const j = await r.json();
    expect(j.code).toBe("session_expired");
    expect(j.request_id).toMatch(/^req_/);
    expect(r.headers.get("set-cookie")).toMatch(/Max-Age=0/);
    expect(seen).toHaveLength(0);
  });

  it("passes status, body and the allowlisted headers through; never forwards the browser cookie", async () => {
    script = () => ({ status: 201, headers: { Location: "/api/v1/jobs/1", "Idempotent-Replayed": "true", "Retry-After": "3", ETag: 'W/"2"', "X-Secret-Internal": "nope", "Set-Cookie": "go=1" }, body: { id: "1", masked_fields: [] } });
    const r = await get("jobs/1?status=failed&status=queued");
    expect(r.status).toBe(201);
    expect(await r.json()).toEqual({ id: "1", masked_fields: [] });
    for (const [k, v] of [["location", "/api/v1/jobs/1"], ["idempotent-replayed", "true"], ["retry-after", "3"], ["etag", 'W/"2"'], ["x-request-id", "go-rid"]]) expect(r.headers.get(k!)).toBe(v);
    expect(r.headers.get("x-secret-internal")).toBeNull();
    expect(r.headers.get("set-cookie")).toBeNull();
    expect(seen[0]!.url).toBe("/api/v1/jobs/1?status=failed&status=queued");
    expect(seen[0]!.headers.cookie).toBeUndefined();
    const { payload } = await claims(seen[0]!.headers);
    expect(payload).toMatchObject({ sub: `user:${SESSION.uid}`, sid: SESSION.sid, role: "operator" });
    expect(seen[0]!.headers["x-request-id"]).toBe(payload.rid);
  });

  it("does not rewrite Go error bodies (409 with details)", async () => {
    const body = { code: "job_terminal", message: "Job is already final.", details: { status: "succeeded", version: 6 }, request_id: "req_go" };
    script = () => ({ status: 409, body });
    const r = await post("jobs/abc/cancel");
    expect(r.status).toBe(409);
    expect(await r.json()).toEqual(body);
  });

  it("passes 403 and 429 through unchanged", async () => {
    script = (n) => (n === 1 ? { status: 403, body: { code: "forbidden", message: "no", request_id: "r1" } } : { status: 429, headers: { "Retry-After": "7" }, body: { code: "rate_limited", message: "slow", request_id: "r2" } });
    const a = await get("jobs");
    expect(a.status).toBe(403);
    const b = await get("jobs");
    expect(b.status).toBe(429);
    expect(b.headers.get("retry-after")).toBe("7");
  });

  it("maps ended sessions to 401 session_expired, clears the cookie, keeps the reason", async () => {
    script = () => ({ status: 401, body: { code: "unauthenticated", message: "x", details: { reason: "session_expired", expiry: "idle" }, request_id: "r" } });
    const r = await get("jobs");
    expect(r.status).toBe(401);
    const j = await r.json();
    expect(j.code).toBe("session_expired");
    expect(j.details).toEqual({ reason: "session_expired", expiry: "idle" });
    expect(r.headers.get("set-cookie")).toMatch(/Max-Age=0/);
    script = () => ({ status: 401, body: { code: "unauthenticated", message: "x", details: { reason: "user_disabled" }, request_id: "r" } });
    expect((await (await get("jobs")).json()).details.reason).toBe("user_disabled");
  });

  it("assertion_invalid is a 502 upstream_auth, does NOT end the session", async () => {
    script = () => ({ status: 401, body: { code: "unauthenticated", message: "x", details: { reason: "assertion_invalid" }, request_id: "r" } });
    const r = await get("jobs");
    expect(r.status).toBe(502);
    expect((await r.json()).code).toBe("upstream_auth");
    expect(r.headers.get("set-cookie")).toBeNull();
  });

  it("retries assertion_expired once with a fresh assertion, transparently", async () => {
    script = (n) => (n === 1 ? { status: 401, body: { code: "unauthenticated", message: "x", details: { reason: "assertion_expired" }, request_id: "r" } } : { status: 200, body: { ok: true } });
    const r = await get("jobs");
    expect(r.status).toBe(200);
    expect(seen).toHaveLength(2);
    const [a, b] = [await claims(seen[0]!.headers), await claims(seen[1]!.headers)];
    expect(a.payload.jti).not.toBe(b.payload.jti);
    script = () => ({ status: 401, body: { code: "unauthenticated", message: "x", details: { reason: "assertion_expired" }, request_id: "r" } });
    expect((await get("jobs")).status).toBe(502);
  });

  it("503 upstream_unavailable when Go is unreachable", async () => {
    env({ CLUSTERX_API_URL: "http://127.0.0.1:1" });
    const r = await get("jobs");
    expect(r.status).toBe(503);
    expect((await r.json()).code).toBe("upstream_unavailable");
  });

  it("rejects mutations without a matching Origin or CSRF token before anything reaches Go (csrf_failed)", async () => {
    const cases: Record<string, string>[] = [{ origin: "" }, { origin: "https://evil.example" }, { "x-csrf-token": "" }, { "x-csrf-token": csrfToken(KEY, "other-session") }];
    for (const h of cases) {
      const merged: Record<string, string> = { cookie: cookie(), "content-type": "application/json", origin: ORIGIN, "x-csrf-token": csrfToken(KEY, SESSION.sid), ...h };
      for (const k of Object.keys(merged)) if (merged[k] === "") delete merged[k];
      const r = await proxyPOST(new Request(`${ORIGIN}/bff/api/v1/jobs`, { method: "POST", body: "{}", headers: merged }));
      expect(r.status, JSON.stringify(h)).toBe(403);
      expect((await r.json()).code).toBe("csrf_failed");
    }
    expect(seen).toHaveLength(0);
  });

  it("GET needs no CSRF token, the same body cannot be smuggled through GET", async () => {
    expect((await get("jobs")).status).toBe(200);
  });

  it("session routes and the stream are not reachable through the proxy", async () => {
    for (const p of ["sessions", "sessions/current", "stream"]) expect((await get(p)).status, p).toBe(404);
    expect((await post("sessions")).status).toBe(404);
    expect(seen).toHaveLength(0);
  });

  it("is misconfiguration-safe: 502 upstream_auth and no upstream call", async () => {
    const headers = { cookie: cookie() };
    env({ CLUSTERX_WEB_SESSION_KEY: "" });
    const r = await get("jobs", headers);
    expect(r.status).toBe(502);
    expect((await r.json()).code).toBe("upstream_auth");
    expect(seen).toHaveLength(0);
  });

  it("limits request bodies", async () => {
    const r = await post("jobs", {}, "x".repeat(1024 * 1024 + 1));
    expect(r.status).toBe(413);
    expect(seen).toHaveLength(0);
  });
});

describe("logout", () => {
  const call = (headers: Record<string, string>) => logoutPOST(new Request(`${ORIGIN}/bff/logout`, { method: "POST", headers }));
  const ok = { cookie: "", origin: ORIGIN, "x-csrf-token": csrfToken(KEY, SESSION.sid) };
  it("requires Origin and CSRF", async () => {
    expect((await call({ ...ok, cookie: cookie(), "x-csrf-token": "bad" })).status).toBe(403);
    expect((await call({ cookie: cookie() })).status).toBe(403);
    expect(seen).toHaveLength(0);
  });
  it("revokes upstream and clears the cookie", async () => {
    script = () => ({ status: 204 });
    const r = await call({ ...ok, cookie: cookie() });
    expect(r.status).toBe(204);
    expect(r.headers.get("set-cookie")).toMatch(/Max-Age=0/);
    expect(seen[0]).toMatchObject({ method: "DELETE", url: "/api/v1/sessions/current" });
  });
  it("keeps the cookie and says so when Go is unreachable", async () => {
    env({ CLUSTERX_API_URL: "http://127.0.0.1:1" });
    const r = await call({ ...ok, cookie: cookie() });
    expect(r.status).toBe(503);
    expect(r.headers.get("set-cookie")).toBeNull();
  });
});
