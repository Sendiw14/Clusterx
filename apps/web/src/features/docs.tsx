import type { ReactNode } from "react";
import Link from "next/link";
import { allDocMeta, loadDoc, type DocSlug } from "@/lib/docs/content";
import { renderMarkdown, type Heading } from "@/lib/docs/markdown";
import { OnThisPage } from "@/features/on-this-page";

/** Shared docs page frame: one h1, a lede, the article, and the on-this-page rail. Content never depends on script. */
export function DocFrame({ title, lead, headings, children, href }: { title: string; lead: string; headings: Heading[]; children: ReactNode; href: string }) {
  return (
    <div className="docs-cols">
      <article className="docs-article" data-doc={href}>
        <h1>{title}</h1>
        <p className="lede">{lead}</p>
        <OnThisPage headings={headings} variant="inline" />
        {children}
        <PrevNext href={href} />
      </article>
      <OnThisPage headings={headings} variant="rail" />
    </div>
  );
}

/** A docs page authored in content/docs/<slug>.md. */
export function MarkdownDoc({ slug }: { slug: DocSlug }) {
  const d = loadDoc(slug);
  const r = renderMarkdown(d.body, { sectionLabel: d.title });
  return <DocFrame title={d.title} lead={d.lead} headings={r.headings} href={`/docs/${slug}`}>{r.nodes}</DocFrame>;
}

export function docMetadata(slug: DocSlug) {
  const d = loadDoc(slug);
  return { title: `${d.title}: Docs`, description: d.description };
}

export function PrevNext({ href }: { href: string }) {
  const all = allDocMeta();
  const i = all.findIndex((d) => `/docs/${d.slug}` === href);
  const prev = all[i - 1];
  const next = all[i + 1];
  return (
    <nav className="prevnext" aria-label="Previous and next page">
      {prev ? <Link href={`/docs/${prev.slug}`} rel="prev"><span>Previous</span>{prev.title}</Link> : <span />}
      {next ? <Link href={`/docs/${next.slug}`} rel="next" className="next"><span>Next</span>{next.title}</Link> : <span />}
    </nav>
  );
}
