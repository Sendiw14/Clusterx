import type { Attempt, Job, JobEvent, JobStatus, MetricsOverview, Worker, WorkerDetail, Config } from "../provider";

/**
 * ILLUSTRATIVE FIXTURES. Seeded, synthetic, read-only. Used only by /demo through FixtureProvider.
 * Every ID carries the visibly synthetic `demo-` prefix so none can be mistaken for a real record (E2E-DEMO-26).
 * Times are fixed: there is no clock and no simulated live arrival.
 */
export const FIXTURE_AS_OF = "2026-03-02T10:42:07Z";
export const FIXTURE_LABEL = "Fixture time: 2 Mar 2026, 10:42:07 UTC";

const t = (m: number) => new Date(Date.parse(FIXTURE_AS_OF) - m * 60_000).toISOString();

interface Seed {
  n: number;
  handler: string;
  v: number;
  status: JobStatus;
  priority: number;
  attempts: number;
  max: number;
  createdMin: number;
  unschedulable?: "no_capable_worker" | "capable_workers_offline" | "capable_workers_draining";
  failure?: Job["failure_class"];
  errorCode?: string;
}
const SEEDS: Seed[] = [
  { n: 1, handler: "thumbnail", v: 2, status: "running", priority: 5, attempts: 1, max: 3, createdMin: 2 },
  { n: 2, handler: "ocr", v: 1, status: "succeeded", priority: 3, attempts: 2, max: 3, createdMin: 9 },
  { n: 3, handler: "export", v: 3, status: "failed", priority: 5, attempts: 3, max: 3, createdMin: 21, failure: "retryable_exhausted", errorCode: "upstream_timeout" },
  { n: 4, handler: "digest", v: 1, status: "retry_wait", priority: 2, attempts: 1, max: 3, createdMin: 14 },
  { n: 5, handler: "transcode", v: 1, status: "queued", priority: 9, attempts: 0, max: 3, createdMin: 6, unschedulable: "no_capable_worker" },
  { n: 6, handler: "thumbnail", v: 2, status: "queued", priority: 0, attempts: 0, max: 3, createdMin: 4 },
  { n: 7, handler: "export", v: 3, status: "scheduled", priority: 1, attempts: 0, max: 3, createdMin: 30 },
  { n: 8, handler: "ocr", v: 1, status: "canceled", priority: 4, attempts: 1, max: 3, createdMin: 44, failure: "canceled_by_operator" },
  { n: 9, handler: "digest", v: 1, status: "succeeded", priority: 0, attempts: 1, max: 3, createdMin: 55 },
  { n: 10, handler: "thumbnail", v: 2, status: "failed", priority: 6, attempts: 1, max: 1, createdMin: 70, failure: "permanent", errorCode: "bad_input" },
  { n: 11, handler: "ocr", v: 1, status: "succeeded", priority: 7, attempts: 1, max: 3, createdMin: 90 },
  { n: 12, handler: "export", v: 3, status: "succeeded", priority: 5, attempts: 1, max: 3, createdMin: 120 },
  /* 13 is the manual retry of failed job 3: a NEW job with a fresh budget and a link back to its parent. */
  { n: 13, handler: "export", v: 3, status: "succeeded", priority: 5, attempts: 1, max: 3, createdMin: 12 },
];
/** Lineage: job number to the job number it was retried from. */
const RETRIED_FROM: Record<number, number> = { 13: 3 };

export const jobId = (n: number) => `demo-job-${String(n).padStart(4, "0")}`;
const workerId = (n: number) => `demo-worker-${n}`;

export const FIXTURE_JOBS: Job[] = SEEDS.map((s) => ({
  id: jobId(s.n),
  namespace: "default",
  handler_name: s.handler,
  handler_version: s.v,
  status: s.status,
  priority: s.priority,
  attempts_used: s.attempts,
  max_attempts: s.max,
  available_at: t(s.createdMin),
  terminal_at: ["succeeded", "failed", "canceled"].includes(s.status) ? t(Math.max(1, s.createdMin - 3)) : null,
  error_code: s.errorCode ?? null,
  unschedulable_reason: s.unschedulable ?? null,
  unschedulable_since: s.unschedulable ? t(s.createdMin - 1) : null,
  failure_class: s.failure ?? null,
  created_at: t(s.createdMin),
  updated_at: t(Math.max(0, s.createdMin - 2)),
  version: 3 + s.attempts * 3,
  payload: { source: `demo://assets/${s.n}`, width: 320, height: 200 },
  result: s.status === "succeeded" ? { output: `demo://out/${s.n}`, bytes: 18432 } : null,
  error_message: s.errorCode ? `Handler reported ${s.errorCode} (illustrative message)` : null,
  timeout_seconds: 600,
  lease_generation: s.attempts,
  created_by: "user:demo-operator",
  as_of: FIXTURE_AS_OF,
  masked_fields: [],
  ...(RETRIED_FROM[s.n] ? { retried_from_job_id: jobId(RETRIED_FROM[s.n]!) } : {}),
}));

