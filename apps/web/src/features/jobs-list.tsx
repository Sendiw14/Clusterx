"use client";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Button, DataTable, EmptyState, FilterChip, Icon, Pagination, ReasonChip, Select, StatusBadge, TextInput, Timestamp, JOB_STATES, Notice, useOnline, type Column } from "@clusterx/ui";
import { useDataProvider } from "@/data/context";
import { useRealtime } from "@/data/realtime";
import type { JobQuery, JobSort, JobStatus, JobSummary, UnschedulableReason } from "@/data/provider";
import { Load, reasonLabel } from "./common";
import { useResource } from "./use-resource";

export interface JobFilters { status?: JobStatus; handler?: string; priorityMin?: number; reason?: UnschedulableReason; sort?: JobSort }

export const SORTS: { value: JobSort; label: string }[] = [
  { value: "-created_at", label: "Newest first" },
  { value: "created_at", label: "Oldest first" },
  { value: "-available_at", label: "Next eligible, latest first" },
  { value: "available_at", label: "Next eligible, soonest first" },
  { value: "-priority", label: "Claim order (priority)" },
];
export const REASONS: { value: UnschedulableReason; label: string }[] = [
  { value: "no_capable_worker", label: "No capable worker" },
  { value: "capable_workers_offline", label: "Capable workers offline" },
  { value: "capable_workers_draining", label: "Capable workers draining" },
];

const NEVER = () => false;
const STATUS_LABEL: Record<JobStatus, string> = { scheduled: "Scheduled", queued: "Queued", running: "Running", retry_wait: "Retry wait", succeeded: "Succeeded", failed: "Failed", canceled: "Canceled" };

export function filtersToQuery(f: JobFilters): JobQuery {
  return { status: f.status ? [f.status] : undefined, handler_name: f.handler || undefined, priority_min: f.priorityMin, unschedulable_reason: f.reason ? [f.reason] : undefined, sort: f.sort };
}

/**
 * Jobs list view shared by the console (live provider, URL-synced filters) and the demo (fixtures, local state).
 * Server-filtered and cursor-paginated through the provider; Previous uses a client-kept cursor stack.
 */
