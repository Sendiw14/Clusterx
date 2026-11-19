import { readFileSync, readdirSync, statSync, existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const here = dirname(fileURLToPath(import.meta.url));
const SRC = resolve(here, "..");
const UI_SRC = resolve(here, "../../../../packages/ui/src");

/** Specifiers that must never be reachable from /demo, the fixture provider or shared views (FR-DEMO-01, E2E-DEMO-27). */
export const FORBIDDEN: RegExp[] = [
  /live-provider/, /console-runtime/, /^@\/server\//, /lib\/session/, /lib\/bff/, /app\/bff/, /\(console\)/, /^openapi-fetch$/, /^server-only$/,
];
const SDK = /^@clusterx\/sdk-ts(\/|$)/;

export function importsOf(source: string): { spec: string; typeOnly: boolean }[] {
  const out: { spec: string; typeOnly: boolean }[] = [];
  const re = /(?:^|\n)\s*(?:import|export)\s+(type\s+)?(?:[^"';]*?\sfrom\s+)?["']([^"']+)["']|import\(\s*["']([^"']+)["']\s*\)/g;
  for (const m of source.matchAll(re)) out.push({ spec: (m[2] ?? m[3])!, typeOnly: Boolean(m[1]) });
  return out;
}

function resolveSpec(from: string, spec: string): string | null {
  const base = spec.startsWith("@/") ? join(SRC, spec.slice(2)) : spec.startsWith(".") ? resolve(dirname(from), spec) : null;
  if (!base) return null;
  for (const c of [base, `${base}.ts`, `${base}.tsx`, join(base, "index.ts"), join(base, "index.tsx")]) if (existsSync(c) && statSync(c).isFile()) return c;
  return null;
}

function walk(entries: string[]): { violations: string[]; files: Set<string> } {
  const seen = new Set<string>();
  const violations: string[] = [];
  const stack = [...entries];
  while (stack.length) {
    const f = stack.pop()!;
    if (seen.has(f)) continue;
    seen.add(f);
    if (!/\.(ts|tsx)$/.test(f)) continue;
    for (const { spec, typeOnly } of importsOf(readFileSync(f, "utf8"))) {
      if (SDK.test(spec) && !typeOnly) violations.push(`${f}: runtime import of ${spec}`);
      if (FORBIDDEN.some((r) => r.test(spec))) violations.push(`${f}: forbidden import ${spec}`);
      const next = resolveSpec(f, spec);
      if (next) {
        if (FORBIDDEN.some((r) => r.test(next.replace(SRC, "")))) violations.push(`${f}: reaches ${next.replace(SRC, "")}`);
        stack.push(next);
      }
    }
  }
  return { violations, files: seen };
}

function listFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => (e.isDirectory() ? listFiles(join(dir, e.name)) : /\.(ts|tsx)$/.test(e.name) ? [join(dir, e.name)] : []));
}

describe("demo isolation: import graph (E2E-DEMO-27)", () => {
  it("everything reachable from src/app/demo and the fixture provider avoids the live provider, BFF, session, SDK runtime and server-only", () => {
    const entries = [...listFiles(join(SRC, "app/demo")), ...listFiles(join(SRC, "features")), join(SRC, "data/fixture-provider.ts"), join(SRC, "data/context.tsx"), join(SRC, "data/provider.ts")];
    const { violations, files } = walk(entries);
    expect(files.size).toBeGreaterThan(15);
    expect(violations).toEqual([]);
  });
  it("the shared UI package has no network client, data layer or session import", () => {
    const { violations, files } = walk(listFiles(UI_SRC));
    expect(files.size).toBeGreaterThan(15);
    expect(violations).toEqual([]);
  });
  it("the checker has teeth: it flags a live-provider import, a runtime SDK import and server-only", () => {
    const bad = importsOf(`import { LiveProvider } from "@/data/live-provider";\nimport { createClusterxClient } from "@clusterx/sdk-ts";\nimport "server-only";\nimport type { components } from "@clusterx/sdk-ts";`);
    expect(bad.map((b) => b.spec)).toEqual(["@/data/live-provider", "@clusterx/sdk-ts", "server-only", "@clusterx/sdk-ts"]);
    expect(bad.filter((b) => FORBIDDEN.some((r) => r.test(b.spec))).map((b) => b.spec)).toEqual(["@/data/live-provider", "server-only"]);
    expect(bad.filter((b) => SDK.test(b.spec) && !b.typeOnly)).toHaveLength(1);
  });
  it("the live provider is imported only by the console runtime, which only the console providers module imports", () => {
    const using = (re: RegExp) => listFiles(SRC).filter((f) => !f.includes("__tests__") && importsOf(readFileSync(f, "utf8")).some((i) => re.test(i.spec))).map((u) => u.replace(SRC, ""));
    expect(using(/live-provider/)).toEqual(["/data/console-runtime.ts"]);
    expect(using(/console-runtime/)).toEqual(["/app/(console)/console-providers.tsx"]);
  });
});
