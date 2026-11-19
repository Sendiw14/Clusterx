import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { ActivityConsole } from "@/features/console-views";

export const metadata: Metadata = { title: "Activity: Console" };

export default function Page() {
  return (
    <>
      <PageHeader title="Activity" lead="Privileged, security and diagnostic events, newest first." />
      <ActivityConsole />
    </>
  );
}
