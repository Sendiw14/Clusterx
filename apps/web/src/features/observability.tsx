"use client";
import { useState } from "react";
import { Notice, Select, Timestamp } from "@clusterx/ui";
import { useConsoleProvider } from "@/data/context";
import type { Metric, Series } from "@/data/console-provider";
import { SeriesChart } from "./chart";
import { Load } from "./common";
import { useResource } from "./use-resource";

const RANGES = {
  "1h": { label: "Last hour", ms: 3_600_000, step: "1m" as const, stepLabel: "1-minute" },
  "6h": { label: "Last 6 hours", ms: 6 * 3_600_000, step: "5m" as const, stepLabel: "5-minute" },
  "24h": { label: "Last 24 hours", ms: 24 * 3_600_000, step: "5m" as const, stepLabel: "5-minute" },
  "7d": { label: "Last 7 days", ms: 7 * 24 * 3_600_000, step: "1h" as const, stepLabel: "1-hour" },
};
type RangeKey = keyof typeof RANGES;
const METRICS: Metric[] = ["jobs_admitted", "jobs_succeeded", "jobs_failed", "jobs_canceled", "attempts_started", "lease_expirations", "queue_age_p95_seconds", "run_duration_p95_seconds"];
const ANY = () => true;

/** All eight real metrics (GAP-OBS-01: no invented series). Null buckets are gaps, never zero. */
export function ObservabilityView() {
  const p = useConsoleProvider();
  const [range, setRange] = useState<RangeKey>("6h");
  const r = RANGES[range];
  const res = useResource(async () => {
    const to = new Date();
    const from = new Date(to.getTime() - r.ms);
    const q = { from: from.toISOString(), to: to.toISOString(), step: r.step };
    const all = await Promise.all(METRICS.map((metric) => p.getTimeseries({ metric, ...q })));
    const bad = all.find((x) => !x.ok);
    if (bad && !bad.ok) return bad;
    const by = Object.fromEntries(METRICS.map((m, i) => [m, (all[i] as { ok: true; data: Series }).data])) as Record<Metric, Series>;
    return { ok: true as const, data: by };
  }, `obs-${range}`, ANY);
  const pts = (m: Metric, d: Record<Metric, Series>) => d[m].points;

  return (
    <>
      <div className="fbar card" style={{ marginBottom: "var(--space-4)" }}>
        <div className="filters">
          <Select label="Time range" value={range} onChange={(e) => setRange(e.target.value as RangeKey)}>
            {(Object.keys(RANGES) as RangeKey[]).map((k) => <option key={k} value={k}>{RANGES[k].label}</option>)}
          </Select>
        </div>
      </div>
      <Load res={res} rows={6} label="Loading metrics" errorHeading="Metrics could not be loaded">
        {(d) => (
          <div style={{ display: "grid", gap: "var(--space-4)" }}>
            <Notice variant="info" lead="Complete buckets only." role="none">
              {r.stepLabel} buckets, as of <Timestamp value={d.jobs_admitted.as_of} />. A bucket appears once it has closed, so the newest minutes are not shown yet. Percentile series show gaps where no attempt ran.
            </Notice>
            <section className="card" aria-label="Throughput">
              <SeriesChart title="Jobs by outcome" stepLabel={r.stepLabel} stale={res.refreshing} series={[
                { label: "Admitted", color: "var(--color-ink-2)", points: pts("jobs_admitted", d) },
                { label: "Succeeded", color: "var(--color-success)", points: pts("jobs_succeeded", d) },
                { label: "Failed", color: "var(--color-danger)", points: pts("jobs_failed", d) },
                { label: "Canceled", color: "var(--color-warning)", points: pts("jobs_canceled", d) },
              ]} />
            </section>
            <section className="card" aria-label="Attempts">
              <SeriesChart title="Attempts started and lease expirations" stepLabel={r.stepLabel} stale={res.refreshing} series={[
                { label: "Attempts started", color: "var(--color-ink-2)", points: pts("attempts_started", d) },
                { label: "Lease expirations", color: "var(--color-danger)", points: pts("lease_expirations", d) },
              ]} />
            </section>
            <section className="card" aria-label="Queue age">
              <SeriesChart title="Queue age, 95th percentile" unit="s" stepLabel={r.stepLabel} stale={res.refreshing} series={[{ label: "Queue age p95", color: "var(--color-ink-2)", points: pts("queue_age_p95_seconds", d) }]} />
            </section>
            <section className="card" aria-label="Run duration">
              <SeriesChart title="Run duration, 95th percentile" unit="s" stepLabel={r.stepLabel} stale={res.refreshing} series={[{ label: "Run duration p95", color: "var(--color-ink-2)", points: pts("run_duration_p95_seconds", d) }]} />
            </section>
          </div>
        )}
      </Load>
    </>
  );
}
