import { bffError, gate } from "@/server/bff";
import { clearCookie, sessionCookieName } from "@/server/cookies";
import { callGo } from "@/server/upstream";

export const dynamic = "force-dynamic";

/**
 * POST /bff/logout: Origin + CSRF first (mutating), then DELETE /sessions/current. Already-ended sessions (401) count
 * as logged out. If Go is unreachable the cookie is kept and the user is told, so a live session is never silently left behind.
 */
export async function POST(req: Request) {
  const g = gate(req, { mutating: true });
  if (g instanceof Response) return g;
  const { c, rid, session: s } = g;
  try {
    const r = await callGo(c, { method: "DELETE", path: "/api/v1/sessions/current", as: { userId: s.uid, sid: s.sid, role: s.role, rid } });
    if (r.status !== 204 && r.status !== 401) return bffError(502, "upstream_auth", "Sign-out could not be completed.", rid);
  } catch {
    return bffError(503, "upstream_unavailable", "The control plane could not be reached. You are still signed in.", rid);
  }
  return new Response(null, { status: 204, headers: { "Set-Cookie": clearCookie(c, sessionCookieName(c)), "Cache-Control": "no-store", "X-Request-Id": rid } });
}
