"use client";
import { useEffect, useEffectEvent, useState } from "react";
import type { ApiFailure, Result } from "@/data/provider";

export type Resource<T> =
  | { state: "loading" }
  | { state: "ok"; data: T; requestId?: string; loadedAt: number }
  | { state: "error"; error: ApiFailure };

/**
 * Loads once per `key` change (the previous key's data is never shown for a new key: it reads as `loading`).
 * `reload` refetches and keeps the previous data on screen with `refreshing` set, so tables dim instead of blanking.
 */
export function useResource<T>(load: () => Promise<Result<T>>, key: string): Resource<T> & { reload: () => void; refreshing: boolean } {
  const [store, setStore] = useState<{ key: string; res: Resource<T> } | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [nonce, setNonce] = useState(0);
  const run = useEffectEvent(() => load());

  useEffect(() => {
    let live = true;
    run().then((r) => {
      if (!live) return;
      setRefreshing(false);
      setStore({ key, res: r.ok ? { state: "ok", data: r.data, requestId: r.requestId, loadedAt: Date.now() } : { state: "error", error: r.error } });
    });
    return () => { live = false; };
  }, [key, nonce]);

  const res: Resource<T> = store && store.key === key ? store.res : { state: "loading" };
  return { ...res, refreshing, reload: () => { setRefreshing(true); setNonce((n) => n + 1); } };
}
