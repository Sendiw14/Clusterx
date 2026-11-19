import type { ReactNode } from "react";
import { DocsSidebar } from "@clusterx/ui";
import { DocsSearch } from "@/features/docs-search";

export default function DocsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="wrap docs">
      <DocsSidebar search={<DocsSearch />} />
      {children}
    </div>
  );
}