const attempt = (job: number, n: number, state: Attempt["state"], extra: Partial<Attempt> = {}): Attempt => ({
  id: `demo-attempt-${job}-${n}`,
  job_id: jobId(job),
  attempt_number: n,
  lease_generation: n,
  worker_id: workerId(n === 1 ? 1 : 2),
  worker_incarnation_id: `demo-inc-${n === 1 ? "3e8" : "91c"}`,
  state,
  error_class: null,
  error_code: null,
  error_message: null,
  log_tail: null,
  log_truncated: false,
  log_expired: false,
  lease_expires_at: null,
  started_at: t(20 - n * 6),
  finished_at: null,
  masked_fields: [],
  ...extra,
});

/** Attempts for the demonstration jobs: a success after a lease expiry (job 2), an exhausted failure (job 3), a running job (1). */
export const FIXTURE_ATTEMPTS: Record<string, Attempt[]> = {
  [jobId(1)]: [attempt(1, 1, "active", { lease_expires_at: t(-1), log_tail: "claimed attempt 1\nresizing demo://assets/1\n" })],
  [jobId(2)]: [
    attempt(2, 1, "lease_expired", { finished_at: t(14), error_code: "lease_expired", log_tail: "claimed attempt 1\nworker went silent\n" }),
    attempt(2, 2, "succeeded", { finished_at: t(9), log_tail: "claimed attempt 2\nrecognised 412 words\nreported success\n" }),
  ],
  [jobId(3)]: [1, 2, 3].map((n) => attempt(3, n, "failed", { error_class: "retryable", error_code: "upstream_timeout", error_message: "upstream timed out after 30s (illustrative)", finished_at: t(20 - n * 5), log_tail: `attempt ${n}\nPOST demo://upstream -> timeout\n`, log_truncated: n === 3 })),
};

const ev = (job: number, version: number, type: JobEvent["event_type"], to: JobStatus, minutes: number, actor = "system", data?: JobEvent["data"]): JobEvent => ({
  job_id: jobId(job), version, event_type: type, from_status: null, to_status: to, actor, occurred_at: t(minutes), data: data ?? null,
});
export const FIXTURE_EVENTS: Record<string, JobEvent[]> = {
  [jobId(2)]: [
    ev(2, 1, "admitted", "queued", 20, "user:demo-operator"),
    ev(2, 2, "claimed", "running", 19.5, "worker:demo-worker-1", { attempt_number: 1 }),
    ev(2, 3, "lease_expired", "retry_wait", 14, "system", { attempt_number: 1, failure_class: "lease_expired", backoff_ms: 2500 }),
    ev(2, 4, "promoted", "queued", 11.5),
    ev(2, 5, "claimed", "running", 11, "worker:demo-worker-2", { attempt_number: 2 }),
    ev(2, 6, "succeeded", "succeeded", 9, "worker:demo-worker-2", { attempt_number: 2 }),
  ],
  [jobId(3)]: [
    ev(3, 1, "admitted", "queued", 30, "user:demo-operator"),
    ev(3, 2, "claimed", "running", 29),
    ev(3, 3, "failed_retryable", "retry_wait", 26, "worker:demo-worker-1", { attempt_number: 1, error_code: "upstream_timeout", backoff_ms: 5000 }),
    ev(3, 4, "attempts_exhausted", "failed", 21, "system", { failure_class: "retryable_exhausted", attempt_number: 3 }),
  ],
};

FIXTURE_ATTEMPTS[jobId(13)] = [attempt(13, 1, "succeeded", { worker_id: workerId(2), worker_incarnation_id: "demo-inc-91c", finished_at: t(9), log_tail: "claimed attempt 1\nexport finished\nreported success\n" })];
FIXTURE_EVENTS[jobId(13)] = [
  ev(13, 1, "retry_created", "queued", 12, "user:demo-operator", { retried_from_job_id: jobId(3) }),
  ev(13, 2, "claimed", "running", 11, "worker:demo-worker-2", { attempt_number: 1 }),
  ev(13, 3, "succeeded", "succeeded", 9, "worker:demo-worker-2", { attempt_number: 1 }),
];

