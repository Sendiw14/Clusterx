"use client";
import { useEffect, useEffectEvent, useRef, useState } from "react";
import { useRealtime } from "@/data/realtime";
import type { RealtimeEvent } from "@/data/stream-client";
import type { ApiFailure, Result } from "@/data/provider";

export type Resource<T> =
  | { state: "loading" }
  | { state: "ok"; data: T; requestId?: string; loadedAt: number }
  | { state: "error"; error: ApiFailure };

/**
 * Whether a realtime event makes this resource refetch. Hints carry no data (FR-RT-01): a refetch is the only effect.
 * `resync` and `poll` always refetch (reconnect, gap, overflow, polling fallback: AC-16).
 */
export type LiveMatch<T> = (e: Extract<RealtimeEvent, { type: "hint" }>, current: T) => boolean;

/** Bursts of hints become at most one refetch per second (throttle, so a steady stream still refreshes). */
const COALESCE_MS = 1000;

/**
 * Loads once per `key` change (the previous key's data is never shown for a new key: it reads as `loading`).
 * `reload` refetches and keeps the previous data on screen with `refreshing` set, so tables dim instead of blanking.
 * With `live`, realtime events refetch in place; bursts are coalesced into one request.
 */
export function useResource<T>(load: () => Promise<Result<T>>, key: string, live?: LiveMatch<T>): Resource<T> & { reload: () => void; refreshing: boolean } {
  const [store, setStore] = useState<{ key: string; res: Resource<T> } | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [nonce, setNonce] = useState(0);
  const run = useEffectEvent(() => load());
  const rt = useRealtime();
  const current = useRef<{ key: string; data?: T }>({ key });

  useEffect(() => {
    let alive = true;
    run().then((r) => {
      if (!alive) return;
      setRefreshing(false);
      // A failed background refresh keeps the last good data on screen (stale, not blank) when there is one.
      setStore((prev) => (!r.ok && prev && prev.key === key && prev.res.state === "ok" && nonce > 0 ? prev : { key, res: r.ok ? { state: "ok", data: r.data, requestId: r.requestId, loadedAt: Date.now() } : { state: "error", error: r.error } }));
    });
    return () => { alive = false; };
  }, [key, nonce]);

  const res: Resource<T> = store && store.key === key ? store.res : { state: "loading" };
  const data = res.state === "ok" ? res.data : undefined;
  useEffect(() => { current.current = { key, data }; });

  useEffect(() => {
    if (!rt || !live) return;
    let t: ReturnType<typeof setTimeout> | undefined;
    const off = rt.subscribe((e) => {
      const cur = current.current.data;
      const hit = e.type === "hint" ? cur !== undefined && live(e, cur) : true;
      if (!hit) return;
      if (t) return;
      t = setTimeout(() => { t = undefined; setNonce((n) => n + 1); }, COALESCE_MS);
    });
    return () => { off(); clearTimeout(t); };
  }, [rt, live]);

  return { ...res, refreshing, reload: () => { setRefreshing(true); setNonce((n) => n + 1); } };
}
