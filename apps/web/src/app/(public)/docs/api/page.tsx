import type { Metadata } from "next";
import { DocFrame } from "@/features/docs";
import { ApiReference } from "@/features/api-reference";
import { docMetadata } from "@/features/docs";
import { loadDoc } from "@/lib/docs/content";
import { loadApi } from "@/lib/docs/openapi";
import { renderMarkdown, type Heading } from "@/lib/docs/markdown";

export const metadata: Metadata = docMetadata("api");

export default function Page() {
  const d = loadDoc("api");
  const api = loadApi();
  const md = renderMarkdown(d.body, { sectionLabel: d.title });
  const generated: Heading[] = [
    { id: "contract", text: "This build's contract", depth: 2 },
    { id: "security-schemes", text: "Security schemes", depth: 2 },
    { id: "error-codes", text: "Error codes and statuses", depth: 2 },
    { id: "operations", text: "Operations", depth: 2 },
    ...api.tags.filter((t) => api.operations.some((o) => o.tag === t.name)).map((t) => ({ id: `tag-${t.name}`, text: t.name[0]!.toUpperCase() + t.name.slice(1), depth: 3 as const })),
    { id: "schemas", text: "Schemas", depth: 2 },
  ];
  return (
    <DocFrame title={d.title} lead={d.lead} headings={[...md.headings, ...generated]} href="/docs/api">
      {md.nodes}
      <ApiReference api={api} />
    </DocFrame>
  );
}
