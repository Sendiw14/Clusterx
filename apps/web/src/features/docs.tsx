import type { ReactNode } from "react";
import Link from "next/link";
import { Notice } from "@clusterx/ui";

export const DOC_ORDER = [
  { href: "/docs/quickstart", title: "Quickstart", blurb: "From a clean checkout to a first job, three workers and a recovery demonstration." },
  { href: "/docs/concepts", title: "Concepts", blurb: "Job states, attempts, leases, fencing, at-least-once and priorities." },
  { href: "/docs/workers", title: "Worker SDK", blurb: "Register handlers, error types, timeouts and worker credentials." },
  { href: "/docs/api", title: "API reference", blurb: "The versioned REST contract, authentication and the error model." },
  { href: "/docs/reliability", title: "Reliability", blurb: "Guarantees and non-guarantees, retry budgets and recovery." },
  { href: "/docs/deployment", title: "Deployment", blurb: "Single-VM reference, TLS, backups and upgrades." },
  { href: "/docs/security", title: "Security", blurb: "Authentication, roles, worker trust and the demo boundary." },
] as const;

/** Shared docs page frame: one h1, a lede, an honest status notice where content is not final. */
export function DocPage({ title, lead, pending, children }: { title: string; lead: string; pending?: string; children?: ReactNode }) {
  return (
    <>
      <h1>{title}</h1>
      <p className="lede">{lead}</p>
      {pending ? <Notice variant="info" lead="Documentation in progress." role="none">{pending}</Notice> : null}
      {children}
    </>
  );
}

export function PrevNext({ href }: { href: string }) {
  const i = DOC_ORDER.findIndex((d) => d.href === href);
  const prev = DOC_ORDER[i - 1];
  const next = DOC_ORDER[i + 1];
  return (
    <nav aria-label="Previous and next page" style={{ display: "flex", justifyContent: "space-between", gap: "var(--space-4)", marginTop: "var(--space-8)", paddingTop: "var(--space-4)", borderTop: "1px solid var(--color-line)", flexWrap: "wrap" }}>
      {prev ? <Link href={prev.href}>Previous: {prev.title}</Link> : <span />}
      {next ? <Link href={next.href}>Next: {next.title}</Link> : <span />}
    </nav>
  );
}
