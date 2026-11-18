"use client";
import { browserZone, usePrefs } from "./prefs";

function fmt(d: Date, zone: string): string {
  try {
    return new Intl.DateTimeFormat("en-GB", {
      timeZone: zone,
      year: "numeric",
      month: "short",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hourCycle: "h23",
      timeZoneName: "short",
    }).format(d);
  } catch {
    return d.toISOString();
  }
}

/**
 * `<time datetime=UTC ISO>` with user-local text and an explicit zone. The title carries UTC. Server render and first
 * client render use UTC (no hydration mismatch); the zone preference applies after hydration. `frozen` marks a value
 * that must not be shown as current ("as of").
 */
export function Timestamp({ value, utc, frozen }: { value: string | Date | null | undefined; utc?: boolean; frozen?: boolean }) {
  const prefs = usePrefs();
  if (!value) return <span className="meta">Not set</span>;
  const d = typeof value === "string" ? new Date(value) : value;
  if (Number.isNaN(d.getTime())) return <span className="meta">Invalid time</span>;
  const zone = utc ? "UTC" : prefs.timezone === "browser" ? browserZone() : prefs.timezone;
  return (
    <time dateTime={d.toISOString()} title={`${d.toISOString()} (UTC)`} suppressHydrationWarning data-frozen={frozen ? "true" : undefined}>
      {fmt(d, zone)}
    </time>
  );
}
