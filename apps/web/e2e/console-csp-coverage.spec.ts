import { test, expect } from "@playwright/test";
import { raw } from "./raw";

/** RV6B-03 (REVIEW-M6-RV-01 round 2): the right CSP on every path, in any letter case. */

const scriptSrc = (csp: string) => csp.split(";").map((d) => d.trim()).find((d) => d.startsWith("script-src")) ?? "";
const cspHeaders = (rawHeaders: string[]) => rawHeaders.flatMap((h, i) => (i % 2 === 0 && h.toLowerCase() === "content-security-policy" ? [rawHeaders[i + 1]!] : []));

test.describe("RV6B-03: every response under /app, /login and /auth carries exactly one nonce CSP, in any letter case", () => {
  const nonce = ["/app", "/app/x", "/app/jobs", "/APP", "/APP/jobs", "/App/x", "/login", "/login/x", "/LOGIN", "/Login", "/auth", "/auth/x", "/AUTH/x", "/Auth/end", "/auth/end"];
  const staticPolicy = ["/", "/docs", "/DOCS", "/nope", "/demo", "/application", "/authors", "/loginx", "/architecture"];
  for (const p of nonce) {
    test(`${p}: one CSP, nonce and strict-dynamic, no 'unsafe-inline'`, async () => {
      const r = await raw("GET", p, {});
      const all = cspHeaders(r.rawHeaders);
      expect(all, `${p} -> ${r.status}`).toHaveLength(1);
      expect(scriptSrc(all[0]!)).toMatch(/'nonce-[A-Za-z0-9+/=_-]{16,}'.*'strict-dynamic'/);
      expect(scriptSrc(all[0]!)).not.toContain("'unsafe-inline'");
    });
  }
  for (const p of staticPolicy) {
    test(`${p}: one static CSP`, async () => {
      const r = await raw("GET", p, {});
      const all = cspHeaders(r.rawHeaders);
      expect(all, `${p} -> ${r.status}`).toHaveLength(1);
      expect(scriptSrc(all[0]!)).not.toContain("'nonce-");
    });
  }
});
