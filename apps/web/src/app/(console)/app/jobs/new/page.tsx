import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { Forbidden, NewJobConsole } from "@/features/console-views";
import { getSession, roleOf } from "@/lib/session";

export const metadata: Metadata = { title: "New job: Console" };

export default async function Page() {
  const role = roleOf(await getSession());
  return (
    <>
      <PageHeader title="New job" lead="Submit a job to an allowlisted handler." />
      {role === "viewer" ? <Forbidden role="operator" /> : <NewJobConsole />}
    </>
  );
}
