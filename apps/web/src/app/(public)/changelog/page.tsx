import type { Metadata } from "next";
import { readFileSync } from "node:fs";
import path from "node:path";
import { Notice } from "@clusterx/ui";
import { renderMarkdown } from "@/lib/docs/markdown";

export const metadata: Metadata = { title: "Changelog", description: "What has been merged into CLUSTERX so far. No release has shipped yet." };

/** Real history only: rendered from content/changelog.md, which lists merged milestones and no invented versions. */
export default function ChangelogPage() {
  const r = renderMarkdown(readFileSync(path.join(process.cwd(), "content", "changelog.md"), "utf8"), { sectionLabel: "Changelog" });
  return (
    <div className="section-page">
      <div className="wrap">
        <div className="docs-article changelog" style={{ marginInline: "auto" }}>
          <p className="eyebrow">Changelog</p>
          <h1 className="page-title" style={{ marginTop: "var(--space-4)" }}>Changelog</h1>
          <div style={{ marginTop: "var(--space-5)" }}>
            <Notice variant="info" lead="No releases yet." role="none">CLUSTERX 1.0 has not shipped. Release notes and migration notes will appear here, version by version, once there is a version.</Notice>
          </div>
          {r.nodes}
        </div>
      </div>
    </div>
  );
}
