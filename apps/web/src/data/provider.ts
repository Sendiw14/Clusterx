import type { components } from "@clusterx/sdk-ts";

type S = components["schemas"];
export type JobStatus = S["JobStatus"];
export type JobSummary = S["JobSummary"];
export type Job = S["Job"];
export type Attempt = S["Attempt"];
export type JobEvent = S["JobEvent"];
export type Worker = S["Worker"];
export type WorkerDetail = S["WorkerDetail"];
export type MetricsOverview = S["MetricsOverview"];
export type Config = S["Config"];

/** Normalised failure: `code` is the API (or BFF) error code; `request_id` is shown and copyable (S-ERR-*). */
export interface ApiFailure {
  code: string;
  status: number;
  message?: string;
  requestId?: string;
  retryAfterSeconds?: number;
}
export type Result<T> = { ok: true; data: T; requestId?: string } | { ok: false; error: ApiFailure };

export interface Page<T> {
  items: T[];
  next_cursor: string | null;
  as_of: string;
}

export interface JobQuery {
  status?: JobStatus[];
  handler_name?: string;
  priority_min?: number;
  cursor?: string;
  limit?: number;
}

/**
 * The single seam every page and shared view reads through. `LiveProvider` (console) goes through the BFF using the
 * generated SDK; `FixtureProvider` (demo) is in-memory and has no network capability. Views never import either.
 */
export interface DataProvider {
  readonly kind: "live" | "fixture";
  getOverview(): Promise<Result<MetricsOverview>>;
  listJobs(q: JobQuery): Promise<Result<Page<JobSummary>>>;
  getJob(id: string): Promise<Result<Job>>;
  listAttempts(jobId: string): Promise<Result<{ items: Attempt[] }>>;
  listJobEvents(jobId: string): Promise<Result<{ items: JobEvent[] }>>;
  listWorkers(q: { cursor?: string; limit?: number }): Promise<Result<Page<Worker>>>;
  getWorker(id: string): Promise<Result<WorkerDetail>>;
  getConfig(): Promise<Result<Config>>;
}
