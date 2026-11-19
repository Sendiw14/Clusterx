import { open, seal } from "./crypto.ts";
import { keyring, type WebConfig } from "./config.ts";

export interface SessionCookie {
  sid: string;
  uid: string;
  login: string;
  role: "operator" | "viewer";
  /** Absolute session expiry, epoch seconds. */
  exp: number;
}
export interface OAuthCookie { state: string; verifier: string; next: string; exp: number }

export const sessionCookieName = (c: WebConfig) => (c.secureCookies ? "__Host-clusterx_session" : "clusterx_session");
export const oauthCookieName = (c: WebConfig) => (c.secureCookies ? "__Host-clusterx_oauth" : "clusterx_oauth");

export function cookieHeader(c: WebConfig, name: string, value: string, maxAgeSeconds: number): string {
  return [`${name}=${value}`, "Path=/", "HttpOnly", "SameSite=Lax", c.secureCookies ? "Secure" : "", `Max-Age=${Math.max(0, Math.floor(maxAgeSeconds))}`].filter(Boolean).join("; ");
}
export const clearCookie = (c: WebConfig, name: string) => cookieHeader(c, name, "", 0);

export function parseCookies(header: string | null | undefined): Map<string, string> {
  const m = new Map<string, string>();
  for (const part of (header ?? "").split(";")) {
    const i = part.indexOf("=");
    if (i > 0) m.set(part.slice(0, i).trim(), part.slice(i + 1).trim());
  }
  return m;
}

const now = () => Math.floor(Date.now() / 1000);

export function sealSession(c: WebConfig, s: SessionCookie): string {
  return cookieHeader(c, sessionCookieName(c), seal(c.sessionKey, "session", s), s.exp - now());
}
export function readSession(c: WebConfig, cookieHeaderValue: string | null | undefined): SessionCookie | null {
  const s = open<SessionCookie>(keyring(c), "session", parseCookies(cookieHeaderValue).get(sessionCookieName(c)));
  return s && typeof s.sid === "string" && typeof s.uid === "string" && s.exp > now() && (s.role === "operator" || s.role === "viewer") ? s : null;
}
export function sealOAuth(c: WebConfig, o: OAuthCookie): string {
  return cookieHeader(c, oauthCookieName(c), seal(c.sessionKey, "oauth", o), 600);
}
export function readOAuth(c: WebConfig, cookieHeaderValue: string | null | undefined): OAuthCookie | null {
  const o = open<OAuthCookie>(keyring(c), "oauth", parseCookies(cookieHeaderValue).get(oauthCookieName(c)));
  return o && o.exp > now() ? o : null;
}

/** `next` is clamped to a same-origin path under /app (ADR-0006 section 3). Anything else becomes /app. */
export function clampNext(next: string | null | undefined, origin: string): string {
  if (!next || next.length > 512 || !next.startsWith("/") || /[\\\u0000-\u001f]/.test(next)) return "/app";
  try {
    const u = new URL(next, origin);
    if (u.origin !== origin || !(u.pathname === "/app" || u.pathname.startsWith("/app/"))) return "/app";
    return u.pathname + u.search;
  } catch {
    return "/app";
  }
}
