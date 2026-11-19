import type { components } from "@clusterx/sdk-ts";
import type { DataProvider, Job, Page, Result, Worker } from "./provider";

type S = components["schemas"];
export type Series = S["TimeSeries"];
export type AuditEvent = S["AuditEvent"];
export type Handler = S["Handler"];
export type ApiToken = S["ApiToken"];
export type ApiTokenCreated = S["ApiTokenCreated"];
export type ApiTokenCreate = S["ApiTokenCreate"];
export type WorkerCredential = S["WorkerCredential"];
export type WorkerCredentialCreated = S["WorkerCredentialCreated"];
export type JobCreate = S["JobCreate"];
export type Metric = "jobs_admitted" | "jobs_succeeded" | "jobs_failed" | "jobs_canceled" | "attempts_started" | "lease_expirations" | "queue_age_p95_seconds" | "run_duration_p95_seconds";
export interface AuditQuery { category?: "privileged" | "security" | "diagnostic"; subject_type?: "job" | "worker" | "worker_credential" | "api_token" | "user" | "session"; subject_id?: string; from?: string; to?: string; cursor?: string; limit?: number }
export interface CredentialQuery { include_revoked?: boolean; cursor?: string; limit?: number }

/**
 * Everything the console reads or changes. Only `LiveProvider` implements it; `/demo` never sees these methods. Mutating
 * calls carry a client-generated Idempotency-Key where the API supports one.
 */
export interface ConsoleProvider extends DataProvider {
  getTimeseries(q: { metric: Metric; from: string; to: string; step: "1m" | "5m" | "1h" }): Promise<Result<Series>>;
  listAudit(q: AuditQuery): Promise<Result<Page<AuditEvent>>>;
  listHandlers(): Promise<Result<{ items: Handler[]; as_of: string }>>;
  createJob(body: JobCreate, idempotencyKey: string): Promise<Result<Job>>;
  cancelJob(id: string): Promise<Result<Job>>;
  retryJob(id: string, idempotencyKey: string): Promise<Result<Job>>;
  drainWorker(id: string, graceSeconds: number): Promise<Result<Worker>>;
  listTokens(q: CredentialQuery): Promise<Result<Page<ApiToken>>>;
  createToken(body: ApiTokenCreate, idempotencyKey: string): Promise<Result<ApiTokenCreated | ApiToken>>;
  revokeToken(id: string): Promise<Result<ApiToken>>;
  listWorkerCredentials(q: CredentialQuery): Promise<Result<Page<WorkerCredential>>>;
  createWorkerCredential(body: { name: string; expires_in_days: number | null }, idempotencyKey: string): Promise<Result<WorkerCredentialCreated | WorkerCredential>>;
  revokeWorkerCredential(id: string): Promise<Result<WorkerCredential>>;
}
