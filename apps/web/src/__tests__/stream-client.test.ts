import { describe, expect, it } from "vitest";
import { BACKOFF_CAP_MS, POLL_MS, StreamClient, backoff, type RealtimeEvent, type SocketLike } from "@/data/stream-client";

function harness() {
  const socks: (SocketLike & { closed?: number; send: (m: unknown) => void; drop: (code: number) => void })[] = [];
  const timers: { fn: () => void; ms: number; every: boolean; dead?: boolean }[] = [];
  let sessionEnded = 0;
  const events: RealtimeEvent[] = [];
  const c = new StreamClient(
    {
      open: () => {
        const s = {
          onmessage: null as SocketLike["onmessage"],
          onclose: null as SocketLike["onclose"],
          closed: undefined as number | undefined,
          close(code?: number) { s.closed = code; },
          send(m: unknown) { s.onmessage?.({ data: JSON.stringify(m) }); },
          drop(code: number) { s.onclose?.({ code }); },
        };
        socks.push(s);
        return s;
      },
      setTimeout: (fn, ms) => { const t = { fn, ms, every: false }; timers.push(t); return t; },
      clearTimeout: (h) => { if (h) (h as { dead?: boolean }).dead = true; },
      setInterval: (fn, ms) => { const t = { fn, ms, every: true }; timers.push(t); return t; },
      clearInterval: (h) => { if (h) (h as { dead?: boolean }).dead = true; },
      random: () => 1,
    },
    () => { sessionEnded++; },
  );
  c.subscribe((e) => events.push(e));
  const fire = (pred: (t: (typeof timers)[number]) => boolean) => { const t = timers.filter((x) => !x.dead && pred(x)).at(-1); if (!t) throw new Error("no timer"); if (!t.every) t.dead = true; t.fn(); };
  return { c, socks, timers, events, fire, ended: () => sessionEnded };
}
const hello = { type: "hello", server_time: "2026-11-19T00:00:00Z", session_expires_at: "2026-11-19T12:00:00Z" };

describe("StreamClient (AC-16 client side)", () => {
  it("goes live on hello and forwards hints and resync, ignoring junk", () => {
    const h = harness();
    h.c.start();
    expect(h.c.status).toBe("connecting");
    h.socks[0]!.send(hello);
    expect(h.c.status).toBe("live");
    h.socks[0]!.send({ type: "hint", entity: "job", id: "j1", version: 4 });
    h.socks[0]!.send({ type: "resync", reason: "buffer_overflow" });
    h.socks[0]!.send({ type: "hint", entity: "bogus", id: null, version: 1 });
    h.socks[0]!.onmessage?.({ data: "not json" });
    expect(h.events).toEqual([{ type: "hint", entity: "job", id: "j1", version: 4 }, { type: "resync", reason: "buffer_overflow" }]);
  });

  it("reconnects with backoff after a drop and forces a snapshot refetch on the next hello (stream drop, AC-16)", () => {
    const h = harness();
    h.c.start();
    h.socks[0]!.send(hello);
    h.socks[0]!.drop(1006);
    expect(h.c.status).toBe("reconnecting");
    expect(h.socks).toHaveLength(1);
    h.fire((t) => !t.every);
    expect(h.socks).toHaveLength(2);
    h.socks[1]!.send(hello);
    expect(h.c.status).toBe("live");
    expect(h.events).toContainEqual({ type: "resync", reason: "reconnect" });
  });

  it("the first hello is not a resync (the page loaded its own snapshot)", () => {
    const h = harness();
    h.c.start();
    h.socks[0]!.send(hello);
    expect(h.events).toEqual([]);
  });

  it("falls back to polling after repeated failures and stops polling when live again", () => {
    const h = harness();
    h.c.start();
    h.socks[0]!.drop(1006);
    h.fire((t) => !t.every);
    h.socks[1]!.drop(1006);
    expect(h.c.status).toBe("polling");
    const poll = h.timers.find((t) => t.every && !t.dead)!;
    expect(poll.ms).toBe(POLL_MS);
    poll.fn();
    expect(h.events.at(-1)).toEqual({ type: "poll" });
    h.fire((t) => !t.every);
    h.socks[2]!.send(hello);
    expect(h.c.status).toBe("live");
    expect(poll.dead).toBe(true);
  });

  it("4401 ends the stream without reconnecting and tells the app the session ended", () => {
    const h = harness();
    h.c.start();
    h.socks[0]!.send(hello);
    h.socks[0]!.drop(4401);
    expect(h.c.status).toBe("closed");
    expect(h.ended()).toBe(1);
    expect(h.timers.filter((t) => !t.every && !t.dead)).toHaveLength(0);
  });

  it("4403 stays usable through polling and does not reconnect", () => {
    const h = harness();
    h.c.start();
    h.socks[0]!.send(hello);
    h.socks[0]!.drop(4403);
    expect(h.c.status).toBe("polling");
    expect(h.timers.filter((t) => !t.every && !t.dead)).toHaveLength(0);
    expect(h.timers.some((t) => t.every && !t.dead)).toBe(true);
  });

  it("surfaces auth_expiring and handles offline/online", () => {
    const h = harness();
    h.c.start();
    h.socks[0]!.send(hello);
    h.socks[0]!.send({ type: "auth_expiring", expires_at: "2026-11-19T11:59:00Z" });
    expect(h.c.authExpiringAt).toBe("2026-11-19T11:59:00Z");
    h.c.offline();
    expect(h.c.status).toBe("offline");
    expect(h.socks[0]!.closed).toBe(1000);
    h.c.online();
    expect(h.socks).toHaveLength(2);
    expect(h.events.at(-1)).toEqual({ type: "resync", reason: "online" });
  });

  it("stop closes the socket and a late close does not reconnect", () => {
    const h = harness();
    h.c.start();
    h.c.stop();
    h.socks[0]!.drop(1006);
    expect(h.timers.filter((t) => !t.dead)).toHaveLength(0);
  });

  it("backoff doubles, is jittered and capped", () => {
    expect(backoff(1, 1)).toBe(2000);
    expect(backoff(2, 0)).toBe(2000);
    expect(backoff(20, 1)).toBe(BACKOFF_CAP_MS);
    expect(backoff(20, 0)).toBe(BACKOFF_CAP_MS / 2);
  });
});
