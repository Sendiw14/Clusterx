import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { headers } from "next/headers";
import type { ReactNode } from "react";
import { getSession, roleOf } from "@/lib/session";
import { ConsoleProviders } from "../console-providers";

export const metadata: Metadata = { robots: { index: false, follow: false } };
// Per-request: the session decides what renders, and nothing in the console may be cached.
export const dynamic = "force-dynamic";

export default async function ConsoleLayout({ children }: { children: ReactNode }) {
  const session = await getSession();
  if (session.kind === "none") {
    // Ended or missing session: /auth/end clears this browser's cookie and lands on /login with a closed-set reason.
    const path = (await headers()).get("x-pathname") ?? "/app";
    const q = new URLSearchParams(session.ended === "user_disabled" ? { error: "user_disabled" } : { reason: "session_expired" });
    if (path !== "/app") q.set("next", path);
    redirect(`/auth/end?${q}`);
  }
  return (
    <div className="console-root bg-canvas text-ink">
      <ConsoleProviders
        role={roleOf(session)}
        login={session.kind === "active" ? session.user.login : undefined}
        csrf={session.kind === "active" ? session.csrf_token : undefined}
        unavailable={session.kind === "unavailable" ? session.reason : null}
      >
        {children}
      </ConsoleProviders>
    </div>
  );
}
