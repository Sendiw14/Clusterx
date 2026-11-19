import type { ApiFailure } from "@/data/provider";

export interface FieldIssue { field: string; rule?: string; message: string }

/** Validation fields from a 422 `details.fields` (GAP-ERR-01: key on `field`, show `message`). */
export function fieldIssues(e: ApiFailure): FieldIssue[] {
  const f = e.details?.fields;
  if (!Array.isArray(f)) return [];
  return f.filter((x): x is FieldIssue => typeof x === "object" && x !== null && typeof (x as FieldIssue).field === "string" && typeof (x as FieldIssue).message === "string");
}

const NAMES: Record<string, string> = {
  job_terminal: "The job already finished, so it cannot change.",
  retry_not_allowed: "Only failed jobs can be retried.",
  retry_already_exists: "This job was already retried.",
  worker_deregistered: "The worker is deregistered.",
  idempotency_conflict: "That idempotency key was used with a different request.",
  payload_too_large: "The payload is larger than the limit.",
  validation_failed: "The request did not pass validation.",
  rate_limited: "Too many requests.",
  forbidden: "Your role cannot do this.",
  csrf_failed: "The request was rejected. Reload the page and try again. Nothing was changed.",
  session_expired: "Your session ended.",
  upstream_unavailable: "The control plane could not be reached. Nothing was changed.",
  upstream_auth: "The console could not authenticate to its API. Nothing was changed.",
  unavailable: "The service is not ready. Try again shortly. Nothing was changed.",
  timeout: "No answer arrived in time. The change may or may not have been applied: check the current state before trying again.",
  network: "The request did not reach the server. Nothing was changed.",
  expired: "That item expired by retention and is no longer stored.",
  job_not_found: "No such job.",
  worker_not_found: "No such worker.",
};

/** One plain-language sentence for a failed action. Never the raw server message for BFF-owned or unknown codes. */
export function describeFailure(e: ApiFailure): string {
  let t = NAMES[e.code] ?? "The action failed.";
  if (e.code === "rate_limited" && e.retryAfterSeconds) t += ` Try again in ${e.retryAfterSeconds} s.`;
  const st = e.details?.status;
  if (typeof st === "string" && (e.code === "job_terminal" || e.code === "retry_not_allowed")) t += ` Current status: ${st.replace("_", " ")}.`;
  return t;
}

/** Client-side idempotency key (ADR-0004): one per user intent, reused on retry of the same intent. */
export const newKey = (): string => `ui-${crypto.randomUUID()}`;
