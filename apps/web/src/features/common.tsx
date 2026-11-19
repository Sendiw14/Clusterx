"use client";
import type { ReactNode } from "react";
import { ErrorState, SkeletonRows, type ErrorInfo } from "@clusterx/ui";
import type { Resource } from "./use-resource";

export const toErrorInfo = (e: { code: string; status: number; requestId?: string }): ErrorInfo => ({ code: e.code, status: e.status || undefined, requestId: e.requestId });

/** Renders loading (skeleton), error (retry) or the data. One failing region never blanks the page. */
export function Load<T>({ res, onRetry, rows = 4, label, children, errorHeading }: { res: Resource<T> & { reload: () => void }; onRetry?: () => void; rows?: number; label: string; children: (data: T) => ReactNode; errorHeading?: string }) {
  if (res.state === "loading") return <SkeletonRows rows={rows} label={label} />;
  if (res.state === "error") return <ErrorState error={toErrorInfo(res.error)} onRetry={onRetry ?? res.reload} heading={errorHeading} />;
  return <>{children(res.data)}</>;
}

const REASONS: Record<string, string> = {
  no_capable_worker: "No capable worker",
  capable_workers_offline: "Capable workers offline",
  capable_workers_draining: "Capable workers draining",
};
export const reasonLabel = (r: string): string => REASONS[r] ?? r;

/** UUIDv7 ids share their leading digits (a timestamp), so the tail is what tells jobs apart. */
export const shortId = (id: string) => (id.startsWith("demo-") ? id : id.slice(-8));
