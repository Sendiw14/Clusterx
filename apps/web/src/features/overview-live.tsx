"use client";
import Link from "next/link";
import { Region, StatusBadge, Timestamp } from "@clusterx/ui";
import { useConsoleProvider } from "@/data/context";
import type { MetricsOverview, Worker } from "@/data/provider";
import { SeriesChart } from "./chart";
import { Load } from "./common";
import { useResource } from "./use-resource";
import { workerState } from "./workers";

const ANY = () => true;

/** Last hour of completed 5-minute buckets (the API returns only complete buckets). */
function useThroughput() {
  const p = useConsoleProvider();
  return useResource(async () => {
    const to = new Date();
    const from = new Date(to.getTime() - 60 * 60_000);
    const q = { from: from.toISOString(), to: to.toISOString(), step: "5m" as const };
    const [a, s, f] = await Promise.all([p.getTimeseries({ metric: "jobs_admitted", ...q }), p.getTimeseries({ metric: "jobs_succeeded", ...q }), p.getTimeseries({ metric: "jobs_failed", ...q })]);
    if (!a.ok) return a;
    if (!s.ok) return s;
    if (!f.ok) return f;
    return { ok: true as const, data: { admitted: a.data, succeeded: s.data, failed: f.data }, requestId: a.requestId };
  }, "throughput", ANY);
}

export function ThroughputPanel() {
  const res = useThroughput();
  return (
    <Region title="Throughput, last hour" id="throughput">
      <Load res={res} rows={3} label="Loading throughput" errorHeading="Throughput could not be loaded">
        {(d) => (
          <>
            <SeriesChart
              title="Jobs per 5 minutes"
              stepLabel="5-minute"
              stale={res.refreshing}
              series={[
                { label: "Admitted", color: "var(--color-ink-2)", points: d.admitted.points },
                { label: "Succeeded", color: "var(--color-success)", points: d.succeeded.points },
                { label: "Failed", color: "var(--color-danger)", points: d.failed.points },
              ]}
            />
            <p className="meta" style={{ padding: "0 var(--space-4) var(--space-3)" }}>Complete buckets only, as of <Timestamp value={d.admitted.as_of} />. The newest open bucket appears when it closes.</p>
          </>
        )}
      </Load>
    </Region>
  );
}

function Topology({ o, workers }: { o: MetricsOverview; workers: Worker[] }) {
  const q = o.jobs_by_status;
  const text = `Control plane holds ${q.scheduled} scheduled, ${q.queued} queued and ${q.retry_wait} retry-wait jobs. Workers: ${o.workers.live} live, ${o.workers.stale} stale, ${o.workers.draining} draining. ${q.running} jobs are running.`;
  const node = (x: number, title: string, sub: string, state?: string) => (
    <g className="node" data-live={state} transform={`translate(${x} 20)`}>
      <rect className="body" width="170" height="64" rx="6" />
      <text x="12" y="26" fontWeight="600">{title}</text>
      <text className="sub" x="12" y="46">{sub}</text>
    </g>
  );
  return (
    <div className="topo-view" style={{ minHeight: 0 }}>
      <svg viewBox="0 0 640 104" role="img" aria-labelledby="topo-t topo-d">
        <title id="topo-t">Topology: control plane and workers</title>
        <desc id="topo-d">{text}</desc>
        {node(10, "Control plane", `${q.queued + q.scheduled + q.retry_wait} waiting`)}
        <line x1="180" x2="235" y1="52" y2="52" stroke="var(--color-line-strong)" strokeWidth="1.5" />
        {node(235, "Running", `${q.running} attempts hold a lease`)}
        <line x1="405" x2="460" y1="52" y2="52" stroke="var(--color-line-strong)" strokeWidth="1.5" />
        {node(460, "Workers", `${o.workers.live} live · ${o.workers.stale} stale · ${o.workers.draining} draining`, o.workers.stale > 0 && o.workers.live === 0 ? "stale" : undefined)}
      </svg>
      {workers.length ? null : <p className="meta" style={{ padding: "0 var(--space-4) var(--space-3)" }}>No workers have registered.</p>}
    </div>
  );
}

/** Accessible topology: the SVG has a title and description, and the list below it carries the same facts per worker. */
export function TopologyPanel() {
  const p = useConsoleProvider();
  const ov = useResource(() => p.getOverview(), "topo-ov", ANY);
  const wk = useResource(() => p.listWorkers({ limit: 20 }), "topo-wk", ANY);
  return (
    <Region title="Topology" id="topology">
      <Load res={ov} rows={2} label="Loading topology" errorHeading="Topology could not be loaded">
        {(o) => (
          <>
            <Topology o={o} workers={wk.state === "ok" ? wk.data.items : []} />
            <Load res={wk} rows={2} label="Loading workers" errorHeading="Workers could not be loaded">
              {(w) => (w.items.length ? (
                <ul aria-label="Workers" style={{ listStyle: "none", padding: "var(--space-3) var(--space-4)", display: "grid", gap: "var(--space-2)" }}>
                  {w.items.map((x) => (
                    <li key={x.id}><Link href={`/app/workers/${x.id}`}>{x.name}</Link> <StatusBadge state={workerState(x)} /> <span className="meta">{x.active_attempts} of {x.capacity} slots busy</span></li>
                  ))}
                </ul>
              ) : null)}
            </Load>
          </>
        )}
      </Load>
    </Region>
  );
}
