import { createHash, randomBytes } from "node:crypto";
import { createServer, type IncomingMessage, type Server } from "node:http";

/**
 * Local mock of GitHub's OAuth endpoints for the live E2E (no real GitHub, no real credentials). It enforces what the BFF
 * must do: S256 PKCE, state round-trip, client_secret_post, single-use codes, redirect_uri match.
 * The identity to sign in as is read from the `mock_identity` cookie on this origin (`<numeric id>:<login>`), which each
 * test sets in its own browser context, so parallel tests never share a "current user". Special values:
 *   `deny`         authorization is denied (redirect with error=access_denied)
 *   `provider_500` the token endpoint answers 500 (GitHub down)
 */
export const MOCK_OAUTH_PORT = 19300;
export const MOCK_CLIENT_ID = "e2e-client-id";
export const MOCK_CLIENT_SECRET = "e2e-client-secret-not-real";

interface Code { challenge: string; redirect: string; id: number; login: string; mode?: string }

const form = async (req: IncomingMessage) => {
  const chunks: Buffer[] = [];
  for await (const c of req) chunks.push(c as Buffer);
  return new URLSearchParams(Buffer.concat(chunks).toString());
};

export function startMockOAuth(): Promise<{ server: Server; log: { path: string; ok: boolean; why?: string }[] }> {
  const codes = new Map<string, Code>();
  const tokens = new Map<string, { id: number; login: string }>();
  const log: { path: string; ok: boolean; why?: string }[] = [];
  const server = createServer(async (req, res) => {
    const u = new URL(req.url ?? "/", `http://localhost:${MOCK_OAUTH_PORT}`);
    const json = (status: number, body: unknown) => { res.writeHead(status, { "Content-Type": "application/json" }); res.end(JSON.stringify(body)); };
    if (u.pathname === "/authorize") {
      const q = u.searchParams;
      const redirect = q.get("redirect_uri") ?? "";
      const state = q.get("state") ?? "";
      const bad = q.get("client_id") !== MOCK_CLIENT_ID || q.get("response_type") !== "code" || q.get("code_challenge_method") !== "S256" || !(q.get("code_challenge") ?? "").length || !state || !redirect;
      if (bad) { log.push({ path: "authorize", ok: false, why: "missing client_id/response_type/S256 challenge/state/redirect_uri" }); return json(400, { error: "invalid_request" }); }
      const ident = /(?:^|;\s*)mock_identity=([^;]*)/.exec(req.headers.cookie ?? "")?.[1] ?? "";
      const back = new URL(redirect);
      back.searchParams.set("state", state);
      if (ident === "deny") back.searchParams.set("error", "access_denied");
      else {
        const [id, login] = ident === "provider_500" ? ["1", "down"] : decodeURIComponent(ident).split(":");
        const code = randomBytes(16).toString("hex");
        codes.set(code, { challenge: q.get("code_challenge")!, redirect, id: Number(id), login: login ?? "", mode: ident === "provider_500" ? "500" : undefined });
        if (!Number(id)) { back.searchParams.delete("state"); back.searchParams.set("error", "server_error"); } else back.searchParams.set("code", code);
      }
      log.push({ path: "authorize", ok: true });
      res.writeHead(302, { Location: back.href });
      return res.end();
    }
    if (u.pathname === "/token" && req.method === "POST") {
      const f = await form(req);
      const c = codes.get(f.get("code") ?? "");
      codes.delete(f.get("code") ?? "");
      if (c?.mode === "500") return json(500, { error: "server_error" });
      const verifierOk = c && createHash("sha256").update(f.get("code_verifier") ?? "").digest("base64url") === c.challenge;
      const ok = c && verifierOk && f.get("client_id") === MOCK_CLIENT_ID && f.get("client_secret") === MOCK_CLIENT_SECRET && f.get("redirect_uri") === c.redirect && f.get("grant_type") === "authorization_code";
      log.push({ path: "token", ok: Boolean(ok), why: ok ? undefined : "bad code, verifier, client secret or redirect_uri" });
      if (!ok || !c) return json(400, { error: "invalid_grant" });
      const t = `mock_${randomBytes(12).toString("hex")}`;
      tokens.set(t, { id: c.id, login: c.login });
      return json(200, { access_token: t, token_type: "bearer", scope: "read:user" });
    }
    if (u.pathname === "/user") {
      const who = tokens.get((req.headers.authorization ?? "").replace(/^Bearer /i, ""));
      return who ? json(200, { id: who.id, login: who.login }) : json(401, { message: "Bad credentials" });
    }
    json(404, { error: "not_found" });
  });
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(MOCK_OAUTH_PORT, "127.0.0.1", () => resolve({ server, log }));
  });
}
