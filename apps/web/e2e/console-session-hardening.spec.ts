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
    await page.goto("/auth/end?reason=signed_out");
    expect((await page.context().cookies()).some((x) => x.name === COOKIE_NAME)).toBe(false);
    expect((await raw("GET", "/bff/session", { Cookie: c })).status).toBe(401);
  });
});