export const FIXTURE_WORKERS: Worker[] = [
  { id: workerId(1), name: "demo-worker-a", status: "active", live: true, version: 4, incarnation_id: "demo-inc-3e8", capacity: 4, active_attempts: 2, protocol_version: 1, sdk_version: "demo", hostname: "demo-host-1.invalid", capabilities: [{ name: "thumbnail", version: 2 }, { name: "ocr", version: 1 }], registered_at: t(600), last_seen_at: t(0.1), masked_fields: [] },
  { id: workerId(2), name: "demo-worker-b", status: "active", live: true, version: 2, incarnation_id: "demo-inc-91c", capacity: 4, active_attempts: 1, protocol_version: 1, sdk_version: "demo", hostname: "demo-host-2.invalid", capabilities: [{ name: "export", version: 3 }, { name: "digest", version: 1 }], registered_at: t(480), last_seen_at: t(0.2), masked_fields: [] },
  { id: workerId(3), name: "demo-worker-c", status: "active", live: false, version: 6, incarnation_id: "demo-inc-c01", capacity: 2, active_attempts: 0, protocol_version: 1, sdk_version: "demo", hostname: "demo-host-3.invalid", capabilities: [{ name: "ocr", version: 1 }], registered_at: t(900), last_seen_at: t(7), masked_fields: [] },
  { id: workerId(4), name: "demo-worker-d", status: "draining", live: true, version: 8, incarnation_id: "demo-inc-d77", capacity: 2, active_attempts: 1, protocol_version: 1, sdk_version: "demo", hostname: "demo-host-4.invalid", capabilities: [{ name: "digest", version: 1 }], registered_at: t(300), last_seen_at: t(0.3), drain_requested_at: t(5), drain_deadline_at: t(-1), masked_fields: [] },
  { id: workerId(5), name: "demo-worker-e", status: "active", live: true, version: 9, incarnation_id: "demo-inc-e12", capacity: 2, active_attempts: 0, protocol_version: 1, sdk_version: "demo", hostname: "demo-host-5.invalid", capabilities: [{ name: "export", version: 3 }], registered_at: t(200), last_seen_at: t(40), credential_revoked: true, credential_revoked_at: t(40), masked_fields: [] },
];

export const fixtureWorkerDetail = (w: Worker): WorkerDetail => ({
  ...w,
  as_of: FIXTURE_AS_OF,
  active: w.active_attempts > 0 ? [{ job_id: jobId(1), attempt_id: "demo-attempt-1-1", attempt_number: 1, handler_name: "thumbnail", started_at: t(2), lease_expires_at: t(-0.5) }] : [],
});

export const FIXTURE_OVERVIEW: MetricsOverview = {
  as_of: FIXTURE_AS_OF,
  jobs_by_status: { scheduled: 1, queued: 2, running: 1, retry_wait: 1, succeeded: 5, failed: 2, canceled: 1 },
  workers: { live: 3, stale: 1, draining: 1 },
  oldest_queued_age_seconds: 360,
  unschedulable_jobs: 1,
  terminal_counts_window: { days: 30, since: new Date(Date.parse(FIXTURE_AS_OF) - 30 * 86400_000).toISOString() },
};

export const FIXTURE_CONFIG = {
  api_version: "fixture",
  contract_version: "fixture",
  environment: "demo fixtures",
  build: { version: "demo", commit: "none" },
  limits: { payload_max_bytes: 65536, result_max_bytes: 131072, request_body_max_bytes: 81920, log_tail_max_bytes: 16384, max_attempts_min: 1, max_attempts_max: 5, max_attempts_default: 3, timeout_seconds_max: 86400, priority_min: 0, priority_max: 9, page_size_max: 200, page_size_default: 50, jobs_list_window_max_days: 31, run_after_max_days: 30, timeseries_points_max: 2016, idempotency_ttl_hours: 24, timeseries_window_max_days: 7 },
  retention: { attempt_logs_days: 7, terminal_jobs_days: 30, audit_days: 90, tombstone_days: 90 },
  timing: { heartbeat_interval_seconds: 10, liveness_timeout_seconds: 30, lease_ttl_seconds: 30, cancel_grace_seconds: 15, drain_grace_default_seconds: 60 },
} satisfies Config;
