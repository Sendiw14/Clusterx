// Per-route first-load client JS from `next build` (Next 16 writes .next/diagnostics/route-bundle-stats.json).
// Usage: node scripts/bundle-size.mjs [outfile.json]. Run `pnpm build` first. Sizes: raw, gzip -9, brotli q11.
import { readFileSync, writeFileSync } from "node:fs";
import { gzipSync, brotliCompressSync, constants } from "node:zlib";

const stats = JSON.parse(readFileSync(".next/diagnostics/route-bundle-stats.json", "utf8"));
const manifest = JSON.parse(readFileSync(".next/build-manifest.json", "utf8"));
const group = (r) => (r.startsWith("/app") ? "console" : r === "/demo" ? "demo" : r.startsWith("/docs") ? "docs" : r === "/login" ? "auth" : r === "/_not-found" || r === "/[...unmatched]" ? "not-found" : "public");
const size = (f) => { const b = readFileSync(f); return { raw: b.length, gzip: gzipSync(b, { level: 9 }).length, brotli: brotliCompressSync(b, { params: { [constants.BROTLI_PARAM_QUALITY]: 11 } }).length }; };
const cache = new Map();
const of = (f) => { if (!cache.has(f)) cache.set(f, size(f)); return cache.get(f); };
const sum = (fs) => fs.reduce((a, f) => { const s = of(f); return { raw: a.raw + s.raw, gzip: a.gzip + s.gzip, brotli: a.brotli + s.brotli }; }, { raw: 0, gzip: 0, brotli: 0 });

const routes = stats.map((s) => {
  const t = sum(s.firstLoadChunkPaths);
  if (t.raw !== s.firstLoadUncompressedJsBytes) throw new Error(`${s.route}: raw ${t.raw} != Next ${s.firstLoadUncompressedJsBytes}`);
  return { route: s.route, group: group(s.route), chunks: s.firstLoadChunkPaths.length, ...t };
}).sort((a, b) => a.group.localeCompare(b.group) || a.route.localeCompare(b.route));

const polyfills = manifest.polyfillFiles.map((f) => `.next/${f}`);
const out = { next_build_id: readFileSync(".next/BUILD_ID", "utf8").trim(), polyfills: { files: polyfills.length, ...sum(polyfills), note: "loaded only by browsers that need them (nomodule); not in firstLoad" }, routes };
writeFileSync(process.argv[2] ?? "/dev/stdout", JSON.stringify(out, null, 2) + "\n");
