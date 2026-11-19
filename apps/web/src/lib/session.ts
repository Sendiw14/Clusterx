import "server-only";
import { cookies } from "next/headers";
import { cache } from "react";
import type { BffSessionInfo } from "./bff-contract";
import { config } from "@/server/config";
import { readSession, sessionCookieName } from "@/server/cookies";
import { csrfToken, requestId } from "@/server/crypto";
import { callGo, sessionEnded } from "@/server/upstream";

/**
 * Server-only session module (ADR-0006), used by the console layout and server components.
 *  - `unavailable`: the BFF cannot reach or authenticate to Go (misconfiguration, outage, rejected assertion). The shell
 *    renders with an explicit banner; nothing is shown that was not authorized. Never a login redirect (no login loop).
 *  - `none`: no valid session; `ended` says why when Go reported it. Console routes redirect to /login via /auth/end.
 *  - `active`: an authenticated principal. The role comes from Go `/me` (DB role), never from the cookie.
 * The UI never decides authorization: Go enforces it on every request. The role here only hides controls.
 */
export type Session =
  | { kind: "unavailable"; reason: "upstream_auth" | "upstream_unavailable" }
  | { kind: "none"; ended?: "session_expired" | "user_disabled" }
  | ({ kind: "active" } & BffSessionInfo);

export const getSession = cache(async (): Promise<Session> => {
  const cfg = config();
  if (!cfg.ok) {
    console.error(`[bff] configuration problems: ${cfg.problems.join("; ")}`);
    return { kind: "unavailable", reason: "upstream_auth" };
  }
  const c = cfg.config;
  const jar = await cookies();
  const raw = jar.get(sessionCookieName(c))?.value;
  const s = readSession(c, raw ? `${sessionCookieName(c)}=${raw}` : null);
  // A cookie that no longer opens (expired or tampered) is "ended"; no cookie at all is a plain visitor.
  if (!s) return raw ? { kind: "none", ended: "session_expired" } : { kind: "none" };
  const rid = requestId();
  try {
    const r = await callGo(c, { method: "GET", path: "/api/v1/me", as: { userId: s.uid, sid: s.sid, role: s.role, rid } });
    const ended = sessionEnded(r);
    if (ended) return { kind: "none", ended };
    if (r.status === 401) return { kind: "unavailable", reason: "upstream_auth" };
    if (r.status !== 200) return { kind: "unavailable", reason: "upstream_unavailable" };
    const me = JSON.parse(new TextDecoder().decode(r.body)) as { user: { id: string; github_login: string }; role: "operator" | "viewer"; session: { expires_at: string; idle_expires_at: string } };
    return { kind: "active", user: { id: me.user.id, login: me.user.github_login }, role: me.role, expires_at: me.session.expires_at, idle_expires_at: me.session.idle_expires_at, csrf_token: csrfToken(c.sessionKey, s.sid) };
  } catch {
    return { kind: "unavailable", reason: "upstream_unavailable" };
  }
});

export function roleOf(s: Session): "operator" | "viewer" | null {
  return s.kind === "active" ? s.role : null;
}
