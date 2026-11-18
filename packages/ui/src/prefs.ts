"use client";
import { useEffect, useSyncExternalStore } from "react";

/** Per-viewer conveniences only (zone, reduced motion). Every access is wrapped: storage may throw or be blocked. */
const KEY = "clusterx.prefs.v1";
export interface Prefs {
  /** IANA zone, or "browser" to follow the browser. */
  timezone: string;
  /** "system" follows the OS; "always" can only add reduction, never force motion on. */
  reduceMotion: "system" | "always";
}
export const DEFAULT_PREFS: Prefs = { timezone: "browser", reduceMotion: "system" };

let cache: Prefs | null = null;
const listeners = new Set<() => void>();

function read(): Prefs {
  try {
    const raw = window.localStorage.getItem(KEY);
    if (!raw) return DEFAULT_PREFS;
    const j = JSON.parse(raw) as Partial<Prefs>;
    return {
      timezone: typeof j.timezone === "string" && j.timezone.length < 64 ? j.timezone : "browser",
      reduceMotion: j.reduceMotion === "always" ? "always" : "system",
    };
  } catch {
    return DEFAULT_PREFS;
  }
}

export function applyMotionPref(p: Prefs): void {
  if (typeof document === "undefined") return;
  if (p.reduceMotion === "always") document.documentElement.setAttribute("data-rm", "1");
  else document.documentElement.removeAttribute("data-rm");
}

export function getPrefs(): Prefs {
  if (typeof window === "undefined") return DEFAULT_PREFS;
  cache ??= read();
  return cache;
}

export function setPrefs(next: Partial<Prefs>): void {
  const merged = { ...getPrefs(), ...next };
  cache = merged;
  try {
    window.localStorage.setItem(KEY, JSON.stringify(merged));
  } catch {
    /* storage blocked: the preference lasts for this page view only */
  }
  applyMotionPref(merged);
  listeners.forEach((l) => l());
}

function subscribe(cb: () => void): () => void {
  listeners.add(cb);
  const onStorage = (e: StorageEvent) => {
    if (e.key === KEY) {
      cache = null;
      cb();
    }
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(cb);
    window.removeEventListener("storage", onStorage);
  };
}

export function usePrefs(): Prefs {
  return useSyncExternalStore(subscribe, getPrefs, () => DEFAULT_PREFS);
}

/** Applies stored motion preference at startup (rendered once in the root layout). */
export function PrefsBoot() {
  useEffect(() => {
    applyMotionPref(getPrefs());
  }, []);
  return null;
}

export function browserZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}
