import type { Metadata } from "next";
import Link from "next/link";
import { DOC_ORDER, DocPage } from "@/features/docs";

export const metadata: Metadata = { title: "Docs", description: "CLUSTERX documentation index." };

export default function DocsIndex() {
  return (
    <DocPage title="Documentation" lead="Start with the quickstart, then read how jobs, leases and fencing work." pending="Pages below hold the facts that are settled. Full guides, search and verified commands are written with the documentation milestone.">
      <h2>Start here</h2>
      <ol>
        <li><Link href="/docs/quickstart">Quickstart</Link></li>
        <li><Link href="/docs/concepts">Concepts</Link></li>
        <li><Link href="/docs/workers">Worker SDK</Link></li>
        <li><Link href="/docs/api">API reference</Link></li>
      </ol>
      <h2>All pages</h2>
      <ul style={{ listStyle: "none", padding: 0, display: "grid", gap: "var(--space-3)" }}>
        {DOC_ORDER.map((d) => (
          <li key={d.href} className="card card-pad"><h3 style={{ fontSize: "var(--text-step-1)" }}><Link href={d.href}>{d.title}</Link></h3><p className="muted" style={{ margin: "var(--space-1) 0 0" }}>{d.blurb}</p></li>
        ))}
      </ul>
    </DocPage>
  );
}
