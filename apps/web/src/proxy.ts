import { NextResponse, type NextRequest } from "next/server";
import { buildCsp, isNoncePath } from "@/lib/csp";

/**
 * Console, /login and /auth/*: (1) hands the requested path to the server-rendered layout so an ended session can return to
 * it after sign-in (any client-supplied `x-pathname` is overwritten, and the value is clamped again before use), and
 * (2) issues a fresh nonce CSP per request (RV6-09). The nonce goes on the request headers so Next stamps it on its own
 * scripts, and on the response as the only CSP these routes carry. Every other path passes through untouched and gets the
 * static policy from `next.config.ts`. The matcher is deliberately broad and the decision is made here, case-insensitively
 * (RV6B-03: a `/APP` or `/login/x` 404 used to match neither policy and carried no CSP).
 */
export function proxy(req: NextRequest) {
  if (!isNoncePath(req.nextUrl.pathname)) return NextResponse.next();
  const nonce = btoa(crypto.randomUUID());
  const csp = buildCsp({ nonce, dev: process.env.NODE_ENV !== "production" });
  const h = new Headers(req.headers);
  h.set("x-pathname", req.nextUrl.pathname + req.nextUrl.search);
  h.set("Content-Security-Policy", csp);
  const res = NextResponse.next({ request: { headers: h } });
  res.headers.set("Content-Security-Policy", csp);
  return res;
}

export const config = { matcher: ["/((?!_next/|fonts/|images/|icon\\.svg$).*)"] };
