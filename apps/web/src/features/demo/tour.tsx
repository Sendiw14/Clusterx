"use client";
import { useEffect, useMemo, useState } from "react";
import { ForceUtcContext, Icon, Notice, Segmented } from "@clusterx/ui";
import { DataProviderContext } from "@/data/context";
import { FixtureProvider, type FixtureView } from "@/data/fixture-provider";
import { JobDetailView } from "@/features/job-detail";
import { JobsListView, type JobFilters } from "@/features/jobs-list";
import { OverviewView } from "@/features/overview";
import { WorkersListView } from "@/features/workers";
import { ConfigView } from "@/features/config";
import { RecoveryReplay } from "@/features/demo/replay";

type Screen = "overview" | "jobs" | "workers" | "recovery" | "config";

/**
 * Read-only console tour on fixtures. It imports only the fixture provider (ESLint boundary + import-graph test).
 * No mutation controls exist: where the real console has Cancel, Retry or Drain, a static note says so.
 */
export function DemoTour() {
  const [view, setView] = useState<FixtureView>("operator");
  const [screen, setScreen] = useState<Screen>("overview");
  const [jobId, setJobId] = useState<string | null>(null);
  const [filters, setFilters] = useState<JobFilters>({});
  const provider = useMemo(() => new FixtureProvider(view), [view]);
  const [retriedAs, setRetriedAs] = useState<Record<string, string>>({});
  useEffect(() => {
    let live = true;
    provider.listJobs({}).then((r) => {
      if (!live || !r.ok) return;
      const m: Record<string, string> = {};
      for (const j of r.data.items) if (j.retried_from_job_id) m[j.retried_from_job_id] = j.id;
      setRetriedAs(m);
    });
    return () => { live = false; };
  }, [provider]);

  const jobLink = (j: { id: string }) => (
    <button type="button" className="link-btn mono" onClick={() => { setJobId(j.id); setScreen("jobs"); }}>{j.id}</button>
  );

  return (
    <ForceUtcContext value>
    <DataProviderContext provider={provider}>
      <div className="toolbar" style={{ justifyContent: "space-between" }}>
        <Segmented label="Demo screen" value={screen} onChange={(v) => { setScreen(v as Screen); setJobId(null); }} options={[
          { value: "overview", label: "Overview" }, { value: "jobs", label: "Jobs" }, { value: "workers", label: "Workers" }, { value: "recovery", label: "Recovery replay" }, { value: "config", label: "Config" },
        ]} />
        <fieldset className="seg-fieldset" style={{ border: 0, padding: 0, margin: 0 }}>
          <legend className="sr-only">View as</legend>
          <span className="seg" role="radiogroup" aria-label="View as">
            {(["operator", "viewer"] as const).map((v) => (
              <button key={v} type="button" role="radio" aria-checked={view === v} aria-pressed={undefined} onClick={() => setView(v)} data-on={view === v ? "1" : undefined}>
                <Icon name={v === "viewer" ? "i-lock" : "i-check"} />View as {v}
              </button>
            ))}
          </span>
        </fieldset>
      </div>
      {view === "viewer" ? (
        <Notice variant="warning" icon="i-lock" lead="Viewing as a read-only viewer." role="status">Payloads, results, error text and log tails are removed from the data, not just hidden. This is what the console shows a viewer.</Notice>
      ) : null}

      <section aria-labelledby="demo-screen-h" style={{ display: "grid", gap: "var(--space-4)", gridTemplateColumns: "minmax(0, 1fr)" }}>
        {screen === "overview" ? (
          <>
            <h2 id="demo-screen-h" className="sr-only">Demo: Overview</h2>
            <OverviewView jobLink={jobLink} />
          </>
        ) : null}
        {screen === "jobs" && !jobId ? (
          <>
            <h2 id="demo-screen-h" className="sr-only">Demo: Jobs</h2>
            <JobsListView filters={filters} onFilters={setFilters} jobLink={jobLink} headingLevel={3} />
          </>
        ) : null}
        {screen === "jobs" && jobId ? (
          <>
            <h2 id="demo-screen-h" className="sr-only">Demo: Job detail</h2>
            <div><button type="button" className="btn btn-sm" onClick={() => setJobId(null)}><Icon name="i-chev-l" />Back to jobs</button></div>
            <JobDetailView id={jobId} jobLink={jobLink} retriedAs={retriedAs[jobId] ?? null} />
            <p className="meta">Operators can cancel a running job or retry a failed one here. Those actions are not available in the demo.</p>
          </>
        ) : null}
        {screen === "workers" ? (
          <>
            <h2 id="demo-screen-h" className="sr-only">Demo: Workers</h2>
            <WorkersListView workerLink={(w) => <span className="id">{w.name}</span>} />
            <p className="meta">Operators can drain a worker here. That action is not available in the demo.</p>
          </>
        ) : null}
        {screen === "recovery" ? (
          <>
            <h2 id="demo-screen-h" className="sr-only">Demo: Recovery replay</h2>
            <RecoveryReplay />
          </>
        ) : null}
        {screen === "config" ? (
          <>
            <h2 id="demo-screen-h" className="sr-only">Demo: Configuration</h2>
            <ConfigView />
          </>
        ) : null}
      </section>
    </DataProviderContext>
    </ForceUtcContext>
  );
}
