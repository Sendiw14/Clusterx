import type { Metadata } from "next";
import { EmptyState } from "@clusterx/ui";

export const metadata: Metadata = { title: "Changelog", description: "Shipped CLUSTERX releases and migration notes." };

/** Pre-release: no fictitious versions. The list renders real releases once CHANGELOG.md has any. */
export default function ChangelogPage() {
  return (
    <div className="section-page">
      <div className="wrap main-prose">
        <p className="eyebrow">Changelog</p>
        <h1 className="page-title" style={{ marginTop: "var(--space-4)" }}>Releases</h1>
        <p className="muted" style={{ marginTop: "var(--space-3)" }}>Dates are in UTC.</p>
        <EmptyState title="No releases yet." icon="i-clock">CLUSTERX 1.0 has not shipped. Release notes and migration notes will appear here, version by version.</EmptyState>
      </div>
    </div>
  );
}
