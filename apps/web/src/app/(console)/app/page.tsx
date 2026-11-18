import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { OverviewConsole } from "@/features/console-views";

export const metadata: Metadata = { title: "Overview: Console" };

export default function Page() {
  return (
    <>
      <PageHeader title="Overview" lead="Is the cluster healthy, what is stuck, what needs you." />
      <OverviewConsole />
    </>
  );
}
