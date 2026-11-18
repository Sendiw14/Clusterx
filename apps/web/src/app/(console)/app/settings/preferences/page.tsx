import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { PreferencesConsole } from "@/features/console-views";

export const metadata: Metadata = { title: "Preferences: Console" };

export default function Page() {
  return (
    <>
      <PageHeader title="Preferences" lead="Stored in this browser only." />
      <PreferencesConsole />
    </>
  );
}
