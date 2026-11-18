import type { Metadata } from "next";
import { StatusLegend, JOB_STATES } from "@clusterx/ui";
import { DocPage, PrevNext } from "@/features/docs";

export const metadata: Metadata = { title: "Concepts: Docs" };

export default function Page() {
  return (
    <DocPage title="Concepts" lead="The vocabulary exactly as the engine uses it.">
      <h2 id="states">Job states</h2>
      <p>A job has exactly seven states in 1.0. The same glyph and words appear everywhere in the product; this list is the canonical legend.</p>
      <StatusLegend states={JOB_STATES} label="Job state legend" />
      <h2 id="reasons">Reasons and views, not states</h2>
      <ul>
        <li><strong>Unschedulable</strong> is a reason shown beside a queued job when no worker can take it. It is not a state.</li>
        <li><strong>Dead-letter</strong> is a filtered view of jobs whose status is <code>failed</code>. It is not a state.</li>
        <li><code>blocked</code> and <code>skipped</code> are reserved for a later release and do not exist in 1.0.</li>
      </ul>
      <h2 id="attempts">Attempts and leases</h2>
      <p>Each claim creates an attempt and a lease generation in one transaction. A worker must renew its lease; if it stops, the lease expires on the database clock and the job can be retried while attempt budget remains.</p>
      <h2 id="fencing">Fencing</h2>
      <p>A result is accepted only for the current attempt, lease generation and worker identity. A late report from a worker that lost its lease is rejected and recorded as a diagnostic event.</p>
      <h2 id="at-least-once">At-least-once</h2>
      <p>A handler can run more than once, for example when a worker goes silent while still running. Make external side effects idempotent. The engine does not promise exactly-once execution.</p>
      <h2 id="priority">Priority and due jobs</h2>
      <p>Priority is 0 to 9 and 9 is the highest. Due jobs are claimed by priority, due time and creation time, which is not strict FIFO.</p>
      <PrevNext href="/docs/concepts" />
    </DocPage>
  );
}
