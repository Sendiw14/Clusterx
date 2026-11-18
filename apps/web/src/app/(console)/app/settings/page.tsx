import type { Metadata } from "next";
import { PageHeader } from "@clusterx/ui";
import { SettingsConsole } from "@/features/console-views";

export const metadata: Metadata = { title: "Configuration: Console" };

export default function Page() {
  return (
    <>
      <PageHeader title="Configuration" lead="Read-only environment, limits and retention." />
      <SettingsConsole />
    </>
  );
}
