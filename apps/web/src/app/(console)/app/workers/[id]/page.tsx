import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { WorkerDetailConsole } from "@/features/console-views";

export const metadata: Metadata = { title: "Worker: Console" };

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <>
      <PageHeader title="Worker" lead="Identity, capacity and active attempts." />
      <WorkerDetailConsole id={id} />
    </>
  );
}
