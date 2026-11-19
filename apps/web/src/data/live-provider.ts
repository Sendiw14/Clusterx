import { createClusterxClient, type components } from "@clusterx/sdk-ts";
import type { ApiToken, ApiTokenCreate, ApiTokenCreated, AuditEvent, AuditQuery, ConsoleProvider, CredentialQuery, Handler, JobCreate, Metric, Series, WorkerCredential, WorkerCredentialCreated } from "./console-provider";
import type { ApiFailure, Attempt, Config, Job, JobEvent, JobQuery, JobSummary, MetricsOverview, Page, Result, Worker, WorkerDetail } from "./provider";

type ErrorBody = components["schemas"]["Error"];
const TIMEOUT_MS = 10_000;
const WRITE_TIMEOUT_MS = 20_000;

function failure(status: number, body: unknown, response?: Response): ApiFailure {
  const b = (body ?? {}) as Partial<ErrorBody>;
  const retry = Number(response?.headers.get("Retry-After"));
  return {
    code: typeof b.code === "string" ? b.code : status >= 500 ? "internal" : "bad_request",
    status,
    message: typeof b.message === "string" ? b.message : undefined,
    requestId: (typeof b.request_id === "string" ? b.request_id : undefined) ?? response?.headers.get("X-Request-Id") ?? undefined,
    retryAfterSeconds: Number.isFinite(retry) && retry > 0 ? retry : undefined,
    details: b.details && typeof b.details === "object" ? (b.details as Record<string, unknown>) : undefined,
  };
}

export interface LiveHooks {
  /** CSRF token for mutating calls (from the server-rendered session). */
  csrf?: () => string | undefined;
  /** Fetches a fresh CSRF token after `csrf_failed` (one automatic retry). */
  refreshCsrf?: () => Promise<string | undefined>;
  /** The BFF said the human session ended (`details.reason`: session_expired | user_disabled). */
  onSessionEnded?: (reason: string) => void;
  /** Go answered 403: the role may have changed, so the page should re-read it. */
  onForbidden?: () => void;
  /** Every completed call, for snapshot freshness (stale indicator). `false` = network, timeout or 5xx. */
  onResult?: (ok: boolean) => void;
}

/**
 * Browser-side provider for the console: every call goes to the BFF (`/bff/api/v1/...`) through the generated
 * `@clusterx/sdk-ts` client. The browser never talks to the Go control plane and holds no API tokens (MASTER 4).
 * A client timeout becomes `timeout` (S-ERR-TIMEOUT), distinct from 503.
 */
export class LiveProvider implements ConsoleProvider {
  readonly kind = "live" as const;
  private client: ReturnType<typeof createClusterxClient>;
  private csrfOverride?: string;

  /** `baseUrl` is the BFF origin path; tests pass an absolute URL because Node cannot resolve relative Requests. */
  constructor(baseUrl = "/bff", private hooks: LiveHooks = {}) {
    this.client = createClusterxClient(baseUrl, {
      fetch: async (input) => {
        const req = input as Request;
        const write = req.method !== "GET" && req.method !== "HEAD";
        const send = async (token?: string) => {
          const r = req.clone();
          if (write && token) r.headers.set("X-CSRF-Token", token);
          const ctl = new AbortController();
          const t = setTimeout(() => ctl.abort(), write ? WRITE_TIMEOUT_MS : TIMEOUT_MS);
          try {
            return await fetch(new Request(r, { signal: ctl.signal, credentials: "same-origin" }));
          } finally {
            clearTimeout(t);
          }
        };
        let res = await send(this.csrfOverride ?? this.hooks.csrf?.());
        if (write && res.status === 403 && (await res.clone().json().catch(() => null))?.code === "csrf_failed" && this.hooks.refreshCsrf) {
          this.csrfOverride = await this.hooks.refreshCsrf();
          res = await send(this.csrfOverride);
        }
        return res;
      },
    });
  }

