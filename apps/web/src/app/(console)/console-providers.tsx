"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useMemo, useState, useSyncExternalStore, type ReactNode } from "react";
import { ConsoleShell, Notice, type LookupResult, type Role } from "@clusterx/ui";
import { DataProviderContext } from "@/data/context";
import { ConsoleRuntime } from "@/data/console-runtime";
import { RealtimeContext } from "@/data/realtime";

/** The only place the live runtime (provider, stream, freshness) is created. Console pages read it through context. */
export function ConsoleProviders({ role, login, csrf, unavailable, children }: { role: Role; login?: string; csrf?: string; unavailable: "upstream_auth" | "upstream_unavailable" | null; children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [rt] = useState(() => new ConsoleRuntime());
  const snap = useSyncExternalStore(rt.subscribe, rt.getSnapshot, rt.getServerSnapshot);

  useEffect(() => { rt.setCsrf(csrf); }, [rt, csrf]);
  useEffect(() => { rt.setRefresh(() => router.refresh()); }, [rt, router]);
  useEffect(() => {
    if (unavailable) return;
    rt.start();
    return () => rt.stop();
  }, [rt, unavailable]);

  const realtime = useMemo(() => ({ status: snap.live === "live" ? ("live" as const) : ("reconnecting" as const), subscribe: rt.realtimeSubscribe }), [rt, snap.live]);

  const lookup = useMemo(
    () => async (id: string): Promise<LookupResult> => {
      const j = await rt.provider.getJob(id);
      if (j.ok) return { kind: "job", id };
      if (j.error.code === "expired") return { kind: "expired" };
      if (j.error.code !== "job_not_found") return { kind: "error", message: "Lookup failed. Try again." };
      const w = await rt.provider.getWorker(id);
      if (w.ok) return { kind: "worker", id };
      return w.error.code === "worker_not_found" ? { kind: "none" } : { kind: "error", message: "Lookup failed. Try again." };
    },
    [rt],
  );

  async function signOut() {
    const fail = () => window.alert("Sign-out could not be completed. You are still signed in. Try again.");
    try {
      const r = await fetch("/bff/logout", { method: "POST", credentials: "same-origin", headers: { "X-CSRF-Token": csrf ?? "" } });
      if (r.ok || r.status === 401) {
        rt.markEnded();
        // eslint-disable-next-line @next/next/no-location-assign-relative-destination -- a full navigation drops all console state
        window.location.href = "/login?reason=signed_out";
      } else fail();
    } catch {
      fail();
    }
  }

  const banner = unavailable ? (
    <Notice variant="error" lead={unavailable === "upstream_auth" ? "The console could not reach its own API." : "The control plane is unreachable."} icon="i-signal-off">
      {unavailable === "upstream_auth" ? "This is an infrastructure fault, not a problem with your account, so you are not asked to sign in again. Data cannot load until it is fixed." : "Data cannot load right now. Nothing was changed. Reload to try again."}
    </Notice>
  ) : snap.expiring ? (
    <Notice variant="warning" lead="Your session ends soon." icon="i-alert">
      It ends at <time dateTime={snap.expiring}>{new Date(snap.expiring).toISOString().slice(11, 19)} UTC</time>. Unsaved changes on this page will be lost. <Link href={`/auth/github?next=${encodeURIComponent(pathname)}`}>Sign in again</Link> to continue.
    </Notice>
  ) : null;

  return (
    <RealtimeContext.Provider value={realtime}>
      <DataProviderContext provider={rt.provider} role={role}>
        <ConsoleShell role={role} login={login} live={unavailable ? "offline" : snap.live} liveDetail={unavailable ? "no backend" : undefined} lookup={lookup} banner={banner} onSignOut={login ? () => void signOut() : undefined}>
          {children}
        </ConsoleShell>
      </DataProviderContext>
    </RealtimeContext.Provider>
  );
}
