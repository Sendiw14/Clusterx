import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

/**
 * Demo isolation boundary (FR-DEMO-01, AC-14, E2E-DEMO-27).
 * Anything that renders under /demo, plus the fixture provider and shared feature views, must never reach the
 * live provider, the BFF client, the session module or the generated SDK runtime. Type-only imports of the SDK are
 * erased at build time and stay allowed. A Vitest test (src/data/boundary.test.ts) re-checks the real import graph.
 */
const liveOnly = [
  { group: ["@/data/live", "@/data/live-*", "@/data/live/*"], message: "Live provider is console-only. /demo uses the fixture provider." },
  { group: ["@/lib/session", "@/lib/bff", "@/lib/bff-*", "@/app/bff/*", "@/app/(console)/*"], message: "BFF and session modules are not reachable from demo or shared views." },
  { group: ["@clusterx/sdk-ts", "@clusterx/sdk-ts/*"], allowTypeImports: true, message: "The SDK runtime (network client) must not be imported here. Import types only." },
  { group: ["openapi-fetch"], message: "No network client in demo or shared views." },
  { group: ["server-only"], message: "Server-only modules are not part of the demo graph." },
];

export default defineConfig([
  ...nextVitals,
  ...nextTs,
  globalIgnores([".next/**", "out/**", "build/**", "next-env.d.ts", "playwright-report/**", "test-results/**", "coverage/**"]),
  {
    files: ["src/app/demo/**/*.{ts,tsx}", "src/features/**/*.{ts,tsx}", "src/data/fixture*/**/*.{ts,tsx}", "src/data/fixture-*.{ts,tsx}", "src/data/context.tsx", "src/data/provider.ts"],
    rules: { "no-restricted-imports": ["error", { patterns: liveOnly }] },
  },
  {
    files: ["src/**/*.{ts,tsx}"],
    rules: {
      "@typescript-eslint/no-unused-vars": ["warn", { argsIgnorePattern: "^_", varsIgnorePattern: "^_", ignoreRestSiblings: true }],
      "@typescript-eslint/consistent-type-imports": ["error", { fixStyle: "inline-type-imports" }],
      "no-restricted-globals": ["error", { name: "localStorage", message: "Wrap storage access in lib/safe-storage (may throw or be unavailable)." }],
    },
  },
  { files: ["src/lib/safe-storage.ts", "**/*.test.{ts,tsx}"], rules: { "no-restricted-globals": "off" } },
]);