export function JobsListView({ filters, onFilters, jobLink, headingLevel = 2 }: { filters: JobFilters; onFilters: (f: JobFilters) => void; jobLink: (j: JobSummary) => ReactNode; headingLevel?: 2 | 3 }) {
  const provider = useDataProvider();
  const online = useOnline();
  const [stack, setStack] = useState<string[]>([]);
  const cursor = stack.at(-1);
  const key = JSON.stringify([filters, cursor, provider.kind]);
  const res = useResource(() => provider.listJobs({ ...filtersToQuery(filters), cursor, limit: 50 }), key, NEVER);
  const H = `h${headingLevel}` as "h2" | "h3";
  const live = provider.kind === "live";
  // Rows never reorder under the reader: job hints only flag that a refresh is worth it. Resync and polling refetch.
  const rt = useRealtime();
  const [dirty, setDirty] = useState(false);
  useEffect(() => rt?.subscribe((e) => { if (e.type === "hint" && e.entity === "job") setDirty(true); }), [rt]);

  const active = useMemo(() => {
    const chips: { id: string; label: string; remove: () => void }[] = [];
    if (filters.status) chips.push({ id: "status", label: `Status: ${STATUS_LABEL[filters.status]}`, remove: () => change({ ...filters, status: undefined }) });
    if (filters.handler) chips.push({ id: "handler", label: `Handler: ${filters.handler}`, remove: () => change({ ...filters, handler: undefined }) });
    if (filters.reason) chips.push({ id: "reason", label: `Reason: ${REASONS.find((r) => r.value === filters.reason)?.label ?? filters.reason}`, remove: () => change({ ...filters, reason: undefined }) });
    if (filters.priorityMin !== undefined) chips.push({ id: "priority", label: `Priority ${filters.priorityMin} or higher`, remove: () => change({ ...filters, priorityMin: undefined }) });
    return chips;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters]);

  function change(f: JobFilters) {
    setStack([]);
    onFilters(f);
  }

  const columns: Column<JobSummary>[] = [
    { key: "id", header: "Job", primary: true, render: (j) => <>{jobLink(j)} <StatusBadge state={j.status} />{j.unschedulable_reason ? <> <ReasonChip>{reasonLabel(j.unschedulable_reason)}</ReasonChip></> : null}</> },
    { key: "handler", header: "Handler", render: (j) => <span className="mono">{j.handler_name}@{j.handler_version}</span> },
    { key: "priority", header: "Priority", numeric: true, render: (j) => j.priority },
    { key: "created", header: "Created", render: (j) => <Timestamp value={j.created_at} /> },
    { key: "attempts", header: "Attempts", numeric: true, render: (j) => `${j.attempts_used} of ${j.max_attempts}` },
  ];

  return (
    <>
      <section className="card" aria-labelledby="jf-h">
        <div className="fbar">
          <H id="jf-h" className="sr-only">Filters</H>
          <form className="filters" role="search" aria-label="Filter jobs" onSubmit={(e) => e.preventDefault()}>
            <Select label="Status" value={filters.status ?? ""} onChange={(e) => change({ ...filters, status: (e.target.value || undefined) as JobStatus | undefined })}>
              <option value="">All states</option>
              {JOB_STATES.map((s) => <option key={s} value={s}>{STATUS_LABEL[s]}</option>)}
            </Select>
            <TextInput label="Handler" placeholder="exact name" value={filters.handler ?? ""} onChange={(e) => change({ ...filters, handler: e.target.value || undefined })} autoComplete="off" spellCheck={false} />
            <Select label="Priority at least" help="9 is highest" value={filters.priorityMin ?? ""} onChange={(e) => change({ ...filters, priorityMin: e.target.value === "" ? undefined : Number(e.target.value) })}>
              <option value="">0 (any)</option>
              {[1, 2, 3, 4, 5, 6, 7, 8, 9].map((p) => <option key={p} value={p}>{p === 9 ? "9 (highest)" : `${p} or higher`}</option>)}
            </Select>
            {live ? (
              <Select label="Unschedulable reason" help="Queued jobs no worker can take" value={filters.reason ?? ""} onChange={(e) => change({ ...filters, reason: (e.target.value || undefined) as UnschedulableReason | undefined })}>
                <option value="">Any</option>
                {REASONS.map((r) => <option key={r.value} value={r.value}>{r.label}</option>)}
              </Select>
            ) : null}
            {live ? (
              <Select label="Sort" value={filters.sort ?? "-created_at"} onChange={(e) => change({ ...filters, sort: e.target.value === "-created_at" ? undefined : (e.target.value as JobSort) })}>
                {SORTS.map((r) => <option key={r.value} value={r.value}>{r.label}</option>)}
              </Select>
            ) : null}
            <div className="field">
              <span className="label">Dead-letter view</span>
              <button type="button" className="btn" aria-pressed={filters.status === "failed"} onClick={() => change({ ...filters, status: filters.status === "failed" ? undefined : "failed" })}>
                <Icon name="g-failed" />Failed (dead-letter)
              </button>
            </div>
          </form>
          {active.length ? (
            <div className="chips" aria-label="Active filters" style={{ marginTop: "var(--space-3)" }}>
              {active.map((c) => <FilterChip key={c.id} label={c.label} onRemove={c.remove} />)}
              {active.length > 1 ? <Button size="sm" variant="ghost" onClick={() => change({})}>Clear all filters</Button> : null}
            </div>
          ) : null}
        </div>
      </section>

      <section className="card sect" aria-labelledby="jl-h" style={{ marginTop: "var(--space-4)" }}>
        <H id="jl-h" className="sr-only">Jobs list</H>
        {dirty ? <div style={{ padding: "var(--space-3) var(--space-4)" }}><Notice variant="info" lead="Jobs changed." actions={<Button size="sm" onClick={() => { setDirty(false); res.reload(); }}>Refresh list</Button>}>The list keeps its order until you refresh it.</Notice></div> : null}
        {!online ? <div style={{ padding: "var(--space-4)" }}><Notice variant="warning" icon="i-signal-off" lead="You are offline.">Showing the last page you loaded. Filters need a connection.</Notice></div> : null}
        <Load res={res} rows={5} label="Loading jobs" errorHeading="Jobs could not be loaded">
          {(page) =>
            page.items.length === 0 ? (
              active.length ? (
                <EmptyState icon="i-filter" title="No jobs match these filters" actions={<Button onClick={() => change({})}>Clear filters</Button>}>
                  Widen the filters or clear one. Dead-letter is the failed view, so an empty failed list is good news.
                </EmptyState>
              ) : (
                <EmptyState title="No jobs yet">Jobs appear here after something submits one. Each shows its attempts, lease and outcome.</EmptyState>
              )
            ) : (
              <>
                <DataTable
                  busy={res.refreshing}
                  caption={<>Jobs, {page.items.length} shown · as of <Timestamp value={page.as_of} frozen={!online} /></>}
                  columns={columns}
                  rows={page.items}
                  rowKey={(j) => j.id}
                />
                <Pagination
                  hasPrev={stack.length > 0}
                  hasNext={Boolean(page.next_cursor)}
                  onPrev={() => setStack((s) => s.slice(0, -1))}
                  onNext={() => page.next_cursor && setStack((s) => [...s, page.next_cursor as string])}
                  note={`50 per page · ${SORTS.find((x) => x.value === (filters.sort ?? "-created_at"))?.label.toLowerCase()}. Reordering happens only when you refresh.`}
                />
              </>
            )
          }
        </Load>
      </section>
      <p className="meta" style={{ marginTop: "var(--space-3)" }}>Dead-letter is a view of jobs whose status is failed, not a separate state. Unschedulable is a reason shown on queued jobs, not a status. There are no bulk actions.</p>
    </>
  );
}