  private async run<T>(call: () => Promise<{ data?: T; error?: unknown; response: Response }>): Promise<Result<T>> {
    try {
      const { data, error, response } = await call();
      this.hooks.onResult?.(response.status < 500);
      if (response.ok && (data !== undefined || response.status === 204)) {
        return { ok: true, data: data as T, requestId: response.headers.get("X-Request-Id") ?? undefined, status: response.status, replayed: response.headers.get("Idempotent-Replayed") === "true" };
      }
      const f = failure(response.status, error, response);
      if (response.status === 401 && f.code === "session_expired") this.hooks.onSessionEnded?.(String(f.details?.reason ?? "session_expired"));
      if (response.status === 403 && f.code === "forbidden") this.hooks.onForbidden?.();
      return { ok: false, error: f };
    } catch (e) {
      this.hooks.onResult?.(false);
      const aborted = (e as { name?: string } | null)?.name === "AbortError";
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
    return this.run(() => this.client.GET("/api/v1/jobs/{id}/events", { params: { path: { id }, query: { limit: 200 } } }) as never);
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

  getTimeseries(q: { metric: Metric; from: string; to: string; step: "1m" | "5m" | "1h" }): Promise<Result<Series>> {
    return this.run(() => this.client.GET("/api/v1/metrics/timeseries", { params: { query: q } }) as never);
  }
  listAudit(q: AuditQuery): Promise<Result<Page<AuditEvent>>> {
    return this.run(() => this.client.GET("/api/v1/audit", { params: { query: q } }) as never);
  }
  listHandlers(): Promise<Result<{ items: Handler[]; as_of: string }>> {
    return this.run(() => this.client.GET("/api/v1/handlers") as never);
  }
  createJob(body: JobCreate, key: string): Promise<Result<Job>> {
    return this.run(() => this.client.POST("/api/v1/jobs", { body, params: { header: { "Idempotency-Key": key } } }) as never);
  }
  cancelJob(id: string): Promise<Result<Job>> {
    return this.run(() => this.client.POST("/api/v1/jobs/{id}/cancel", { params: { path: { id } } }) as never);
  }
  retryJob(id: string, key: string): Promise<Result<Job>> {
    return this.run(() => this.client.POST("/api/v1/jobs/{id}/retry", { params: { path: { id }, header: { "Idempotency-Key": key } } }) as never);
  }
  drainWorker(id: string, grace: number): Promise<Result<Worker>> {
    return this.run(() => this.client.POST("/api/v1/workers/{id}/drain", { params: { path: { id } }, body: { grace_seconds: grace } }) as never);
  }
  listTokens(q: CredentialQuery): Promise<Result<Page<ApiToken>>> {
    return this.run(() => this.client.GET("/api/v1/tokens", { params: { query: q } }) as never);
  }
  createToken(body: ApiTokenCreate, key: string): Promise<Result<ApiTokenCreated | ApiToken>> {
    return this.run(() => this.client.POST("/api/v1/tokens", { body, params: { header: { "Idempotency-Key": key } } }) as never);
  }
  revokeToken(id: string): Promise<Result<ApiToken>> {
    return this.run(() => this.client.POST("/api/v1/tokens/{id}/revoke", { params: { path: { id } } }) as never);
  }
  listWorkerCredentials(q: CredentialQuery): Promise<Result<Page<WorkerCredential>>> {
    return this.run(() => this.client.GET("/api/v1/worker-credentials", { params: { query: q } }) as never);
  }
  createWorkerCredential(body: { name: string; expires_in_days: number | null }, key: string): Promise<Result<WorkerCredentialCreated | WorkerCredential>> {
    return this.run(() => this.client.POST("/api/v1/worker-credentials", { body, params: { header: { "Idempotency-Key": key } } }) as never);
  }
  revokeWorkerCredential(id: string): Promise<Result<WorkerCredential>> {
    return this.run(() => this.client.POST("/api/v1/worker-credentials/{id}/revoke", { params: { path: { id } } }) as never);
  }
}
