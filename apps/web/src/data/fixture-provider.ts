import type { Attempt, DataProvider, Job, JobEvent, JobQuery, JobSummary, Page, Result, Worker, WorkerDetail } from "./provider";
import { FIXTURE_ATTEMPTS, FIXTURE_AS_OF, FIXTURE_CONFIG, FIXTURE_EVENTS, FIXTURE_JOBS, FIXTURE_OVERVIEW, FIXTURE_WORKERS, fixtureWorkerDetail } from "./fixtures/data";

export type FixtureView = "operator" | "viewer";

const ok = <T,>(data: T): Promise<Result<T>> => Promise.resolve({ ok: true, data });
const notFound = (code: "job_not_found" | "worker_not_found"): Promise<Result<never>> => Promise.resolve({ ok: false, error: { code, status: 404 } });

/**
 * In-memory provider for /demo. It has no network capability, no timers and no clock: every call resolves from
 * bundled, synthetic, read-only fixtures. The "viewer" view applies the same masking the API applies to viewers
 * (payload, result, error message, log tail, hostname are removed from the object, never merely hidden by CSS).
 */
export class FixtureProvider implements DataProvider {
  readonly kind = "fixture" as const;
  constructor(private view: FixtureView = "operator") {}

  private maskJob(j: Job): Job {
    if (this.view === "operator") return j;
    return { ...j, payload: null, result: null, error_message: null, masked_fields: ["payload", "result", "error_message"] };
  }
  private maskAttempt(a: Attempt): Attempt {
    if (this.view === "operator") return a;
    return { ...a, error_message: null, log_tail: null, masked_fields: ["error_message", "log_tail"] };
  }
  private maskWorker<W extends Worker>(w: W): W {
    return this.view === "operator" ? w : { ...w, hostname: null, masked_fields: ["hostname"] };
  }

  getOverview() { return ok(FIXTURE_OVERVIEW); }
  getConfig() { return ok(FIXTURE_CONFIG); }

  listJobs(q: JobQuery): Promise<Result<Page<JobSummary>>> {
    let rows = FIXTURE_JOBS.filter((j) => (!q.status?.length || q.status.includes(j.status)) && (!q.handler_name || j.handler_name === q.handler_name) && (q.priority_min === undefined || j.priority >= q.priority_min));
    rows = [...rows].sort((a, b) => b.created_at.localeCompare(a.created_at) || b.id.localeCompare(a.id));
    const limit = q.limit ?? 50;
    const start = q.cursor ? Number.parseInt(q.cursor.replace("demo-cursor-", ""), 10) || 0 : 0;
    const slice = rows.slice(start, start + limit);
    const next = start + limit < rows.length ? `demo-cursor-${start + limit}` : null;
    // Summaries carry no payload fields.
    const items = slice.map((j) => { const { payload: _p, result: _r, error_message: _e, ...s } = j; return s as JobSummary; });
    return ok({ items, next_cursor: next, as_of: FIXTURE_AS_OF });
  }

  getJob(id: string): Promise<Result<Job>> {
    const j = FIXTURE_JOBS.find((x) => x.id === id);
    return j ? ok(this.maskJob(j)) : notFound("job_not_found");
  }
  listAttempts(id: string): Promise<Result<{ items: Attempt[] }>> {
    return FIXTURE_JOBS.some((j) => j.id === id) ? ok({ items: (FIXTURE_ATTEMPTS[id] ?? []).map((a) => this.maskAttempt(a)) }) : notFound("job_not_found");
  }
  listJobEvents(id: string): Promise<Result<{ items: JobEvent[] }>> {
    return FIXTURE_JOBS.some((j) => j.id === id) ? ok({ items: FIXTURE_EVENTS[id] ?? [] }) : notFound("job_not_found");
  }
  listWorkers(_q?: { cursor?: string; limit?: number }): Promise<Result<Page<Worker>>> {
    return ok({ items: FIXTURE_WORKERS.map((w) => this.maskWorker(w)), next_cursor: null, as_of: FIXTURE_AS_OF });
  }
  getWorker(id: string): Promise<Result<WorkerDetail>> {
    const w = FIXTURE_WORKERS.find((x) => x.id === id);
    return w ? ok(this.maskWorker(fixtureWorkerDetail(w))) : notFound("worker_not_found");
  }
}
