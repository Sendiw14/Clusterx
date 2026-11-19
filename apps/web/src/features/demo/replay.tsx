"use client";
import { useState } from "react";
import { Card, Icon, StatusBadge, Timestamp } from "@clusterx/ui";
import { useDataProvider } from "@/data/context";
import type { JobEvent } from "@/data/provider";
import { Load } from "../common";
import "@/features/home.css";
import { useResource } from "../use-resource";

const JOB_ID = "demo-job-0002";

interface Step { time: JobEvent["occurred_at"] | null; title: string; text: string; status: JobEvent["to_status"]; a: string; b: string; budget: string; rejected?: boolean; scripted?: boolean }

/** Turns the fixture's recorded events for one job into replay steps. Worker and budget labels follow the events. */
function steps(events: JobEvent[]): Step[] {
  const out: Step[] = [];
  let a = "idle", b = "idle", used = 0;
  for (const e of events) {
    const who = e.actor.replace("worker:", "");
    let title = e.event_type.replace(/_/g, " ");
    let text = `Recorded event v${e.version}: ${e.event_type.replace(/_/g, " ")}, job is now ${e.to_status.replace("_", " ")}.`;
    if (e.event_type === "admitted") { title = "Admitted"; text = "The job is committed to PostgreSQL and queued. Nothing was acknowledged before the commit."; }
    if (e.event_type === "claimed") {
      used += 1;
      if (who.endsWith("1")) { a = "running attempt 1"; title = "Claimed by worker A"; text = "Worker A takes the job: attempt 1, lease generation 1."; }
      else { b = "running attempt 2"; title = "Claimed by worker B"; text = "Worker B takes the job: attempt 2, lease generation 2."; }
    }
    if (e.event_type === "lease_expired") { a = "silent, lease expired"; title = "Lease expired"; text = "Worker A stopped renewing. On the database clock its lease ran out, attempt 1 is closed as lease_expired and counts against the budget. The job waits for backoff."; }
    if (e.event_type === "promoted") { title = "Back in the queue"; text = "Backoff passed. Budget remains, so the job is queued again."; }
    if (e.event_type === "succeeded") { b = "finished attempt 2"; title = "Result committed"; text = "Worker B's result is accepted: attempt, generation and worker identity are all current."; }
    out.push({ time: e.occurred_at, title, text, status: e.to_status, a, b, budget: `${used} of 3 attempts used` });
  }
  const last = out[out.length - 1];
  if (last) out.push({ time: null, scripted: true, rejected: true, title: "Late report from worker A: rejected", text: "Worker A comes back and reports a result for attempt 1. The fence refuses it because that attempt is stale; the committed result stands. The fixtures hold no record of this step, so it is scripted here to complete the story. The handler ran twice; the record was never overwritten.", status: last.status, a: "late report rejected", b: last.b, budget: last.budget });
  return out;
}

/**
 * Worker-failure recovery replay on fixtures: the recorded events of one demo job, revealed one step at a time by the
 * visitor. No timers, no animation, no network: Step, Show all and Reset are the only controls.
 */
export function RecoveryReplay() {
  const provider = useDataProvider();
  const ev = useResource(() => provider.listJobEvents(JOB_ID), `replay-${provider.kind}`);
  const [n, setN] = useState(0);
  return (
    <Card>
      <div className="card-h"><h2>Recovery replay</h2><span className="meta">Illustrative: replays fixture events of <span className="mono">{JOB_ID}</span></span></div>
      <div className="card-pad">
        <Load res={ev} rows={3} label="Loading replay" errorHeading="The replay could not be loaded">
          {(e) => {
            const all = steps(e.items);
            const cur = n > 0 ? all[n - 1] : undefined;
            return (
              <div className="replay">
                <p className="muted">Worker A claims a job and then goes silent. Step through what the engine does about it. The same sequence is drawn as a diagram on the home page.</p>
                <div className="replay-ctl" role="group" aria-label="Replay controls">
                  <button type="button" className="btn btn-sm" disabled={n >= all.length} onClick={() => setN((x) => Math.min(x + 1, all.length))}><Icon name="i-step" />Step</button>
                  <button type="button" className="btn btn-sm" disabled={n >= all.length} onClick={() => setN(all.length)}>Show all</button>
                  <button type="button" className="btn btn-sm" disabled={n === 0} onClick={() => setN(0)}><Icon name="i-reset" />Reset</button>
                  <span className="meta">Step {n} of {all.length}</span>
                </div>
                <dl className="replay-state" aria-label="State at this step">
                  <div><dt>Job</dt><dd>{cur ? <StatusBadge state={cur.status} /> : <span className="meta">Not started</span>}</dd></div>
                  <div><dt>Worker A</dt><dd>{cur?.a ?? "idle"}</dd></div>
                  <div><dt>Worker B</dt><dd>{cur?.b ?? "idle"}</dd></div>
                  <div><dt>Budget</dt><dd>{cur?.budget ?? "0 of 3 attempts used"}</dd></div>
                </dl>
                <div className="replay-cap" role="status" aria-live="polite">
                  {cur ? <><b>Step {n} of {all.length}: {cur.title}</b><span>{cur.text}</span></> : <span>Press Step to begin.</span>}
                </div>
                <ol className="timeline" aria-label="Replay timeline">
                  {all.slice(0, n).map((s, i) => (
                    <li key={i} data-kind={s.rejected ? "bad" : s.status === "succeeded" ? "ok" : s.title === "Lease expired" ? "warn" : undefined} aria-current={i === n - 1 ? "step" : undefined}>
                      <span className="t">{s.time ? <Timestamp value={s.time} /> : "scripted"}</span>
                      <p><strong>{s.title}.</strong> <span className="meta">{s.scripted ? "Scripted, not a fixture record." : "Fixture event."}</span></p>
                    </li>
                  ))}
                </ol>
              </div>
            );
          }}
        </Load>
      </div>
    </Card>
  );
}
