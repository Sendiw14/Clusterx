import { expect, test } from "@playwright/test";
import { openConsole, signIn } from "./console";
import { COOKIE_NAME, cookieOf, raw } from "./raw";

/** RV6-06: sign-in and /auth/end revoke the previous Go session, from REVIEW-M6-RV-01. */
test.describe("RV6-06: sign-in and /auth/end revoke the previous Go session", () => {
  test("a second sign-in in the same browser revokes the first session", async ({ page }) => {
    await openConsole(page, "operator");
    const first = await cookieOf(page);
    await signIn(page, "operator");
    const second = await cookieOf(page);
    expect(second).not.toBe(first);
    expect((await raw("GET", "/bff/session", { Cookie: first })).status).toBe(401);
    expect((await raw("GET", "/bff/session", { Cookie: second })).status).toBe(200);
  });

  test("/auth/end?reason=signed_out revokes the session it clears", async ({ page }) => {
    await openConsole(page, "operator");
    const c = await cookieOf(page);
    // Settle first, and retry once on net::ERR_ABORTED: under load about 1 run in 20 had Chromium abort this navigation (cause not found;
    // the assertions below are on the end state, which a retried navigation reaches the same way).
    await page.waitForLoadState("networkidle");
    await page.goto("/auth/end?reason=signed_out").catch(async (e: unknown) => { if (!/ERR_ABORTED/.test(String(e))) throw e; await page.goto("/auth/end?reason=signed_out"); });
    expect((await page.context().cookies()).some((x) => x.name === COOKIE_NAME)).toBe(false);
    expect((await raw("GET", "/bff/session", { Cookie: c })).status).toBe(401);
  });
});

