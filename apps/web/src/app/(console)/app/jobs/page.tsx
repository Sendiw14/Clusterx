import type { Metadata } from "next";
import { Suspense } from "react";
import { ButtonLink, PageHeader } from "@clusterx/ui";
import { JobsConsole } from "@/features/console-views";
import { getSession, roleOf } from "@/lib/session";

export const metadata: Metadata = { title: "Jobs: Console" };

export default async function Page() {
  const role = roleOf(await getSession());
  return (
    <>
      <PageHeader title="Jobs" lead="Created in the last 7 days (default window; at most 31 days)." actions={role === "operator" ? <ButtonLink href="/app/jobs/new" variant="primary" icon="i-plus">Submit job</ButtonLink> : null} />
      <Suspense fallback={null}><JobsConsole /></Suspense>
    </>
  );
}
