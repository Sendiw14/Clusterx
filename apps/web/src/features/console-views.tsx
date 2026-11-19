"use client";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Card, DefinitionList, Select, Timestamp, browserZone, setPrefs, usePrefs, useToast } from "@clusterx/ui";
import type { JobSort, JobStatus, JobSummary, UnschedulableReason, Worker } from "@/data/provider";
import { ActivityView } from "./activity";
import { ConfigView } from "./config";
import { JobNewView } from "./job-new";
import { ObservabilityView } from "./observability";
import { TokensView } from "./tokens";
import { JobDetailView } from "./job-detail";
import { JobsListView, REASONS, SORTS, type JobFilters } from "./jobs-list";
import { OverviewView } from "./overview";
import { ThroughputPanel, TopologyPanel } from "./overview-live";
import { WorkerDetailView, WorkersListView } from "./workers";

const REASON_KEYS: string[] = REASONS.map((r) => r.value);
const SORT_KEYS: string[] = SORTS.map((r) => r.value);
const STATUSES = ["scheduled", "queued", "running", "retry_wait", "succeeded", "failed", "canceled"];

const jobLink = (j: JobSummary) => <Link className="id mono" href={`/app/jobs/${j.id}`}>{j.id.slice(0, 8)}</Link>;
const workerLink = (w: Worker) => <Link className="id" href={`/app/workers/${w.id}`}>{w.name}</Link>;

export function OverviewConsole() {
  return <OverviewView jobLink={jobLink} jobsHref="/app/jobs" extra={<><ThroughputPanel /><TopologyPanel /></>} />;
}

/** URL is the filter state: reload, back and shared links reproduce the view. No payload data ever goes in a URL. */
export function JobsConsole() {
  const sp = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const filters = useMemo<JobFilters>(() => {
    const s = sp.get("status");
    const raw = sp.get("priority_min");
    const p = Number(raw);
    return {
      status: s && STATUSES.includes(s) ? (s as JobStatus) : undefined,
      handler: sp.get("handler_name")?.slice(0, 128) || undefined,
      priorityMin: raw !== null && Number.isInteger(p) && p >= 0 && p <= 9 ? p : undefined,
      reason: REASON_KEYS.includes(sp.get("reason") ?? "") ? (sp.get("reason") as UnschedulableReason) : undefined,
      sort: SORT_KEYS.includes(sp.get("sort") ?? "") ? (sp.get("sort") as JobSort) : undefined,
    };
  }, [sp]);
  const onFilters = useCallback((f: JobFilters) => {
    const q = new URLSearchParams();
    if (f.status) q.set("status", f.status);
    if (f.handler) q.set("handler_name", f.handler);
    if (f.priorityMin !== undefined) q.set("priority_min", String(f.priorityMin));
    if (f.reason) q.set("reason", f.reason);
    if (f.sort) q.set("sort", f.sort);
    const qs = q.toString();
    router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
  }, [router, pathname]);
  return <JobsListView filters={filters} onFilters={onFilters} jobLink={jobLink} />;
}

export function JobDetailConsole({ id }: { id: string }) { return <JobDetailView id={id} workerLink={(wid) => <Link className="id mono" href={`/app/workers/${wid}`}>{wid.slice(0, 8)}</Link>} jobLink={(j) => <Link className="id mono" href={`/app/jobs/${j.id}`}>{j.id.slice(0, 8)}</Link>} />; }
export function WorkersConsole() { return <WorkersListView workerLink={workerLink} />; }
export function WorkerDetailConsole({ id }: { id: string }) { return <WorkerDetailView id={id} jobLink={(jid) => <Link className="id mono" href={`/app/jobs/${jid}`}>{jid.slice(0, 8)}</Link>} />; }
export function SettingsConsole() { return <ConfigView />; }
export function NewJobConsole() { return <JobNewView />; }
export function TokensConsole() { return <TokensView />; }
export function ObservabilityConsole() { return <ObservabilityView />; }
export function ActivityConsole() { return <ActivityView />; }

function zones(): string[] {
  try {
    return (Intl as unknown as { supportedValuesOf: (k: string) => string[] }).supportedValuesOf("timeZone");
  } catch {
    return ["UTC"];
  }
}

interface SessionBody { user: { login: string }; role: string; expires_at: string; idle_expires_at: string }

/** Who is signed in and when the session ends, from `GET /bff/session`. Sign-out lives in the sidebar. */
function SessionInfo() {
  const [s, setS] = useState<SessionBody | "none" | null>(null);
  useEffect(() => {
    let alive = true;
    fetch("/bff/session", { credentials: "same-origin" }).then(async (r) => { if (alive) setS(r.ok ? ((await r.json()) as SessionBody) : "none"); }).catch(() => { if (alive) setS("none"); });
    return () => { alive = false; };
  }, []);
  if (s === null) return <p className="meta" role="status">Loading session…</p>;
  if (s === "none") return <p className="meta">The session could not be read.</p>;
  return <DefinitionList items={[{ term: "Signed in as", value: `@${s.user.login}` }, { term: "Role", value: s.role }, { term: "Session ends", value: <Timestamp value={s.expires_at} /> }, { term: "Idle timeout at", value: <Timestamp value={s.idle_expires_at} /> }]} />;
}

/** Local-only preferences (this browser): time zone and reduced motion. Nothing here is sent to a server. */
export function PreferencesConsole() {
  const prefs = usePrefs();
  const toast = useToast();
  const [saved, setSaved] = useState("");
  const list = useMemo(() => ["UTC", ...zones().filter((z) => z !== "UTC")], []);
  const effective = prefs.timezone === "browser" ? browserZone() : prefs.timezone;
  return (
    <div style={{ display: "grid", gap: "var(--space-4)", maxWidth: "40rem" }}>
      <Card>
        <div className="card-h"><h2>Time</h2></div>
        <div className="card-pad">
          <Select label="Time zone" help={`Times display in ${effective}. UTC is always in each time's tooltip.`} value={prefs.timezone} onChange={(e) => { setPrefs({ timezone: e.target.value }); setSaved("Time zone saved for this browser."); toast.show("Time zone saved"); }}>
            <option value="browser">Follow this browser ({browserZone()})</option>
            {list.map((z) => <option key={z} value={z}>{z}</option>)}
          </Select>
        </div>
      </Card>
      <Card>
        <div className="card-h"><h2>Motion</h2></div>
        <div className="card-pad">
          <Select label="Reduce motion" help="Follows your system setting. Choosing Always can only reduce motion, never force it on." value={prefs.reduceMotion} onChange={(e) => { setPrefs({ reduceMotion: e.target.value as "system" | "always" }); setSaved("Motion preference saved for this browser."); toast.show("Motion preference saved"); }}>
            <option value="system">Follow my system setting</option>
            <option value="always">Always reduce motion</option>
          </Select>
        </div>
      </Card>
      <p className="help" role="status">{saved}</p>
      <Card>
        <div className="card-h"><h2>Session</h2></div>
        <div className="card-pad"><SessionInfo /></div>
      </Card>
    </div>
  );
}

/** S-403: names the required role; viewers only. */
export function Forbidden({ role }: { role: string }) {
  return (
    <div className="empty" data-testid="forbidden-state">
      <h2>You do not have access to this page</h2>
      <p>This page needs the <strong>{role}</strong> role. Your role is read-only.</p>
      <Link className="btn btn-primary" href="/app">Back to Overview</Link>
    </div>
  );
}
