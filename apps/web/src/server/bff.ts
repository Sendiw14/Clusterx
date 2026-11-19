import { csrfToken, requestId, safeEqual } from "./crypto.ts";
import { clearCookie, readSession, sealSession, sessionCookieName, type SessionCookie } from "./cookies.ts";
import { config, keyring, type WebConfig } from "./config.ts";
import { PASS_HEADERS, callGo, sessionEnded, type GoResult } from "./upstream.ts";

/** BFF-owned envelope, identical shape to Go errors (BFF-02). */
export function bffError(status: number, code: string, message: string, rid: string, extra?: { details?: Record<string, unknown>; setCookie?: string[]; headers?: Record<string, string> }): Response {
  const h = new Headers({ "Content-Type": "application/json", "X-Request-Id": rid, "Cache-Control": "no-store", ...extra?.headers });
  for (const c of extra?.setCookie ?? []) h.append("Set-Cookie", c);
  return new Response(JSON.stringify({ code, message, ...(extra?.details ? { details: extra.details } : {}), request_id: rid }), { status, headers: h });
}

/** Misconfiguration is an infrastructure fault: 502 upstream_auth, details only in the server log. */
export function misconfigured(problems: string[], rid: string): Response {
  console.error(`[bff] ${rid} configuration problems: ${problems.join("; ")}`);
  return bffError(502, "upstream_auth", "The console is not configured to reach its API.", rid);
}

export function passThrough(r: GoResult, setCookie: string[] = []): Response {
  const h = new Headers();
  for (const k of PASS_HEADERS) {
    const v = r.headers.get(k);
    if (v) h.set(k, v);
  }
  for (const c of setCookie) h.append("Set-Cookie", c);
  return new Response(r.status === 204 || r.status === 304 ? null : Buffer.from(r.body), { status: r.status, headers: h });
}

/** Origin must equal the app origin exactly (a missing Origin is rejected), and X-CSRF-Token must match the session. */
export function csrfOk(c: WebConfig, req: Request, sid: string): boolean {
  if (req.headers.get("origin") !== c.appOrigin) return false;
  const given = req.headers.get("x-csrf-token") ?? "";
  // Any key of the rotation window verifies; /bff/session and the rendered pages always issue the current key's token.
  return keyring(c).reduce((ok, k) => safeEqual(given, csrfToken(k, sid)) || ok, false);
}

export type Ctx = { c: WebConfig; rid: string; session: SessionCookie };

/**
 * Common front door for BFF routes that need a session: config, Origin+CSRF (mutations), cookie. Returns a Response to
 * send, or the context.
 */
export function gate(req: Request, opts: { mutating: boolean }): Response | Ctx {
  const rid = requestId();
  const cfg = config();
  if (!cfg.ok) return misconfigured(cfg.problems, rid);
  const c = cfg.config;
  const session = readSession(c, req.headers.get("cookie"));
  if (opts.mutating && req.headers.get("origin") !== c.appOrigin) return bffError(403, "csrf_failed", "The request origin was rejected.", rid);
  if (!session) return bffError(401, "session_expired", "Your session ended. Sign in again.", rid, { setCookie: [clearCookie(c, sessionCookieName(c))] });
  if (opts.mutating && !csrfOk(c, req, session.sid)) return bffError(403, "csrf_failed", "The request was rejected. Reload the page and try again.", rid);
  return { c, rid, session };
}

/** Set-Cookie value that re-seals the session with the database role, or none when the hint is already right (ADR-0006 section 2, RV6B-02). */
export const roleCookie = (c: WebConfig, s: SessionCookie, role: "operator" | "viewer"): string[] => (role === s.role ? [] : [sealSession(c, { ...s, role })]);

/**
 * After a Go 403: asks `GET /me` for the database role and returns the cookie to set if the hint was stale. The refresh
 * call carries the role Go just reported (a hint only), so the demotion that caused the 403 writes one mismatch row, not two.
 */
async function refreshRole(ctx: Ctx, reported: string | undefined): Promise<string[]> {
  const { c, rid, session: s } = ctx;
  const hint = reported === "operator" || reported === "viewer" ? reported : s.role;
  try {
    const me = await callGo(c, { method: "GET", path: "/api/v1/me", as: { userId: s.uid, sid: s.sid, role: hint, rid } }, 5000);
    if (me.status !== 200) return [];
    const role = (JSON.parse(new TextDecoder().decode(me.body)) as { role?: string }).role;
    return role === "operator" || role === "viewer" ? roleCookie(c, s, role) : [];
  } catch {
    return []; // best effort: the 403 itself is the answer
  }
}

/** Calls Go as the session's user and maps the outcomes of ADR-0006 section 2. Returns the raw Go result (plus any refreshed role cookie) to forward, or a BFF error. */
export async function asUser(ctx: Ctx, req: { method: string; path: string; body?: Uint8Array; headers?: Record<string, string> }): Promise<{ go: GoResult; setCookie: string[] } | { res: Response }> {
  const { c, rid, session: s } = ctx;
  let go: GoResult;
  try {
    go = await callGo(c, { ...req, as: { userId: s.uid, sid: s.sid, role: s.role, rid } });
  } catch (e) {
    console.error(`[bff] ${rid} upstream failure: ${(e as Error).name}`);
    return { res: bffError(503, "upstream_unavailable", "The control plane could not be reached.", rid) };
  }
  const ended = sessionEnded(go);
  if (ended) return { res: bffError(401, "session_expired", "Your session ended. Sign in again.", rid, { details: { reason: ended, ...(go.expiry ? { expiry: go.expiry } : {}) }, setCookie: [clearCookie(c, sessionCookieName(c))] }) };
  if (go.status === 401) {
    // assertion_invalid / repeated assertion_expired: a BFF or config fault, never a login redirect.
    console.error(`[bff] ${rid} upstream rejected the assertion: ${go.reason ?? go.code ?? "unknown"}`);
    return { res: bffError(502, "upstream_auth", "The console could not authenticate to its API.", rid) };
  }
  return { go, setCookie: go.status === 403 ? await refreshRole(ctx, go.role) : [] };
}
