import { ESLint } from "eslint";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const cwd = resolve(dirname(fileURLToPath(import.meta.url)), "../..");

async function lint(code: string, filePath: string) {
  const eslint = new ESLint({ cwd });
  const [r] = await eslint.lintText(code, { filePath: resolve(cwd, filePath) });
  return r!.messages.filter((m) => m.ruleId === "no-restricted-imports");
}

describe("ESLint no-restricted-imports demo boundary", () => {
  it("rejects the live provider, session, BFF contract and SDK runtime inside /demo", async () => {
    for (const spec of ["@/data/live-provider", "@/lib/session", "@/lib/bff-contract", "@clusterx/sdk-ts", "openapi-fetch", "server-only"]) {
      const m = await lint(`import x from "${spec}";\nexport default x;\n`, "src/app/demo/x.tsx");
      expect(m.length, spec).toBeGreaterThan(0);
    }
  }, 30_000);
  it("rejects the same imports in shared feature views and the fixture provider", async () => {
    expect((await lint(`import { LiveProvider } from "@/data/live-provider";\nexport const p = LiveProvider;\n`, "src/features/x.tsx")).length).toBeGreaterThan(0);
    expect((await lint(`import { LiveProvider } from "@/data/live-provider";\nexport const p = LiveProvider;\n`, "src/data/fixture-provider.ts")).length).toBeGreaterThan(0);
  }, 30_000);
  it("allows type-only SDK imports and the fixture provider", async () => {
    expect(await lint(`import type { components } from "@clusterx/sdk-ts";\nexport type T = components;\n`, "src/app/demo/x.tsx")).toEqual([]);
    expect(await lint(`import { FixtureProvider } from "@/data/fixture-provider";\nexport const p = FixtureProvider;\n`, "src/app/demo/x.tsx")).toEqual([]);
  }, 30_000);
  it("the console may import the live provider", async () => {
    expect(await lint(`import { LiveProvider } from "@/data/live-provider";\nexport const p = LiveProvider;\n`, "src/app/(console)/x.tsx")).toEqual([]);
  }, 30_000);
});
