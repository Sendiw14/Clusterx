import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { PUBLIC_ROUTES } from "./helpers";

// AUDIT-M9 F-05 guard: public pages must stay prerendered (a stray `connection()`/`headers()` in the root layout or
// not-found makes every route dynamic). Needs `next build` output, which the script order guarantees before e2e.
const manifest = JSON.parse(readFileSync(".next/prerender-manifest.json", "utf8")) as { routes: Record<string, unknown> };

test.describe("public routes are prerendered", () => {
  test("build output lists every public route as static", () => {
    for (const r of [...PUBLIC_ROUTES.filter((x) => x !== "/login"), "/_not-found"]) expect(Object.keys(manifest.routes), r).toContain(r);
    for (const r of ["/app", "/login", "/app/jobs"]) expect(Object.keys(manifest.routes), r).not.toContain(r);
  });

  test("served as prerendered output with the static CSP (not a per-request nonce)", async ({ request }) => {
    for (const r of PUBLIC_ROUTES.filter((x) => x !== "/login")) {
      const res = await request.get(r);
      expect(res.status(), r).toBe(200);
      expect(res.headers()["x-nextjs-cache"], r).toBe("HIT");
      expect(res.headers()["content-security-policy"], r).toContain("'unsafe-inline'");
    }
  });
});
