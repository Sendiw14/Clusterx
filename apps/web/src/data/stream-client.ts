/**
 * Browser end of the live channel (ADR-0006 section 4, FR-RT-01, AC-16). Messages are HINTS: they only make views
 * refetch snapshots. Pure logic with injected socket and timers so reconnect, backoff, gap and polling behaviour are unit
 * tested without a browser.
 */
export type StreamStatus = "connecting" | "live" | "reconnecting" | "polling" | "offline" | "closed";

export type RealtimeEvent =
  | { type: "hint"; entity: "job" | "worker" | "metrics"; id: string | null; version: number }
  /** Snapshots must be refetched: reconnect, buffer overflow, gap, or server restart. */
  | { type: "resync"; reason: "reconnect" | "buffer_overflow" | "server_restart" | "gap_detected" | "online" }
  /** Polling fallback tick (no live channel). */
  | { type: "poll" };

export interface SocketLike {
  onmessage: ((e: { data: unknown }) => void) | null;
  onclose: ((e: { code: number }) => void) | null;
  close(code?: number): void;
}

export interface StreamDeps {
  open: () => SocketLike;
  setTimeout: (fn: () => void, ms: number) => unknown;
  clearTimeout: (h: unknown) => void;
  setInterval: (fn: () => void, ms: number) => unknown;
  clearInterval: (h: unknown) => void;
  random: () => number;
}

export const BACKOFF_BASE_MS = 1000;
export const BACKOFF_CAP_MS = 30_000;
export const POLL_MS = 15_000;
/** Consecutive failed connection attempts before the polling fallback starts. */
export const POLL_AFTER_FAILURES = 2;

export const backoff = (attempt: number, random: number): number => Math.round(Math.min(BACKOFF_CAP_MS, BACKOFF_BASE_MS * 2 ** attempt) * (0.5 + random / 2));

export class StreamClient {
  status: StreamStatus = "connecting";
  authExpiringAt: string | null = null;
  private sock: SocketLike | null = null;
  private attempts = 0;
  private everLive = false;
  private stopped = false;
  private retry: unknown;
  private poll: unknown;
  private subs = new Set<(e: RealtimeEvent) => void>();
  private statusSubs = new Set<() => void>();

  constructor(private deps: StreamDeps, private onSessionEnded: () => void) {}

  subscribe(fn: (e: RealtimeEvent) => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }
  onChange(fn: () => void): () => void {
    this.statusSubs.add(fn);
    return () => this.statusSubs.delete(fn);
  }
  private emit(e: RealtimeEvent) {
    for (const f of this.subs) f(e);
  }
  private notify() {
    for (const f of this.statusSubs) f();
  }
  private set(s: StreamStatus) {
    if (this.status === s) return;
    this.status = s;
    this.notify();
  }

  start() {
    this.stopped = false;
    this.connect();
  }
  stop() {
    this.stopped = true;
    this.deps.clearTimeout(this.retry);
    this.stopPolling();
    const s = this.sock;
    this.sock = null;
    s?.close(1000);
  }
  /** The `online` event: retry now and refetch. */
  online() {
    if (this.stopped || this.status === "closed") return;
    this.deps.clearTimeout(this.retry);
    this.attempts = 0;
    if (!this.sock) this.connect();
    this.emit({ type: "resync", reason: "online" });
  }
  offline() {
    if (this.stopped) return;
    this.deps.clearTimeout(this.retry);
    const s = this.sock;
    this.sock = null;
    s?.close(1000);
    this.set("offline");
  }

  private connect() {
    if (this.sock || this.stopped) return;
    this.set(this.everLive || this.attempts > 0 ? (this.attempts >= POLL_AFTER_FAILURES ? "polling" : "reconnecting") : "connecting");
    let s: SocketLike;
    try {
      s = this.deps.open();
    } catch {
      this.failed();
      return;
    }
    this.sock = s;
    s.onmessage = (e) => this.message(e.data);
    s.onclose = (e) => {
      if (this.sock !== s) return;
      this.sock = null;
      this.closed(e.code);
    };
  }

  private message(raw: unknown) {
    let m: { type?: string; entity?: string; id?: string | null; version?: number; reason?: string; expires_at?: string };
    try {
      m = JSON.parse(String(raw));
    } catch {
      return;
    }
    if (m.type === "hello") {
      const reconnect = this.everLive;
      this.everLive = true;
      this.attempts = 0;
      this.stopPolling();
      this.set("live");
      // After any reconnect the stream may have missed hints: snapshots are authoritative (AC-16).
      if (reconnect) this.emit({ type: "resync", reason: "reconnect" });
    } else if (m.type === "hint" && (m.entity === "job" || m.entity === "worker" || m.entity === "metrics") && typeof m.version === "number") {
      this.emit({ type: "hint", entity: m.entity, id: m.id ?? null, version: m.version });
    } else if (m.type === "resync" && (m.reason === "buffer_overflow" || m.reason === "server_restart" || m.reason === "gap_detected")) {
      this.emit({ type: "resync", reason: m.reason });
    } else if (m.type === "auth_expiring" && typeof m.expires_at === "string") {
      this.authExpiringAt = m.expires_at;
      this.notify();
    }
  }

  private closed(code: number) {
    if (this.stopped) return;
    if (code === 4401) {
      // Session ended or revoked: no reconnect. The stream is closed; the provider's next call returns 401 and the page leaves.
      this.set("closed");
      this.stopPolling();
      this.onSessionEnded();
      return;
    }
    if (code === 4403 || code === 1008) {
      // Not allowed to stream: stay usable through snapshots.
      this.attempts = POLL_AFTER_FAILURES;
      this.set("polling");
      this.startPolling();
      return;
    }
    this.failed();
  }

  private failed() {
    this.attempts++;
    this.set(this.attempts >= POLL_AFTER_FAILURES ? "polling" : "reconnecting");
    if (this.attempts >= POLL_AFTER_FAILURES) this.startPolling();
    this.deps.clearTimeout(this.retry);
    this.retry = this.deps.setTimeout(() => this.connect(), backoff(this.attempts, this.deps.random()));
  }

  private startPolling() {
    if (this.poll) return;
    this.poll = this.deps.setInterval(() => this.emit({ type: "poll" }), POLL_MS);
  }
  private stopPolling() {
    if (!this.poll) return;
    this.deps.clearInterval(this.poll);
    this.poll = undefined;
  }
}
