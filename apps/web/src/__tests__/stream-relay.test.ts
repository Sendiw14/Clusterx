import { describe, expect, it } from "vitest";
import { MAX_SOCKETS_PER_SESSION, MAX_SOCKETS_TOTAL, reserveSlot, slotsInUse } from "@/server/stream-relay";

describe("reserveSlot (RV6-08)", () => {
  it("reserves synchronously, so concurrent upgrades cannot pass a check-then-act race", () => {
    const got = Array.from({ length: 8 }, () => reserveSlot("sid-a"));
    expect(got.filter(Boolean)).toHaveLength(MAX_SOCKETS_PER_SESSION);
    got.forEach((r) => r?.());
  });
  it("release is idempotent and frees exactly one slot", () => {
    const base = slotsInUse();
    const r = reserveSlot("sid-b")!;
    expect(slotsInUse()).toBe(base + 1);
    r();
    r();
    expect(slotsInUse()).toBe(base);
  });
  it("has a per-process ceiling across sessions", () => {
    const held: (() => void)[] = [];
    for (let i = 0; i < MAX_SOCKETS_TOTAL * 2; i++) {
      const r = reserveSlot(`sid-many-${Math.floor(i / MAX_SOCKETS_PER_SESSION)}`);
      if (r) held.push(r);
    }
    expect(held).toHaveLength(MAX_SOCKETS_TOTAL - slotsInUse() + held.length);
    expect(slotsInUse()).toBe(MAX_SOCKETS_TOTAL);
    expect(reserveSlot("sid-extra")).toBeNull();
    held.forEach((r) => r());
    expect(slotsInUse()).toBe(0);
  });
});
