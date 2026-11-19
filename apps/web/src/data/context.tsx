"use client";
import { createContext, useContext, type ReactNode } from "react";
import type { ConsoleProvider } from "./console-provider";
import type { DataProvider } from "./provider";

const Ctx = createContext<DataProvider | null>(null);
export type ConsoleRole = "operator" | "viewer" | null;
const RoleCtx = createContext<ConsoleRole>(null);

export function DataProviderContext({ provider, role = null, children }: { provider: DataProvider; role?: ConsoleRole; children: ReactNode }) {
  return (
    <Ctx.Provider value={provider}>
      <RoleCtx.Provider value={role}>{children}</RoleCtx.Provider>
    </Ctx.Provider>
  );
}

export function useDataProvider(): DataProvider {
  const p = useContext(Ctx);
  if (!p) throw new Error("useDataProvider: no DataProviderContext above this view");
  return p;
}

/** Console-only capabilities (mutations, audit, tokens). Throws under /demo, which has no such provider. */
export function useConsoleProvider(): ConsoleProvider {
  const p = useDataProvider();
  if (p.kind !== "live") throw new Error("useConsoleProvider: this view needs the live console provider");
  return p as ConsoleProvider;
}

/** The viewer/operator role from the BFF session; `null` in the demo and while unknown. The UI hides, Go enforces. */
export const useRole = (): ConsoleRole => useContext(RoleCtx);
