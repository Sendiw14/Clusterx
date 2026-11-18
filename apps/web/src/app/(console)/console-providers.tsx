"use client";
import Link from "next/link";
import { useMemo, type ReactNode } from "react";
import { ConsoleShell, Notice, type LookupResult, type Role } from "@clusterx/ui";
import { DataProviderContext } from "@/data/context";
import { LiveProvider } from "@/data/live-provider";

/** The only place the live provider is created. Console pages read it through `useDataProvider()`. */
export function ConsoleProviders({ role, login, unavailable, children }: { role: Role; login?: string; unavailable: boolean; children: ReactNode }) {
  const provider = useMemo(() => new LiveProvider(), []);
  const lookup = useMemo(
    () => async (id: string): Promise<LookupResult> => {
      const j = await provider.getJob(id);
      if (j.ok) return { kind: "job", id };
      if (j.error.code === "expired") return { kind: "expired" };
      if (j.error.code !== "job_not_found") return { kind: "error", message: j.error.code === "not_implemented" ? "Lookup is unavailable: the backend is not connected in this build." : "Lookup failed. Try again." };
      const w = await provider.getWorker(id);
      if (w.ok) return { kind: "worker", id };
      return w.error.code === "worker_not_found" ? { kind: "none" } : { kind: "error", message: "Lookup failed. Try again." };
    },
    [provider],
  );
  return (
    <DataProviderContext provider={provider}>
      <ConsoleShell
        role={role}
        login={login}
        live="offline"
        liveDetail="not connected"
        lookup={lookup}
        banner={
          unavailable ? (
            <Notice variant="warning" lead="The console is not connected to a backend in this build." icon="i-signal-off">
              Sessions and the API proxy arrive with the authentication milestone, so data cannot load yet. The layout, filters and states are real. See the <Link href="/demo">demo</Link> for the same views on labelled fixture data.
            </Notice>
          ) : null
        }
      >
        {children}
      </ConsoleShell>
    </DataProviderContext>
  );
}
