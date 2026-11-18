import type { Metadata } from "next";
import { DocPage, PrevNext } from "@/features/docs";

export const metadata: Metadata = { title: "Reliability: Docs" };

export default function Page() {
  return (
    <DocPage title="Reliability" lead="What CLUSTERX guarantees, and what it does not.">
      <h2 id="guarantees">Guarantees</h2>
      <ul>
        <li>A job is acknowledged only after its transaction commits.</li>
        <li>Delivery is at-least-once.</li>
        <li>Database writes from a stale attempt, generation or revoked credential are fenced off.</li>
      </ul>
      <h2 id="non-guarantees">Non-guarantees</h2>
      <ul>
        <li>No exactly-once execution of external side effects.</li>
        <li>No strict FIFO order.</li>
        <li>No high availability: there is one coordinator.</li>
        <li>A deadline does not kill a running handler.</li>
      </ul>
      <h2 id="numbers">Measured numbers</h2>
      <p>Not yet measured. Performance and recovery numbers appear here only with a linked benchmark artifact.</p>
      <PrevNext href="/docs/reliability" />
    </DocPage>
  );
}
