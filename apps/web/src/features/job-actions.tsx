"use client";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button, Notice, useOnline, useToast } from "@clusterx/ui";
import { useConsoleProvider, useRole } from "@/data/context";
import type { ApiFailure, Job } from "@/data/provider";
import { ConfirmDialog } from "./confirm";
import { describeFailure, newKey } from "./failure";

const CANCELABLE = ["scheduled", "queued", "running", "retry_wait"];

/**
 * Cancel and manual retry (operator only; Go enforces, the UI only hides). Both confirm first. 409s carry the current
 * `{status, version}`, so the page reloads to reconcile instead of leaving a stale button (ADR-0002, ADR-0004).
 */
export function JobActions({ job, onChanged }: { job: Job; onChanged: () => void }) {
  const p = useConsoleProvider();
  const role = useRole();
  const router = useRouter();
  const toast = useToast();
  const online = useOnline();
  const [dlg, setDlg] = useState<"cancel" | "retry" | null>(null);
  const [pending, setPending] = useState(false);
  const [err, setErr] = useState<ApiFailure | null>(null);
  const [key, setKey] = useState(newKey);
  const [note, setNote] = useState<string | null>(null);

  const canCancel = CANCELABLE.includes(job.status);
  const canRetry = job.status === "failed" && !job.retried_as_job_id;
  if (role !== "operator") {
    return canCancel || job.status === "failed" ? <p className="meta">Cancel and retry need the operator role. Your access is read-only.</p> : null;
  }
  if (!canCancel && !canRetry && job.status !== "failed") return null;

  const open = (d: "cancel" | "retry") => { setErr(null); setKey(newKey()); setDlg(d); };
  const close = () => { if (!pending) setDlg(null); };

  async function cancel() {
    setPending(true);
    const r = await p.cancelJob(job.id);
    setPending(false);
    if (r.ok) {
      setDlg(null);
      setNote(r.status === 202 ? "Cancel requested. The job is still running; it finishes as canceled when the worker acknowledges or its lease expires. A late success can no longer commit." : "Job canceled.");
      toast.show(r.status === 202 ? "Cancel requested" : "Job canceled");
      onChanged();
    } else {
      setErr(r.error);
      if (r.error.status === 409) onChanged();
    }
  }
  async function retry() {
    setPending(true);
    const r = await p.retryJob(job.id, key);
    setPending(false);
    if (r.ok) {
      setDlg(null);
      toast.show(r.replayed ? "This retry already exists. Opening it." : "Retry created");
      router.push(`/app/jobs/${r.data.id}`);
    } else {
      setErr(r.error);
      if (r.error.status === 409) onChanged();
    }
  }
  const existing = typeof err?.details?.retried_as_job_id === "string" ? err.details.retried_as_job_id : null;

  return (
    <div style={{ marginBottom: "var(--space-3)" }}>
      <div className="toolbar">
        {canCancel ? <Button variant="danger" unavailable={!online} onClick={() => open("cancel")}>Cancel job</Button> : null}
        {job.status === "failed" ? <Button variant="primary" unavailable={!online || !canRetry} onClick={() => open("retry")}>Retry job</Button> : null}
        {!online ? <span className="meta">You are offline. Changes are disabled until you reconnect.</span> : null}
        {job.status === "failed" && job.retried_as_job_id ? <span className="meta">Already retried as <Link href={`/app/jobs/${job.retried_as_job_id}`}>{job.retried_as_job_id.slice(0, 8)}</Link>.</span> : null}
      </div>
      {note ? <div style={{ marginTop: "var(--space-3)" }}><Notice variant="info" lead="Done.">{note}</Notice></div> : null}
      <ConfirmDialog open={dlg === "cancel"} title="Cancel this job?" verb="Cancel job" danger pending={pending} onConfirm={() => void cancel()} onCancel={close} error={err ? describeFailure(err) : null}>
        <p>Job <span className="mono">{job.id.slice(0, 8)}</span> ({job.handler_name}@{job.handler_version}) is <strong>{job.status.replace("_", " ")}</strong>.</p>
        <p className="muted">{job.status === "running" ? "It is running now. Canceling asks the worker to stop; the job ends as canceled once the worker acknowledges or the lease expires. Work already done is not undone." : "It has not started. It will end as canceled and never run."}</p>
      </ConfirmDialog>
      <ConfirmDialog open={dlg === "retry"} title="Retry this job?" verb="Create retry" pending={pending} onConfirm={() => void retry()} onCancel={close} error={err ? describeFailure(err) : null}>
        <p>This creates a <strong>new job</strong> linked to <span className="mono">{job.id.slice(0, 8)}</span> with the same payload. The failed job stays as it is.</p>
        {existing ? <p>It was already retried: <Link href={`/app/jobs/${existing}`}>open the retry</Link>.</p> : null}
      </ConfirmDialog>
    </div>
  );
}
