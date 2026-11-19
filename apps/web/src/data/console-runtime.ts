import type { LiveState } from "@clusterx/ui";
import { LiveProvider } from "./live-provider";
import { StreamClient, type RealtimeEvent, type SocketLike, type StreamStatus } from "./stream-client";

const STALE_AFTER_MS = 45_000;

export interface RuntimeSnapshot { live: LiveState; expiring: string | null }

export function toLive(s: StreamStatus, msSinceLastOk: number): LiveState {
  if (s === "live") return "live";
  if (s === "offline") return "offline";
  if (s === "closed") return "stale";
  // Not live: snapshots may still be fresh (polling works) or not (S-STALE).
  if (msSinceLastOk > STALE_AFTER_MS) return "stale";
  return s === "polling" ? "polling" : "reconnecting";
}

export function sessionEndUrl(reason: string, location: { pathname: string; search: string }): string {
  const q = new URLSearchParams(reason === "user_disabled" ? { error: "user_disabled" } : { reason: "session_expired" });
  const p = location.pathname + location.search;
  if (p.startsWith("/app/") && p.length < 512) q.set("next", p);
  return `/auth/end?${q}`;
}

/**
 * Everything imperative about the live console: the provider, the stream, freshness and the session-end redirect. One
 * instance per mounted console; React reads it through `subscribe`/`getSnapshot` (no refs during render).
 */
export class ConsoleRuntime {
  private csrf: string | undefined;
  private refresh: () => void = () => {};
  readonly provider: LiveProvider;
  private client: StreamClient | null = null;
  private lastOk = Date.now();
  private ended = false;
  private snap: RuntimeSnapshot = { live: "reconnecting", expiring: null };
  private listeners = new Set<() => void>();
  private timer: ReturnType<typeof setInterval> | undefined;
  private cleanup: (() => void)[] = [];

  constructor() {
    this.provider = new LiveProvider("/bff", {
      csrf: () => this.csrf,
      refreshCsrf: async () => {
        const r = await fetch("/bff/session", { credentials: "same-origin" });
        return r.ok ? ((await r.json()) as { csrf_token: string }).csrf_token : undefined;
      },
      onSessionEnded: (reason) => this.leave(reason),
      onForbidden: () => this.refresh(),
      onResult: (ok) => { if (ok) this.lastOk = Date.now(); },
    });
  }

  setCsrf(v: string | undefined) { this.csrf = v; }
  setRefresh(fn: () => void) { this.refresh = fn; }

  leave(reason: string) {
    if (this.ended) return;
    this.ended = true;
    window.location.href = sessionEndUrl(reason, window.location);
  }
  /** Sign-out already navigates; stop the "session ended" redirect from racing it. */
  markEnded() { this.ended = true; }

  subscribe = (fn: () => void) => {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  };
  getSnapshot = (): RuntimeSnapshot => this.snap;
  getServerSnapshot = (): RuntimeSnapshot => this.snap;
  realtimeSubscribe = (fn: (e: RealtimeEvent) => void) => this.client?.subscribe(fn) ?? (() => {});
  get status(): StreamStatus { return this.client?.status ?? "connecting"; }

  private recompute() {
    const live = toLive(this.client?.status ?? "connecting", Date.now() - this.lastOk);
    const exp = this.client?.authExpiringAt ?? null;
    const expiring = exp && Date.parse(exp) > Date.now() ? exp : null;
    if (live !== this.snap.live || expiring !== this.snap.expiring) {
      this.snap = { live, expiring };
      for (const f of this.listeners) f();
    }
  }

  start() {
    const c = new StreamClient(
      {
        open: () => {
          const ws = new WebSocket(`${window.location.protocol === "https:" ? "wss" : "ws"}://${window.location.host}/bff/stream`);
          const s: SocketLike = { onmessage: null, onclose: null, close: (code) => ws.close(code) };
          ws.onmessage = (e) => s.onmessage?.(e);
          ws.onclose = (e) => s.onclose?.(e);
          return s;
        },
        setTimeout: (fn, ms) => setTimeout(fn, ms),
        clearTimeout: (h) => clearTimeout(h as number),
        setInterval: (fn, ms) => setInterval(fn, ms),
        clearInterval: (h) => clearInterval(h as number),
        random: Math.random,
      },
      // Stream closed 4401: one authenticated call returns 401 and routes to the right /login outcome.
      () => void this.provider.getConfig(),
    );
    this.client = c;
    this.cleanup.push(c.onChange(() => this.recompute()));
    const on = () => c.online();
    const off = () => c.offline();
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    this.cleanup.push(() => { window.removeEventListener("online", on); window.removeEventListener("offline", off); });
    this.timer = setInterval(() => this.recompute(), 5000);
    c.start();
    this.recompute();
  }
  stop() {
    clearInterval(this.timer);
    for (const f of this.cleanup.splice(0)) f();
    this.client?.stop();
    this.client = null;
  }
}
