import { describe, expect, it } from "vitest";
import { STATIC_POLICY_SOURCE, buildCsp, isNoncePath } from "@/lib/csp";

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
