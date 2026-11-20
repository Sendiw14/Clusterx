import { describe, expect, it } from "vitest";
import { STATIC_POLICY_SOURCE, buildCsp, isNoncePath } from "@/lib/csp";
import { config } from "@/proxy";

describe("policy selection (RV6B-03)", () => {
  // `next.config.ts` matches `headers()` sources case-insensitively; mirror that here.
  const staticGets = (p: string) => new RegExp(`^${STATIC_POLICY_SOURCE}$`, "i").test(p);
  const paths = ["/", "/docs", "/DOCS/x", "/demo", "/application", "/authors", "/loginx", "/appendix", "/nope", "/app", "/app/jobs", "/APP", "/App/x", "/login", "/login/x", "/LOGIN", "/Login", "/auth", "/auth/end", "/AUTH/x", "/Auth/end"];

  it("every path gets exactly one of the two policies, never both, never neither", () => {
    for (const p of paths) expect(staticGets(p), p).toBe(!isNoncePath(p));
  });
  it("matches the nonce segments in any letter case and depth, and nothing that merely starts with them", () => {
    for (const p of ["/app", "/APP/jobs", "/login/x", "/Login", "/auth", "/Auth/end"]) expect(isNoncePath(p), p).toBe(true);
    for (const p of ["/", "/application", "/authors", "/loginx", "/docs/app"]) expect(isNoncePath(p), p).toBe(false);
  });
  it("the nonce policy has no unsafe-inline for scripts, the static one keeps it", () => {
    expect(buildCsp({ nonce: "abc" })).toMatch(/script-src 'self' 'nonce-abc' 'strict-dynamic'(;|$)/);
    expect(buildCsp({})).toContain("script-src 'self' 'unsafe-inline'");
  });
});

describe("proxy matcher", () => {
  const runs = (p: string) => new RegExp(`^${config.matcher[0]}$`).test(p);
  it("sees every page request, including letter-case variants and deep paths", () => {
    for (const p of ["/", "/docs", "/APP", "/Login/x", "/auth/end", "/app/jobs/abc", "/nope"]) expect(runs(p), p).toBe(true);
  });
  it("never sees BFF routes (a proxy buffers request bodies, which would undo the streaming body cap) or static assets", () => {
    for (const p of ["/bff/api/v1/jobs", "/bff/session", "/bff/stream", "/_next/static/chunk.js", "/fonts/a.woff2", "/images/x.png", "/icon.svg"]) expect(runs(p), p).toBe(false);
  });
});
