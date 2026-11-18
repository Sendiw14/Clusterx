import { beforeEach, describe, expect, it, vi } from "vitest";

describe("preferences storage", () => {
  beforeEach(() => { vi.resetModules(); document.documentElement.removeAttribute("data-rm"); });

  it("blocked or throwing storage degrades to defaults and never throws", async () => {
    const boom = { getItem: () => { throw new Error("blocked"); }, setItem: () => { throw new Error("blocked"); } };
    vi.stubGlobal("localStorage", boom);
    Object.defineProperty(window, "localStorage", { value: boom, configurable: true });
    const { getPrefs, setPrefs } = await import("@clusterx/ui");
    expect(getPrefs()).toEqual({ timezone: "browser", reduceMotion: "system" });
    expect(() => setPrefs({ reduceMotion: "always" })).not.toThrow();
    expect(document.documentElement.getAttribute("data-rm")).toBe("1");
    expect(getPrefs().reduceMotion).toBe("always");
    vi.unstubAllGlobals();
  });
  it("rejects malformed stored values", async () => {
    const store: Record<string, string> = { "clusterx.prefs.v1": JSON.stringify({ timezone: 42, reduceMotion: "never-heard-of-it" }) };
    const ls = { getItem: (k: string) => store[k] ?? null, setItem: (k: string, v: string) => { store[k] = v; } };
    Object.defineProperty(window, "localStorage", { value: ls, configurable: true });
    const { getPrefs } = await import("@clusterx/ui");
    expect(getPrefs()).toEqual({ timezone: "browser", reduceMotion: "system" });
  });
});
