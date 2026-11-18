import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { WorkersConsole } from "@/features/console-views";

export const metadata: Metadata = { title: "Workers: Console" };

export default function Page() {
  return (
    <>
      <PageHeader title="Workers" lead="Inventory, liveness, capacity and handlers." />
      <WorkersConsole />
    </>
  );
}
