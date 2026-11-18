import type { Metadata } from "next";
import type { ReactNode } from "react";
import { getSession, roleOf } from "@/lib/session";
import { ConsoleProviders } from "../console-providers";

export const metadata: Metadata = { robots: { index: false, follow: false } };
// Per-request: the session decides what renders, and nothing in the console may be cached.
export const dynamic = "force-dynamic";

export default async function ConsoleLayout({ children }: { children: ReactNode }) {
  const session = await getSession();
  // `none` redirects to /login?next= once sessions exist (M5). `unavailable` renders the shell with an explicit banner.
  return (
    <div className="console-root bg-canvas text-ink">
      <ConsoleProviders role={roleOf(session)} login={session.kind === "active" ? session.user.login : undefined} unavailable={session.kind === "unavailable"}>
        {children}
      </ConsoleProviders>
    </div>
  );
}
