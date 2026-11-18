import { execFileSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const gen = resolve(dirname(fileURLToPath(import.meta.url)), "../../../../packages/ui/tools/gen-theme.mjs");

describe("design tokens", () => {
  it("the Tailwind @theme file is generated from tokens.css and is not stale", () => {
    expect(() => execFileSync(process.execPath, [gen, "--check"], { stdio: "pipe" })).not.toThrow();
  });
});
