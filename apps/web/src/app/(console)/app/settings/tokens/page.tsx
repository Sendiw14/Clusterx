import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { Forbidden, TokensConsole } from "@/features/console-views";
import { getSession, roleOf } from "@/lib/session";

export const metadata: Metadata = { title: "Tokens and credentials: Console" };

export default async function Page() {
  const role = roleOf(await getSession());
  return (
    <>
      <PageHeader title="Tokens and credentials" lead="API tokens and worker credentials. Operators only." />
      {role === "viewer" ? <Forbidden role="operator" /> : <TokensConsole />}
    </>
  );
}
