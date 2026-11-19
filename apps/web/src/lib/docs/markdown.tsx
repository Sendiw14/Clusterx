import { Fragment, type ReactNode } from "react";
import Link from "next/link";
import { Lexer, type Token, type Tokens } from "marked";
import { CodeBlock, Notice, type NoticeVariant } from "@clusterx/ui";

/**
 * Minimal Markdown to React renderer for the docs (server components only, no raw HTML is ever emitted).
 * Content is repository-authored. Supported: headings 2 to 4, paragraphs, lists, fenced code (copy button),
 * tables (labelled, stacked on narrow screens), blockquotes, GitHub-style callouts, links, emphasis, inline code.
 * Callouts: `> [!VERIFIED]`, `> [!UNVERIFIED]`, `> [!NOTE]`, `> [!WARNING]`.
 */

export interface Heading { id: string; text: string; depth: 2 | 3 }
export interface Section { id: string; heading: string; text: string }
export interface Rendered { nodes: ReactNode; headings: Heading[]; sections: Section[] }

export function slugify(s: string): string {
  return s.toLowerCase().replace(/<[^>]+>/g, "").replace(/[`*_]/g, "").replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "") || "section";
}

/** Plain text of inline tokens, used for the search index and aria labels. */
export function plain(tokens: Token[] | undefined): string {
  if (!tokens) return "";
  return tokens.map((t) => {
    if ("tokens" in t && t.tokens) return plain(t.tokens as Token[]);
    if (t.type === "br") return " ";
    return "text" in t ? String((t as { text: string }).text) : "";
  }).join("");
}

const CALLOUT: Record<string, { variant: NoticeVariant; lead: string }> = {
  VERIFIED: { variant: "success", lead: "Verified." },
  UNVERIFIED: { variant: "warning", lead: "Not yet verified." },
  NOTE: { variant: "info", lead: "Note." },
  WARNING: { variant: "warning", lead: "Careful." },
};

const LANG: Record<string, string> = { sh: "Shell", bash: "Shell", shell: "Shell", go: "Go", json: "JSON", yaml: "YAML", text: "Text", http: "HTTP" };

function inline(tokens: Token[] | undefined, key = "i"): ReactNode {
  if (!tokens) return null;
  return tokens.map((t, i) => {
    const k = `${key}-${i}`;
    switch (t.type) {
      case "strong": return <strong key={k}>{inline((t as Tokens.Strong).tokens, k)}</strong>;
      case "em": return <em key={k}>{inline((t as Tokens.Em).tokens, k)}</em>;
      case "codespan": return <code key={k}>{unescape((t as Tokens.Codespan).text)}</code>;
      case "br": return <br key={k} />;
      case "del": return <del key={k}>{inline((t as Tokens.Del).tokens, k)}</del>;
      case "link": {
        const l = t as Tokens.Link;
        const body = inline(l.tokens, k);
        if (l.href.startsWith("/") || l.href.startsWith("#")) return <Link key={k} href={l.href}>{body}</Link>;
        return <a key={k} href={l.href} rel="noopener noreferrer">{body}<span className="sr-only"> (external)</span></a>;
      }
      case "escape": return <Fragment key={k}>{(t as Tokens.Escape).text}</Fragment>;
      case "text": {
        const x = t as Tokens.Text;
        return x.tokens ? <Fragment key={k}>{inline(x.tokens, k)}</Fragment> : <Fragment key={k}>{unescape(x.text)}</Fragment>;
      }
      default: return <Fragment key={k}>{"raw" in t ? String((t as { raw: string }).raw) : ""}</Fragment>;
    }
  });
}

function unescape(s: string): string {
  return s.replace(/&amp;/g, "&").replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&quot;/g, '"').replace(/&#39;/g, "'");
}

export function renderMarkdown(src: string, opts: { sectionLabel?: string } = {}): Rendered {
  const tokens = new Lexer({ gfm: true }).lex(src);
  const headings: Heading[] = [];
  const sections: Section[] = [];
  const used = new Map<string, number>();
  let current: Section = { id: "", heading: opts.sectionLabel ?? "", text: "" };
  let lastHeading = opts.sectionLabel ?? "this page";
  let tableN = 0;
  const flush = () => { if (current.text.trim() || current.heading) sections.push({ ...current, text: current.text.replace(/\s+/g, " ").trim() }); };
  const addText = (s: string) => { current.text += ` ${s}`; };

  const block = (t: Token, key: string): ReactNode => {
    switch (t.type) {
      case "heading": {
        const h = t as Tokens.Heading;
        const text = plain(h.tokens);
        let id = slugify(text);
        const n = used.get(id) ?? 0;
        used.set(id, n + 1);
        if (n) id = `${id}-${n + 1}`;
        lastHeading = text;
        if (h.depth === 2 || h.depth === 3) {
          headings.push({ id, text, depth: h.depth });
          flush();
          current = { id, heading: text, text: "" };
        }
        const Tag = (`h${Math.min(Math.max(h.depth, 2), 4)}`) as "h2" | "h3" | "h4";
        return (
          <Tag key={key} id={id} className="doc-h">
            {inline(h.tokens, key)}
            <a className="anchor" href={`#${id}`} aria-label={`Link to section: ${text}`}>#</a>
          </Tag>
        );
      }
      case "paragraph": { const p = t as Tokens.Paragraph; addText(plain(p.tokens)); return <p key={key}>{inline(p.tokens, key)}</p>; }
      case "text": { const p = t as Tokens.Text; addText(plain(p.tokens) || p.text); return <Fragment key={key}>{p.tokens ? inline(p.tokens, key) : p.text}</Fragment>; }
      case "list": {
        const l = t as Tokens.List;
        const Tag = l.ordered ? "ol" : "ul";
        return (
          <Tag key={key} {...(l.ordered && l.start && l.start !== 1 ? { start: l.start } : {})}>
            {l.items.map((it, i) => <li key={`${key}-${i}`}>{it.tokens.map((c, j) => block(c, `${key}-${i}-${j}`))}</li>)}
          </Tag>
        );
      }
      case "code": {
        const c = t as Tokens.Code;
        const [lang = "text", ...flags] = (c.lang ?? "text").split(/\s+/);
        addText(c.text);
        const expected = flags.includes("expected");
        return <div key={key} className="doc-code"><CodeBlock code={c.text} language={LANG[lang] ?? lang} label={`${expected ? "Expected output" : "Code"}: ${lastHeading}`} expectedOutput={expected} /></div>;
      }
      case "table": {
        const tb = t as Tokens.Table;
        tableN += 1;
        const heads = tb.header.map((h) => plain(h.tokens));
        addText(heads.join(" "));
        return (
          <div key={key} className="card tbl-wrap doc-table" role="region" tabIndex={0} aria-label={`Table ${tableN}: ${lastHeading}`}>
            <table className="tbl">
              <thead><tr>{tb.header.map((h, i) => <th key={i} scope="col">{inline(h.tokens, `${key}h${i}`)}</th>)}</tr></thead>
              <tbody>
                {tb.rows.map((r, ri) => (
                  <tr key={ri}>
                    {r.map((c, ci) => { addText(plain(c.tokens)); return <td key={ci} data-label={heads[ci]} {...(ci === 0 ? { "data-primary": "" } : {})}>{inline(c.tokens, `${key}c${ri}${ci}`)}</td>; })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        );
      }
      case "blockquote": {
        const q = t as Tokens.Blockquote;
        const first = q.tokens[0];
        const m = first && first.type === "paragraph" ? /^\[!(VERIFIED|UNVERIFIED|NOTE|WARNING)\]\s*/.exec((first as Tokens.Paragraph).text) : null;
        if (m && first) {
          const c = CALLOUT[m[1]!]!;
          const p = first as Tokens.Paragraph;
          const rest = p.tokens.slice();
          const f0 = rest[0];
          if (f0 && f0.type === "text") rest[0] = { ...f0, text: (f0 as Tokens.Text).text.replace(/^\[!\w+\]\s*/, ""), raw: "", tokens: undefined } as Token;
          addText(`${c.lead} ${plain(rest)}`);
          return (
            <div key={key} className="doc-callout">
              <Notice variant={c.variant} lead={c.lead} role="none">{inline(rest, key)}{q.tokens.slice(1).map((x, i) => block(x, `${key}-${i}`))}</Notice>
            </div>
          );
        }
        return <blockquote key={key}>{q.tokens.map((x, i) => block(x, `${key}-${i}`))}</blockquote>;
      }
      case "hr": return <hr key={key} className="tick-rule" />;
      case "space": return null;
      default: return null;
    }
  };

  const nodes = tokens.map((t, i) => block(t, `b${i}`));
  flush();
  return { nodes, headings, sections: sections.filter((s) => s.heading || s.text) };
}
