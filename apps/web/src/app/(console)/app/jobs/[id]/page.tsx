import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { JobDetailConsole } from "@/features/console-views";

export const metadata: Metadata = { title: "Job: Console" };

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <>
      <PageHeader title="Job" lead="Status, attempts and the event timeline." />
      <JobDetailConsole id={id} />
    </>
  );
}
