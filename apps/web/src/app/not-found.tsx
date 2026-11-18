import type { Metadata } from "next";
import Link from "next/link";
import { ButtonLink, PublicFooter, PublicHeader } from "@clusterx/ui";

export const metadata: Metadata = { title: "Page not found", robots: { index: false } };

/** GLOBAL-404: full brand shell, HTTP 404. 1.1 routes (/app/workflows*) land here in 1.0. */
export default function NotFound() {
  return (
    <>
      <PublicHeader />
      <main id="main" tabIndex={-1} className="section-page">
        <div className="wrap main-prose">
          <p className="eyebrow">Error 404</p>
          <h1 className="page-title" style={{ marginTop: "var(--space-4)" }}>That page does not exist.</h1>
          <p className="lede" style={{ marginTop: "var(--space-4)" }}>The address may be mistyped, or the page was never part of CLUSTERX 1.0. Try the home page, the documentation, or the console.</p>
          <div className="cta-row" style={{ display: "flex", gap: "var(--space-3)", flexWrap: "wrap", marginTop: "var(--space-5)" }}>
            <ButtonLink href="/" variant="primary">Home</ButtonLink>
            <ButtonLink href="/docs">Documentation</ButtonLink>
            <ButtonLink href="/app">Console</ButtonLink>
          </div>
          <p className="meta" style={{ marginTop: "var(--space-5)" }}>Looking for something that moved? Start at <Link href="/docs">the docs index</Link>.</p>
        </div>
      </main>
      <PublicFooter />
    </>
  );
}
