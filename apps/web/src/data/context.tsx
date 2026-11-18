"use client";
import { createContext, useContext, type ReactNode } from "react";
import type { DataProvider } from "./provider";

const Ctx = createContext<DataProvider | null>(null);

export function DataProviderContext({ provider, children }: { provider: DataProvider; children: ReactNode }) {
  return <Ctx.Provider value={provider}>{children}</Ctx.Provider>;
}

export function useDataProvider(): DataProvider {
  const p = useContext(Ctx);
  if (!p) throw new Error("useDataProvider: no DataProviderContext above this view");
  return p;
}
