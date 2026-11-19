import { NextResponse } from "next/server";
import { clearCookie, clampNext, sessionCookieName } from "@/server/cookies";
import { config } from "@/server/config";

export const dynamic = "force-dynamic";

const REASONS = new Set(["signed_out", "session_expired"]);

/**
 * Where server-rendered console pages send a request whose session ended: clears this browser's own cookie (no side
 * effect on anyone else), then lands on /login with a reason from the closed set and a clamped `next`.
 */
export function GET(req: Request) {
  const cfg = config();
  const u = new URL(req.url);
  const origin = cfg.ok ? cfg.config.appOrigin : u.origin;
  const q = new URLSearchParams();
  const error = u.searchParams.get("error");
  if (error === "user_disabled") q.set("error", error);
  else q.set("reason", REASONS.has(u.searchParams.get("reason") ?? "") ? (u.searchParams.get("reason") as string) : "session_expired");
  const next = clampNext(u.searchParams.get("next"), origin);
  if (next !== "/app") q.set("next", next);
  const res = NextResponse.redirect(new URL(`/login?${q}`, origin), 303);
  if (cfg.ok) res.headers.append("Set-Cookie", clearCookie(cfg.config, sessionCookieName(cfg.config)));
  res.headers.set("Cache-Control", "no-store");
  return res;
}
