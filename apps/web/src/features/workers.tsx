"use client";
import type { ReactNode } from "react";
import Link from "next/link";
import { Card, DataTable, DefinitionList, MaskedField, StatusBadge, Timestamp, type Column, type StatusKey } from "@clusterx/ui";
import { useDataProvider } from "@/data/context";
import type { Worker, WorkerDetail } from "@/data/provider";
import { Load } from "./common";
import { useResource } from "./use-resource";

export function workerState(w: Worker): StatusKey {
  if (w.credential_revoked) return "revoked";
  if (w.status === "deregistered") return "deregistered";
  if (w.status === "draining") return "draining";
  return w.live ? "online" : "stale";
}

export function WorkersListView({ workerLink }: { workerLink: (w: Worker) => ReactNode }) {
  const provider = useDataProvider();
  const res = useResource(() => provider.listWorkers({ limit: 100 }), `workers-${provider.kind}`);
  const columns: Column<Worker>[] = [
    { key: "name", header: "Worker", primary: true, render: (w) => <>{workerLink(w)} <StatusBadge state={workerState(w)} /></> },
    { key: "caps", header: "Handlers", render: (w) => <span className="mono">{w.capabilities.map((c) => `${c.name}@${c.version}`).join(", ") || "none"}</span> },
    { key: "cap", header: "Active / capacity", numeric: true, render: (w) => `${w.active_attempts} / ${w.capacity}` },
    { key: "seen", header: "Last seen", render: (w) => <Timestamp value={w.last_seen_at} /> },
  ];
  return (
    <Card>
      <Load res={res} rows={4} label="Loading workers" errorHeading="Workers could not be loaded">
        {(p) =>
          p.items.length === 0 ? (
            <div className="empty"><h3>No workers registered</h3><p>Workers appear after a trusted worker registers with a credential issued by an operator. See the Worker SDK documentation.</p><Link href="/docs/workers">Worker SDK docs</Link></div>
          ) : (
            <DataTable caption={<>Workers, {p.items.length} shown · as of <Timestamp value={p.as_of} /></>} columns={columns} rows={p.items} rowKey={(w) => w.id} />
          )
        }
      </Load>
    </Card>
  );
}

export function WorkerDetailView({ id }: { id: string }) {
  const provider = useDataProvider();
  const res = useResource(() => provider.getWorker(id), `worker-${id}-${provider.kind}`);
  return (
    <Card>
      <div className="card-h"><h2>Worker</h2></div>
      <div className="card-pad">
        <Load res={res} rows={3} label="Loading worker" errorHeading={res.state === "error" && res.error.code === "worker_not_found" ? "No worker with that ID" : "The worker could not be loaded"}>
          {(w: WorkerDetail) => (
            <>
              <DefinitionList
                items={[
                  { term: "ID", value: <span className="mono">{w.id}</span> },
                  { term: "Name", value: w.name },
                  { term: "State", value: <StatusBadge state={workerState(w)} /> },
                  { term: "Incarnation", value: <span className="mono">{w.incarnation_id ?? "none"}</span> },
                  { term: "Capacity", value: `${w.active_attempts} active of ${w.capacity}` },
                  { term: "Handlers", value: <span className="mono">{w.capabilities.map((c) => `${c.name}@${c.version}`).join(", ") || "none"}</span> },
                  { term: "Hostname", value: w.masked_fields.includes("hostname") ? <MaskedField /> : <span className="mono">{w.hostname ?? "not reported"}</span> },
                  { term: "Last heartbeat", value: <Timestamp value={w.last_seen_at} /> },
                  { term: "Registered", value: <Timestamp value={w.registered_at} /> },
                ]}
              />
              <h3 style={{ margin: "var(--space-5) 0 var(--space-2)", fontSize: "var(--text-ui-lg)" }}>Active attempts</h3>
              {w.active.length ? (
                <ul>{w.active.map((a) => <li key={a.attempt_id}><span className="mono">{a.job_id}</span> attempt {a.attempt_number}, lease expires <Timestamp value={a.lease_expires_at} /></li>)}</ul>
              ) : <p className="muted">No active attempts.</p>}
            </>
          )}
        </Load>
      </div>
    </Card>
  );
}
