#!/usr/bin/env node
// Generates tokens/theme.generated.css (a Tailwind 4 `@theme static` block) from tokens/tokens.css.
// tokens.css stays the single source of truth; nothing is copied by hand. `--check` exits 1 if stale.
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const dir = join(dirname(fileURLToPath(import.meta.url)), "../tokens");
const src = readFileSync(join(dir, "tokens.css"), "utf8");
const out = join(dir, "theme.generated.css");
const root = src.match(/:root\s*\{([\s\S]*?)\n\}/);
if (!root) throw new Error("tokens.css: :root block not found");
// Only namespaces Tailwind understands become theme variables; everything else (chart-*, space-*, icon-*, row-*,
// duration-*, grid-*, max-width-*) stays available as plain CSS variables from tokens.css.
const keep = /^--(color|font|text|leading|tracking|radius|shadow|ease|breakpoint|spacing)(-|$)/;
const decls = root[1].split("\n").map((l) => l.trim()).filter((l) => l.startsWith("--") && keep.test(l));
const body = `/* GENERATED from tokens.css by tools/gen-theme.mjs. Do not edit. */\n@theme static {\n${decls.map((d) => "  " + d).join("\n")}\n}\n`;
if (process.argv.includes("--check")) {
  if (!existsSync(out) || readFileSync(out, "utf8") !== body) { console.error("STALE theme.generated.css"); process.exit(1); }
} else writeFileSync(out, body);
