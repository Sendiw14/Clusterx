import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { ObservabilityConsole } from "@/features/console-views";

export const metadata: Metadata = { title: "Observability: Console" };

export default function Page() {
  return (
    <>
      <PageHeader title="Observability" lead="Throughput, queue age, attempts, failures and duration." />
      <ObservabilityConsole />
    </>
  );
}
