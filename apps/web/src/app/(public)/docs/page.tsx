import type { Metadata } from "next";
import Link from "next/link";
import { Notice } from "@clusterx/ui";
import { allDocMeta } from "@/lib/docs/content";

export const metadata: Metadata = {
  title: "Docs",
  description: "CLUSTERX documentation: quickstart, concepts, worker SDK, generated API reference, reliability, deployment and security.",
};

const CONCEPTS = [
  { href: "/docs/concepts#job-states", t: "Job states", d: "Seven states, three terminal. Unschedulable and dead-letter are views, not states." },
  { href: "/docs/concepts#jobs-and-attempts", t: "Jobs and attempts", d: "One job, several attempts, one attempt budget that includes the first run." },
  { href: "/docs/concepts#leases-and-heartbeats", t: "Leases", d: "Time-limited right to run an attempt, judged on the database clock." },
  { href: "/docs/concepts#fencing", t: "Fencing", d: "How a late report from a lost lease is refused." },
  { href: "/docs/concepts#at-least-once-delivery", t: "At-least-once", d: "A handler can run twice. Make side effects idempotent." },
  { href: "/docs/concepts#due-jobs-and-priority", t: "Priority and due jobs", d: "0 to 9, nine highest. Not strict FIFO." },
  { href: "/docs/concepts#cancellation", t: "Cancellation", d: "Cooperative. Whichever commits first, cancel or success, wins." },
  { href: "/docs/concepts#idempotent-submission", t: "Idempotency", d: "Idempotency-Key, 24 hours, replay or conflict." },
];

export default function DocsIndex() {
  const pages = allDocMeta();
  return (
    <div className="docs-cols docs-index">
      <article className="docs-article" style={{ maxWidth: "52rem" }}>
        <h1>Documentation</h1>
        <p className="lede">How to run CLUSTERX, how jobs move through it, what it guarantees and what it does not. Pages say which commands were actually run.</p>
        <p className="muted">Press <kbd>/</kbd> or use the search field to search every page and every API operation. The index is built with the site; nothing is sent to an external service.</p>

        <h2 id="first-steps" className="doc-h">Guided first steps</h2>
        <ol className="steps-guide">
          <li><Link href="/docs/quickstart">Run the local stack</Link>: three workers in Docker Compose, then submit a job with <code>curl</code>.</li>
          <li><Link href="/docs/concepts">Learn the vocabulary</Link>: states, attempts, leases and fencing.</li>
          <li><Link href="/docs/reliability">Read the guarantees and the non-guarantees</Link> before you build on it.</li>
          <li><Link href="/docs/workers">Write a handler</Link> with the Go SDK.</li>
          <li><Link href="/docs/api">Use the API reference</Link>, generated from the OpenAPI document.</li>
        </ol>

        <h2 id="concepts-map" className="doc-h">Core concepts</h2>
        <ul className="concept-map">
          {CONCEPTS.map((c) => <li key={c.href}><Link href={c.href}>{c.t}</Link><p>{c.d}</p></li>)}
        </ul>

        <h2 id="all-pages" className="doc-h">All pages</h2>
        <ul className="page-cards">
          {pages.map((p) => (
            <li key={p.slug} className="card card-pad"><h3 style={{ fontSize: "var(--text-step-1)" }}><Link href={`/docs/${p.slug}`}>{p.title}</Link></h3><p className="muted" style={{ margin: "var(--space-1) 0 0" }}>{p.blurb}</p></li>
          ))}
        </ul>

        <h2 id="evidence" className="doc-h">How to read the evidence markers</h2>
        <div className="doc-callout"><Notice variant="success" lead="Verified." role="none">Someone ran it and the result is recorded. The note says who and where.</Notice></div>
        <div className="doc-callout"><Notice variant="warning" lead="Not yet verified." role="none">Specified or written down, but no run has confirmed it. Treat it as intent.</Notice></div>
        <p>Anything without a marker is either a plain statement of design or a limit. The production deployment section is entirely unverified. If you find a claim that no source supports, that is a documentation bug.</p>
      </article>
    </div>
  );
}
