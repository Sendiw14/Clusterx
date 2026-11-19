"use client";
import type { ReactNode } from "react";
import { Icon, KpiList, Region, StatusBadge, Timestamp, DataTable, ReasonChip, type Column } from "@clusterx/ui";
import { useDataProvider } from "@/data/context";
import type { JobSummary, MetricsOverview } from "@/data/provider";
import { Load, reasonLabel } from "./common";
import { useResource } from "./use-resource";

/** Counts and recent jobs follow every job or metrics hint (refetch only; hints carry no data). */
const ANY = () => true;
const age = (s: number | null) => (s === null ? "None waiting" : s < 90 ? `${s} s` : `${Math.round(s / 60)} min`);

/**
 * Overview regions load independently: one failing region does not blank the page. Facts only; the UI invents no
 * thresholds, severities or alert states (CCR-08 deferred). Chart and topology arrive in M6.
 */
export function OverviewView({ jobLink, jobsHref, extra }: { jobLink: (j: JobSummary) => ReactNode; jobsHref?: string; extra?: ReactNode }) {
  const provider = useDataProvider();
  const overview = useResource(() => provider.getOverview(), `ov-${provider.kind}`, ANY);
  const recent = useResource(() => provider.listJobs({ limit: 5 }), `recent-${provider.kind}`, ANY);

  const columns: Column<JobSummary>[] = [
    { key: "id", header: "Job", primary: true, render: (j) => <>{jobLink(j)} <StatusBadge state={j.status} />{j.unschedulable_reason ? <> <ReasonChip>{reasonLabel(j.unschedulable_reason)}</ReasonChip></> : null}</> },
    { key: "handler", header: "Handler", render: (j) => <span className="mono">{j.handler_name}@{j.handler_version}</span> },
    { key: "created", header: "Created", render: (j) => <Timestamp value={j.created_at} /> },
  ];

  return (
    <>
      <Region title="Queue and workers" id="kpis">
        <div style={{ padding: "var(--space-4)" }}>
          <Load res={overview} rows={2} label="Loading overview" errorHeading="The overview could not be loaded">
            {(o: MetricsOverview) => {
              const w = o.terminal_counts_window.days;
              return (
                <>
                  <KpiList
                    label="Jobs and workers"
                    items={[
                      { label: "Scheduled", value: o.jobs_by_status.scheduled },
                      { label: "Queued", value: o.jobs_by_status.queued },
                      { label: "Retry wait", value: o.jobs_by_status.retry_wait },
                      { label: "Running", value: o.jobs_by_status.running },
                      { label: "Oldest queued", value: age(o.oldest_queued_age_seconds) },
                      { label: "Unschedulable", value: o.unschedulable_jobs, note: o.unschedulable_jobs ? <>Waiting with no eligible worker</> : undefined },
                      { label: `Succeeded (${w} days)`, value: o.jobs_by_status.succeeded },
                      { label: `Failed (${w} days)`, value: o.jobs_by_status.failed },
                      { label: `Canceled (${w} days)`, value: o.jobs_by_status.canceled },
                      { label: "Workers live", value: o.workers.live, icon: <Icon name="g-online" /> },
                      { label: "Workers stale", value: o.workers.stale, icon: <Icon name="g-stale" /> },
                      { label: "Workers draining", value: o.workers.draining, icon: <Icon name="g-draining" /> },
                    ]}
                  />
                  <p className="meta" style={{ marginTop: "var(--space-3)" }}>As of <Timestamp value={o.as_of} />. Terminal counts cover the {w}-day retention window.</p>
                </>
              );
            }}
          </Load>
        </div>
      </Region>
      {extra}
      <Region title="Recent jobs" id="recent">
        <Load res={recent} rows={4} label="Loading recent jobs" errorHeading="Recent jobs could not be loaded">
          {(p) => (p.items.length === 0 ? <p style={{ padding: "var(--space-4)" }} className="muted">No jobs yet.</p> : <DataTable caption={<>Latest {p.items.length} jobs · as of <Timestamp value={p.as_of} /></>} columns={columns} rows={p.items} rowKey={(j) => j.id} />)}
        </Load>
        {jobsHref ? <p style={{ padding: "var(--space-3) var(--space-4)" }}><a href={jobsHref}>All jobs</a></p> : null}
      </Region>
    </>
  );
}
