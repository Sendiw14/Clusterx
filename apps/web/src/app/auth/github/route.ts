import { NextResponse } from "next/server";
import { misconfigured } from "@/server/bff";
import { clampNext, sealOAuth } from "@/server/cookies";
import { config } from "@/server/config";
import { requestId } from "@/server/crypto";
import { beginAuthorization } from "@/server/oauth";

export const dynamic = "force-dynamic";

/** Starts sign-in: state + PKCE verifier + clamped `next` go into a sealed 10-minute cookie; the browser is sent to GitHub. */
export async function GET(req: Request) {
  const rid = requestId();
  const cfg = config();
  if (!cfg.ok) {
    misconfigured(cfg.problems, rid);
    return NextResponse.redirect(new URL("/login?error=upstream_auth", new URL(req.url).origin), 303);
  }
  const c = cfg.config;
  const a = await beginAuthorization(c);
  const next = clampNext(new URL(req.url).searchParams.get("next"), c.appOrigin);
  const res = NextResponse.redirect(a.url, 303);
  res.headers.append("Set-Cookie", sealOAuth(c, { state: a.state, verifier: a.verifier, next, exp: Math.floor(Date.now() / 1000) + 600 }));
  res.headers.set("Cache-Control", "no-store");
  return res;
}
