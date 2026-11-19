"use client";
import { createContext, useContext } from "react";
import type { RealtimeEvent, StreamStatus } from "./stream-client";

export interface RealtimeApi {
  status: StreamStatus;
  subscribe: (fn: (e: RealtimeEvent) => void) => () => void;
}
/** `null` outside the live console (the demo has no channel and never refreshes by itself). */
export const RealtimeContext = createContext<RealtimeApi | null>(null);
export const useRealtime = (): RealtimeApi | null => useContext(RealtimeContext);
