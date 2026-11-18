import type { ReactNode } from "react";
import { DocsSidebar } from "@clusterx/ui";

export default function DocsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="wrap docs">
      <DocsSidebar />
      <article className="docs-article">{children}</article>
    </div>
  );
}
