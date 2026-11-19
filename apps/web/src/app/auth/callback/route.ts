import { NextResponse } from "next/server";
import { misconfigured } from "@/server/bff";
import { clearCookie, oauthCookieName, readOAuth, readSession, sealSession } from "@/server/cookies";
import { config } from "@/server/config";
import { requestId, safeEqual } from "@/server/crypto";
import { completeAuthorization } from "@/server/oauth";
import { callGo, revokeSession } from "@/server/upstream";

export const dynamic = "force-dynamic";

/**
 * GitHub redirect target. Validates state against the sealed cookie (single use: the cookie is cleared on every
 * outcome), exchanges the code, then mints a login assertion and POSTs /api/v1/sessions. The session cookie is always a
 * fresh value (fixation defense). Outcomes map to the closed `/login?error=` set of ADR-0006 section 3.
 */
export async function GET(req: Request) {
  const rid = requestId();
  const cfg = config();
  const origin = cfg.ok ? cfg.config.appOrigin : new URL(req.url).origin;
  const done = (to: string, cookies: string[] = []) => {
    const res = NextResponse.redirect(new URL(to, origin), 303);
    for (const c of cookies) res.headers.append("Set-Cookie", c);
    res.headers.set("Cache-Control", "no-store");
    return res;
  };
  if (!cfg.ok) {
    misconfigured(cfg.problems, rid);
    return done("/login?error=upstream_auth");
  }
  const c = cfg.config;
  const clear = clearCookie(c, oauthCookieName(c));
  const url = new URL(req.url);
  const oauth = readOAuth(c, req.headers.get("cookie"));
  const state = url.searchParams.get("state") ?? "";
  if (!oauth || !state || !safeEqual(state, oauth.state) || url.searchParams.has("error") || !url.searchParams.get("code")) return done("/login?error=auth_error", [clear]);

  const id = await completeAuthorization(c, url, { state: oauth.state, verifier: oauth.verifier });
  if (!id.ok) return done(`/login?error=${id.reason}`, [clear]);

  let go;
  try {
    go = await callGo(c, { method: "POST", path: "/api/v1/sessions", body: new TextEncoder().encode(JSON.stringify({ github_login: id.login })), headers: { "Content-Type": "application/json" }, as: { login: true, githubId: id.id, rid } });
  } catch (e) {
    console.error(`[bff] ${rid} login upstream failure: ${(e as Error).name}`);
    return done("/login?error=upstream_auth", [clear]);
  }
  if (go.status === 403 && go.reason === "not_allowlisted") return done("/login?error=no_entitlement", [clear]);
  if (go.status === 403 && go.reason === "user_disabled") return done("/login?error=user_disabled", [clear]);
  if (go.status !== 201) {
    console.error(`[bff] ${rid} session create failed: status=${go.status} reason=${go.reason ?? go.code ?? "-"}`);
    return done("/login?error=upstream_auth", [clear]);
  }
  const s = JSON.parse(new TextDecoder().decode(go.body)) as { session_id: string; user: { id: string; github_login: string }; role: "operator" | "viewer"; expires_at: string };
  const cookie = sealSession(c, { sid: s.session_id, uid: s.user.id, login: s.user.github_login, role: s.role, exp: Math.floor(Date.parse(s.expires_at) / 1000) });
  // A still-valid session of this browser is revoked in Go, not just overwritten (RV6-06). Best effort: sign-in succeeded.
  const previous = readSession(c, req.headers.get("cookie"));
  if (previous && previous.sid !== s.session_id) await revokeSession(c, previous, rid);
  return done(oauth.next, [clear, cookie]);
}
