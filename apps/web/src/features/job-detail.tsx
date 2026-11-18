"use client";
import type { ReactNode } from "react";
import { Card, CopyButton, DefinitionList, MaskedField, Notice, ReasonChip, StatusBadge, Timestamp } from "@clusterx/ui";
import { useDataProvider } from "@/data/context";
import type { Attempt, Job, JobEvent } from "@/data/provider";
import { Load, reasonLabel } from "./common";
import { useResource } from "./use-resource";

const json = (v: unknown) => JSON.stringify(v, null, 2);

function MaybeJson({ value, masked, empty }: { value: unknown; masked: boolean; empty: string }) {
  if (masked) return <MaskedField />;
  if (value === null || value === undefined) return <span className="meta">{empty}</span>;
  return <pre className="jsonblock" tabIndex={0} aria-label="JSON value">{json(value)}</pre>;
}

function AttemptCard({ a }: { a: Attempt }) {
  const logMasked = a.masked_fields.includes("log_tail");
  const errMasked = a.masked_fields.includes("error_message");
  return (
    <article className="attempt">
      <header>
        <h3 style={{ fontSize: "var(--text-ui-lg)" }}>Attempt {a.attempt_number}</h3>
        <span className="badge">{a.state.replace("_", " ")}</span>
        {a.error_class ? <span className="badge">{a.error_class}</span> : null}
        <span className="meta">lease generation {a.lease_generation}</span>
      </header>
      <div>
        <DefinitionList
          items={[
            { term: "Worker", value: <span className="mono">{a.worker_id}</span> },
            { term: "Started", value: <Timestamp value={a.started_at} /> },
            { term: "Finished", value: a.finished_at ? <Timestamp value={a.finished_at} /> : <span className="meta">Not finished</span> },
            { term: "Error", value: errMasked ? <MaskedField /> : a.error_message ? a.error_message : <span className="meta">None</span> },
          ]}
        />
        {logMasked ? <p style={{ marginTop: "var(--space-2)" }}><MaskedField what="log tail" /></p> : a.log_expired ? <p className="meta">Log removed by the 7-day log retention.</p> : a.log_tail ? (
          <>
            <pre className="logtail" tabIndex={0} aria-label={`Log tail, attempt ${a.attempt_number}`}>{a.log_tail}</pre>
            {a.log_truncated ? <p className="meta">Log truncated at 16 KiB.</p> : null}
          </>
        ) : <p className="meta">No log output.</p>}
      </div>
    </article>
  );
}

function Timeline({ events }: { events: JobEvent[] }) {
  return (
    <ol className="timeline" aria-label="Job event timeline, oldest first">
      {events.map((e) => (
        <li key={e.version} data-kind={e.to_status === "succeeded" ? "ok" : e.to_status === "failed" ? "bad" : e.event_type === "lease_expired" ? "warn" : undefined}>
          <span className="t"><span className="badge">v{e.version}</span> <Timestamp value={e.occurred_at} /></span>
          <p><strong>{e.event_type.replace(/_/g, " ")}</strong> → {e.to_status.replace("_", " ")} <span className="meta">by {e.actor}</span></p>
        </li>
      ))}
    </ol>
  );
}

/** Job detail: status, fields subject to viewer masking, attempts, event timeline. Cancel and retry are not wired yet, so they are absent. */
export function JobDetailView({ id, workerLink }: { id: string; workerLink?: (id: string) => ReactNode }) {
  void workerLink;
  const provider = useDataProvider();
  const job = useResource(() => provider.getJob(id), `job-${id}-${provider.kind}`);
  const attempts = useResource(() => provider.listAttempts(id), `att-${id}-${provider.kind}`);
  const events = useResource(() => provider.listJobEvents(id), `ev-${id}-${provider.kind}`);

  return (
    <>
      <Card>
        <div className="card-h"><h2>Job</h2>{job.state === "ok" ? <CopyButton text={job.data.id} label="Copy ID" ariaLabel="Copy ID of this job" /> : null}</div>
        <div className="card-pad">
          <Load res={job} rows={3} label="Loading job" errorHeading={job.state === "error" && job.error.code === "job_not_found" ? "No job with that ID" : "The job could not be loaded"}>
            {(j: Job) => (
              <>
                {j.status === "queued" && j.unschedulable_reason ? <div style={{ marginBottom: "var(--space-3)" }}><Notice variant="warning" lead={reasonLabel(j.unschedulable_reason)}>This job is queued but no worker can take it right now. This is a reason, not a state.</Notice></div> : null}
                <DefinitionList
                  items={[
                    { term: "ID", value: <span className="mono">{j.id}</span> },
                    { term: "Status", value: <><StatusBadge state={j.status} />{j.unschedulable_reason ? <> <ReasonChip>{reasonLabel(j.unschedulable_reason)}</ReasonChip></> : null}</> },
                    { term: "Handler", value: <span className="mono">{j.handler_name}@{j.handler_version}</span> },
                    { term: "Priority", value: j.priority },
                    { term: "Attempts", value: `${j.attempts_used} of ${j.max_attempts}` },
                    { term: "Created", value: <Timestamp value={j.created_at} /> },
                    { term: "Updated", value: <Timestamp value={j.updated_at} /> },
                    { term: "Version", value: <span className="mono">{j.version}</span> },
                    { term: "Failure class", value: j.failure_class ?? <span className="meta">None</span> },
                  ]}
                />
                <h3 style={{ margin: "var(--space-5) 0 var(--space-2)", fontSize: "var(--text-ui-lg)" }}>Payload</h3>
                <MaybeJson value={j.payload} masked={j.masked_fields.includes("payload")} empty="No payload" />
                <h3 style={{ margin: "var(--space-5) 0 var(--space-2)", fontSize: "var(--text-ui-lg)" }}>Result</h3>
                <MaybeJson value={j.result} masked={j.masked_fields.includes("result")} empty="Not produced yet" />
                <h3 style={{ margin: "var(--space-5) 0 var(--space-2)", fontSize: "var(--text-ui-lg)" }}>Error</h3>
                {j.masked_fields.includes("error_message") ? <MaskedField /> : j.error_message ? <p>{j.error_message}</p> : <span className="meta">No error</span>}
              </>
            )}
          </Load>
        </div>
      </Card>
      <Card className="sect">
        <div className="card-h"><h2>Attempts</h2></div>
        <div className="card-pad">
          <Load res={attempts} rows={2} label="Loading attempts" errorHeading="Attempts could not be loaded">
            {(a) => (a.items.length ? <>{a.items.map((x) => <AttemptCard key={x.id} a={x} />)}</> : <p className="muted">No attempts yet.</p>)}
          </Load>
        </div>
      </Card>
      <Card className="sect">
        <div className="card-h"><h2>Event timeline</h2></div>
        <div className="card-pad">
          <Load res={events} rows={3} label="Loading events" errorHeading="Events could not be loaded">
            {(e) => (e.items.length ? <Timeline events={e.items} /> : <p className="muted">No events recorded.</p>)}
          </Load>
        </div>
      </Card>
    </>
  );
}
