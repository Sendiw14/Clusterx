import type { NextConfig } from "next";
import { fileURLToPath } from "node:url";
import { STATIC_POLICY_SOURCE, buildCsp } from "./src/lib/csp";

const isDev = process.env.NODE_ENV !== "production";

/** Public pages are prerendered (`○` in the `next build` route table; e2e/static-public.spec.ts guards it) so they keep a static policy, which needs `'unsafe-inline'` for Next's inline bootstrap. The console, /login and /auth (any letter case, any depth) get a per-request nonce policy from `src/proxy.ts` instead (no second CSP header here). Unmatched URLs render per request via `app/[...unmatched]/page.tsx`, so their 404 can carry that nonce. */

const baseHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()" },
  { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
  { key: "Cross-Origin-Resource-Policy", value: "same-origin" },
  { key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains" },
];

const config: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  transpilePackages: ["@clusterx/ui"],
  turbopack: { root: fileURLToPath(new URL("../..", import.meta.url)) },
  async headers() {
    return [
      { source: "/:path*", headers: baseHeaders },
      { source: STATIC_POLICY_SOURCE, headers: [{ key: "Content-Security-Policy", value: buildCsp({ dev: isDev }) }] },
      {
        source: "/app/:path*",
        headers: [
          { key: "Cache-Control", value: "no-store" },
          { key: "X-Robots-Tag", value: "noindex, nofollow" },
        ],
      },
      { source: "/login", headers: [{ key: "Cache-Control", value: "no-store" }, { key: "X-Robots-Tag", value: "noindex, nofollow" }] },
      { source: "/bff/:path*", headers: [{ key: "Cache-Control", value: "no-store" }, { key: "X-Robots-Tag", value: "noindex, nofollow" }] },
      { source: "/fonts/:path*", headers: [{ key: "Cache-Control", value: "public, max-age=31536000, immutable" }] },
    ];
  },
};

export default config;
