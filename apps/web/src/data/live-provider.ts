import { createClusterxClient, type components } from "@clusterx/sdk-ts";
import type { ApiFailure, Attempt, Config, DataProvider, Job, JobEvent, JobQuery, JobSummary, MetricsOverview, Page, Result, Worker, WorkerDetail } from "./provider";

type ErrorBody = components["schemas"]["Error"];
const TIMEOUT_MS = 10_000;

function failure(status: number, body: unknown, response?: Response): ApiFailure {
  const b = (body ?? {}) as Partial<ErrorBody>;
  const retry = Number(response?.headers.get("Retry-After"));
  return {
    code: typeof b.code === "string" ? b.code : status >= 500 ? "internal" : "bad_request",
    status,
    message: typeof b.message === "string" ? b.message : undefined,
    requestId: (typeof b.request_id === "string" ? b.request_id : undefined) ?? response?.headers.get("X-Request-Id") ?? undefined,
    retryAfterSeconds: Number.isFinite(retry) && retry > 0 ? retry : undefined,
  };
}

/**
 * Browser-side provider for the console: every call goes to the BFF (`/bff/api/v1/...`) through the generated
 * `@clusterx/sdk-ts` client. The browser never talks to the Go control plane and holds no API tokens (MASTER 4).
 * A 10 s client timeout becomes `timeout` (S-ERR-TIMEOUT), distinct from 503.
 */
export class LiveProvider implements DataProvider {
  readonly kind = "live" as const;
  private client = createClusterxClient("/bff", {
    fetch: async (req) => {
      const ctl = new AbortController();
      const t = setTimeout(() => ctl.abort(), TIMEOUT_MS);
      try {
        return await fetch(new Request(req, { signal: ctl.signal, credentials: "same-origin" }));
      } finally {
        clearTimeout(t);
      }
    },
  });

  private async run<T>(call: () => Promise<{ data?: T; error?: unknown; response: Response }>): Promise<Result<T>> {
    try {
      const { data, error, response } = await call();
      if (response.ok && data !== undefined) return { ok: true, data, requestId: response.headers.get("X-Request-Id") ?? undefined };
      return { ok: false, error: failure(response.status, error, response) };
    } catch (e) {
      const aborted = e instanceof DOMException && e.name === "AbortError";
      return { ok: false, error: { code: aborted ? "timeout" : "network", status: 0 } };
    }
  }

  getOverview(): Promise<Result<MetricsOverview>> {
    return this.run(() => this.client.GET("/api/v1/metrics/overview") as never);
  }
  listJobs(q: JobQuery): Promise<Result<Page<JobSummary>>> {
    return this.run(() => this.client.GET("/api/v1/jobs", { params: { query: q } }) as never);
  }
  getJob(id: string): Promise<Result<Job>> {
    return this.run(() => this.client.GET("/api/v1/jobs/{id}", { params: { path: { id } } }) as never);
  }
  listAttempts(id: string): Promise<Result<{ items: Attempt[] }>> {
    return this.run(() => this.client.GET("/api/v1/jobs/{id}/attempts", { params: { path: { id } } }) as never);
  }
  listJobEvents(id: string): Promise<Result<{ items: JobEvent[] }>> {
    return this.run(() => this.client.GET("/api/v1/jobs/{id}/events", { params: { path: { id } } }) as never);
  }
  listWorkers(q: { cursor?: string; limit?: number }): Promise<Result<Page<Worker>>> {
    return this.run(() => this.client.GET("/api/v1/workers", { params: { query: q } }) as never);
  }
  getWorker(id: string): Promise<Result<WorkerDetail>> {
    return this.run(() => this.client.GET("/api/v1/workers/{id}", { params: { path: { id } } }) as never);
  }
  getConfig(): Promise<Result<Config>> {
    return this.run(() => this.client.GET("/api/v1/config") as never);
  }
}
