import { asUser, gate, passThrough, roleCookie } from "@/server/bff";
import { csrfToken } from "@/server/crypto";
import type { BffSessionInfo } from "@/lib/bff-contract";

export const dynamic = "force-dynamic";

/** GET /bff/session -> BffSessionInfo (ADR-0006). The role and expiries come from Go `/me` (the DB role, never the cookie's hint). */
export async function GET(req: Request) {
  const g = gate(req, { mutating: false });
  if (g instanceof Response) return g;
  const out = await asUser(g, { method: "GET", path: "/api/v1/me" });
  if ("res" in out) return out.res;
  if (out.go.status !== 200) return passThrough(out.go, out.setCookie);
  const me = JSON.parse(new TextDecoder().decode(out.go.body)) as { user: { id: string; github_login: string }; role: "operator" | "viewer"; session: { expires_at: string; idle_expires_at: string } };
  const info: BffSessionInfo = { user: { id: me.user.id, login: me.user.github_login }, role: me.role, expires_at: me.session.expires_at, idle_expires_at: me.session.idle_expires_at, csrf_token: csrfToken(g.c.sessionKey, g.session.sid) };
  const res = Response.json(info, { headers: { "Cache-Control": "no-store", "X-Request-Id": g.rid } });
  // The console reads the role from here; re-seal the cookie when its hint is stale so later assertions match (RV6B-02).
  for (const c of roleCookie(g.c, g.session, me.role)) res.headers.append("Set-Cookie", c);
  return res;
}
