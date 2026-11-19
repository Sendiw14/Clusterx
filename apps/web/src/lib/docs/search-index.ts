import { DOC_SLUGS, loadDoc } from "./content";
import { renderMarkdown } from "./markdown";
import { loadApi } from "./openapi";

/** One searchable section. Short keys keep the static JSON small. */
export interface SearchEntry { u: string; p: string; h: string; t: string }

const clip = (s: string, n = 360) => (s.length > n ? `${s.slice(0, n - 1).trimEnd()}...` : s);

/** Built once at build time from the same sources the pages render from (no external service). */
export function buildSearchIndex(): SearchEntry[] {
  const out: SearchEntry[] = [];
  for (const slug of DOC_SLUGS) {
    const d = loadDoc(slug);
    if (slug === "api") {
      const api = loadApi();
      out.push({ u: "/docs/api", p: d.title, h: d.title, t: clip(`${d.lead} ${api.title} ${api.version}`) });
      for (const s of renderMarkdown(d.body, { sectionLabel: d.title }).sections) out.push({ u: s.id ? `/docs/api#${s.id}` : "/docs/api", p: d.title, h: s.heading || d.title, t: clip(s.text) });
      for (const o of api.operations) out.push({ u: `/docs/api#${o.id}`, p: d.title, h: `${o.method} ${o.path} (${o.id})`, t: clip(`${o.summary} ${o.params.map((x) => x.name).join(" ")} ${o.requestRows.map((r) => r.name).join(" ")}`) });
      for (const s of api.schemas) out.push({ u: `/docs/api#schema-${s.name}`, p: d.title, h: `Schema ${s.name}`, t: clip(`${s.description} ${s.rows.map((r) => r.name).join(" ")} ${s.enumValues.join(" ")}`) });
      continue;
    }
    out.push({ u: `/docs/${slug}`, p: d.title, h: d.title, t: clip(d.lead) });
    for (const s of renderMarkdown(d.body, { sectionLabel: d.title }).sections) out.push({ u: s.id ? `/docs/${slug}#${s.id}` : `/docs/${slug}`, p: d.title, h: s.heading || d.title, t: clip(s.text) });
  }
  return out;
}
