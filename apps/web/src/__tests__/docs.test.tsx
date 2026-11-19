import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { HOME_QUICKSTART, START_STACK, SUBMIT_JOB } from "@/content/commands";
import { search } from "@/features/docs-search";
import { DOC_SLUGS, loadDoc, parseFrontmatter } from "@/lib/docs/content";
import { renderMarkdown, slugify } from "@/lib/docs/markdown";
import { EXECUTED_IN_CURL_GUIDE, loadApi } from "@/lib/docs/openapi";
import { buildSearchIndex } from "@/lib/docs/search-index";

const ROOT = path.join(process.cwd(), "..", "..");

describe("markdown renderer", () => {
  it("parses frontmatter and requires the keys", () => {
    expect(parseFrontmatter("---\ntitle: A\norder: 2\n---\nbody").data).toEqual({ title: "A", order: "2" });
    for (const s of DOC_SLUGS) expect(loadDoc(s).title.length).toBeGreaterThan(2);
  });
  it("gives headings unique stable ids and collects sections", () => {
    const r = renderMarkdown("## Same\n\ntext one\n\n## Same\n\ntext two\n\n### Child `code`\n\nx");
    expect(r.headings.map((h) => h.id)).toEqual(["same", "same-2", "child-code"]);
    expect(r.sections.map((s) => s.text)).toEqual(["text one", "text two", "x"]);
    expect(slugify("Hello, World!")).toBe("hello-world");
  });
  it("renders callouts with a text lead, never raw HTML", () => {
    const html = renderToStaticMarkup(<>{renderMarkdown("> [!UNVERIFIED] Not run.\n\n<script>alert(1)</script>\n\n<b>x</b>").nodes}</>);
    expect(html).toContain("Not yet verified.");
    expect(html).not.toContain("<script");
    expect(html).not.toContain("<b>");
  });
  it("labels tables and code blocks for assistive technology", () => {
    const html = renderToStaticMarkup(<>{renderMarkdown("## Things\n\n| A | B |\n| - | - |\n| 1 | 2 |\n\n```sh\necho hi\n```").nodes}</>);
    expect(html).toContain('aria-label="Table 1: Things"');
    expect(html).toContain('data-label="B"');
    expect(html).toContain("Copy: Code: Things");
  });
});

describe("docs content is honest and consistent", () => {
  const all = DOC_SLUGS.map((s) => ({ s, text: loadDoc(s).body }));
  it("quickstart contains every command the home page shows", () => {
    const qs = all.find((d) => d.s === "quickstart")!.text;
    for (const line of [...START_STACK.split("\n"), ...SUBMIT_JOB.split("\n")].filter((l) => l.trim())) expect(qs, line).toContain(line);
    expect(HOME_QUICKSTART).toContain("./infra/compose/bootstrap.sh");
  });
  it("makes no unsupported claim", () => {
    for (const { s, text } of all) {
      expect(text, s).not.toMatch(/production[- ]ready|battle[- ]tested|enterprise[- ]grade|SOC ?2|ISO ?27001|trusted by|\b99\.\d+ ?%/i);
      expect(text, s).not.toMatch(/(provides?|offers?|guarantees?) (exactly[- ]once|high availability|zero downtime)/i);
    }
    expect(all.find((d) => d.s === "deployment")!.text).toMatch(/not yet verified/i);
  });
  it("every internal link resolves to a real route and, for docs, a real heading", () => {
    const routes = new Set(["/", "/architecture", "/docs", "/changelog", "/demo", "/login", ...DOC_SLUGS.map((s) => `/docs/${s}`)]);
    const ids: Record<string, Set<string>> = {};
    for (const { s, text } of all) ids[`/docs/${s}`] = new Set(renderMarkdown(text).headings.map((h) => h.id));
    for (const { s, text } of all) {
      for (const m of text.matchAll(/\]\((\/[^)\s]*)\)/g)) {
        const [p, hash] = m[1]!.split("#");
        expect(routes.has(p!), `${s}: ${m[1]}`).toBe(true);
        if (hash && ids[p!]) expect(ids[p!]!.has(hash), `${s}: ${m[1]}`).toBe(true);
      }
    }
  });
  it("content directory holds only the expected docs", () => {
    expect(readdirSync(path.join(process.cwd(), "content", "docs")).sort()).toEqual(DOC_SLUGS.map((s) => `${s}.md`).sort());
  });
});

describe("API reference generation", () => {
  const api = loadApi();
  const raw = readFileSync(path.join(ROOT, "api", "openapi.yaml"), "utf8");
  it("reads the version and every operation from the spec", () => {
    expect(raw).toContain(`version: ${api.version}`);
    const opIds = [...raw.matchAll(/operationId: (\w+)/g)].map((m) => m[1]);
    expect(api.operations.map((o) => o.id).sort()).toEqual([...opIds].sort());
    expect(api.operations.length).toBeGreaterThan(25);
  });
  it("marks served operations from the spec's own statement", () => {
    expect(api.servedIds).toContain("createJob");
    expect(api.servedIds).not.toContain("cancelJob");
    expect(api.operations.find((o) => o.id === "cancelJob")!.served).toBe(false);
    expect(api.operations.find((o) => o.id === "createJob")!.served).toBe(true);
    for (const id of EXECUTED_IN_CURL_GUIDE) expect(api.operations.some((o) => o.id === id), id).toBe(true);
  });
  it("extracts request fields, responses, examples and the error catalogue", () => {
    const c = api.operations.find((o) => o.id === "createJob")!;
    expect(c.requestRows.map((r) => r.name)).toEqual(expect.arrayContaining(["handler_name", "handler_version", "payload"]));
    expect(c.requestRows.find((r) => r.name === "handler_name")!.required).toBe(true);
    expect(c.responses.map((r) => r.status)).toEqual(expect.arrayContaining(["201", "200", "401", "409", "422"]));
    expect(c.requestExamples.length).toBeGreaterThan(0);
    expect(api.errorCodes).toEqual(expect.arrayContaining(["validation_failed", "idempotency_conflict", "not_found_route"]));
    expect(api.errorResponses.every((e) => e.status)).toBe(true);
  });
});

describe("search", () => {
  const idx = buildSearchIndex();
  it("indexes every docs page and every API operation, with no external service", () => {
    for (const s of DOC_SLUGS) expect(idx.some((e) => e.u === `/docs/${s}`), s).toBe(true);
    expect(idx.some((e) => e.u === "/docs/api#createJob")).toBe(true);
    expect(JSON.stringify(idx).length).toBeLessThan(250_000);
  });
  it("ranks heading matches first and requires every word", () => {
    expect(search(idx, "fencing")[0]!.u).toContain("/docs/concepts#fencing");
    expect(search(idx, "idempotency key").length).toBeGreaterThan(0);
    expect(search(idx, "lease zzzqqq")).toEqual([]);
    expect(search(idx, "   ")).toEqual([]);
    expect(search(idx, "createJob")[0]!.u).toBe("/docs/api#createJob");
  });
});
