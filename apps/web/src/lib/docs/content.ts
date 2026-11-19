import { readFileSync } from "node:fs";
import path from "node:path";

/** Reads repository-authored docs at build time. Runs only on the server (static generation). */
export interface DocMeta { slug: string; title: string; lead: string; description: string; blurb: string; order: number }
export interface DocSource extends DocMeta { body: string }

export const DOC_SLUGS = ["quickstart", "concepts", "api", "workers", "reliability", "deployment", "security"] as const;
export type DocSlug = (typeof DOC_SLUGS)[number];

const DIR = path.join(process.cwd(), "content", "docs");

export function parseFrontmatter(raw: string): { data: Record<string, string>; body: string } {
  const m = /^---\n([\s\S]*?)\n---\n?/.exec(raw);
  if (!m) return { data: {}, body: raw };
  const data: Record<string, string> = {};
  for (const line of m[1]!.split("\n")) {
    const i = line.indexOf(":");
    if (i > 0) data[line.slice(0, i).trim()] = line.slice(i + 1).trim();
  }
  return { data, body: raw.slice(m[0].length) };
}

export function loadDoc(slug: DocSlug): DocSource {
  const { data, body } = parseFrontmatter(readFileSync(path.join(DIR, `${slug}.md`), "utf8"));
  for (const k of ["title", "lead", "description", "blurb", "order"]) if (!data[k]) throw new Error(`content/docs/${slug}.md: frontmatter "${k}" is required`);
  return { slug, title: data.title!, lead: data.lead!, description: data.description!, blurb: data.blurb!, order: Number(data.order), body };
}

export function allDocMeta(): DocMeta[] {
  return DOC_SLUGS.map((s) => { const { body: _b, ...meta } = loadDoc(s); return meta; }).sort((a, b) => a.order - b.order);
}
