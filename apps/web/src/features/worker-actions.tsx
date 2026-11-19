"use client";
import { useState } from "react";
import { Button, Notice, TextInput, useOnline, useToast } from "@clusterx/ui";
import { useConsoleProvider, useRole } from "@/data/context";
import type { ApiFailure, WorkerDetail } from "@/data/provider";
import { ConfirmDialog } from "./confirm";
import { describeFailure } from "./failure";

/** Drain (operator only): stops new claims; active attempts may finish until the grace period ends, then the lease path recovers them. */
export function WorkerActions({ worker, onChanged }: { worker: WorkerDetail; onChanged: () => void }) {
  const p = useConsoleProvider();
  const role = useRole();
  const toast = useToast();
  const online = useOnline();
  const [open, setOpen] = useState(false);
  const [grace, setGrace] = useState("60");
  const [pending, setPending] = useState(false);
  const [err, setErr] = useState<ApiFailure | null>(null);
  const g = Number(grace);
  const valid = grace.trim() !== "" && Number.isInteger(g) && g >= 0 && g <= 3600;

  if (worker.status === "deregistered") return <p className="meta">This worker is deregistered and cannot be drained.</p>;
  if (role !== "operator") return <p className="meta">Draining needs the operator role. Your access is read-only.</p>;
  if (worker.status === "draining") {
    return <Notice variant="info" lead="Draining." role="none">No new claims. Active attempts may finish until {worker.drain_deadline_at ? new Date(worker.drain_deadline_at).toISOString() : "the grace period ends"} (UTC); anything still running is then recovered through lease expiry and retried elsewhere.</Notice>;
  }

  async function drain() {
    if (!valid) return;
    setPending(true);
    const r = await p.drainWorker(worker.id, g);
    setPending(false);
    if (r.ok) {
      setOpen(false);
      toast.show("Worker is draining");
      onChanged();
    } else {
      setErr(r.error);
      if (r.error.status === 409) onChanged();
    }
  }

  return (
    <div>
      <div className="toolbar">
        <Button variant="danger" unavailable={!online} onClick={() => { setErr(null); setOpen(true); }}>Drain worker</Button>
        {!online ? <span className="meta">You are offline. Changes are disabled until you reconnect.</span> : null}
      </div>
      <ConfirmDialog open={open} title="Drain this worker?" verb="Drain worker" danger pending={pending} onConfirm={() => void drain()} onCancel={() => { if (!pending) setOpen(false); }} error={err ? describeFailure(err) : null}>
        <p><strong>{worker.name}</strong> has {worker.active_attempts} active attempt{worker.active_attempts === 1 ? "" : "s"}. It stops taking new work immediately.</p>
        <TextInput label="Grace period (seconds)" help="Active attempts may finish within this time. After it, the lease path recovers them (0 to 3600)." inputMode="numeric" value={grace} onChange={(e) => setGrace(e.target.value)} error={valid ? undefined : "Enter a whole number from 0 to 3600."} />
      </ConfirmDialog>
    </div>
  );
}
