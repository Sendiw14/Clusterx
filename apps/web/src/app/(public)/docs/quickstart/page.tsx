import type { Metadata } from "next";
import { Notice } from "@clusterx/ui";
import { DocPage, PrevNext } from "@/features/docs";

export const metadata: Metadata = { title: "Quickstart: Docs" };

export default function Page() {
  return (
    <DocPage title="Quickstart" lead="Clean checkout to a first job, three workers and a recovery demonstration.">
      <Notice variant="warning" lead="Not yet verified from a clean checkout." role="none">Commands appear here only after a clean-checkout run has passed. Until then this page lists what you will need and what the finished flow shows.</Notice>
      <h2>Prerequisites</h2>
      <ul>
        <li>Docker with the Compose plugin (<code>docker compose</code>, Compose V2).</li>
        <li>A Linux or macOS machine for the local reference stack.</li>
      </ul>
      <h2>What the finished flow shows</h2>
      <ol>
        <li>Start the stack: database, control plane, console and three workers.</li>
        <li>Submit a first job and watch it complete.</li>
        <li>Stop a worker, watch its lease expire, and see the job retried on another worker.</li>
        <li>Watch a late report from the stopped worker being rejected.</li>
      </ol>
      <PrevNext href="/docs/quickstart" />
    </DocPage>
  );
}
