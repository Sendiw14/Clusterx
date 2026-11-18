import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { Pending } from "@/features/console-views";

export const metadata: Metadata = { title: "Activity: Console" };

export default function Page() {
  return (
    <>
      <PageHeader title="Activity" lead="Job and audit events, newest first." />
      <Pending what="The activity timeline" />
    </>
  );
}
