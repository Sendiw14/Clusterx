import { expect, test } from "@playwright/test";
import { bff, chooseIdentity, openConsole, signIn } from "./console";
import { sql, cli, USERS } from "./stack/stack";

const SESSION = /clusterx_session$/;

test.describe("sign-in through the real OAuth flow (mock GitHub)", () => {
  test("operator lands on the console with the identity and the operator role", async ({ page }) => {
    await openConsole(page, "operator");
    await expect(page.locator("aside .side-foot")).toContainText("operator");
    await expect(page.getByText("Read-only access (viewer)")).toHaveCount(0);
    await expect(page.getByRole("link", { name: "Tokens and credentials" }).first()).toBeVisible();
  });

  test("viewer lands read-only and the operator-only pages are hidden and refused", async ({ page }) => {
    await openConsole(page, "viewer");
    await expect(page.getByText("Read-only access (viewer)")).toBeVisible();
    await expect(page.getByRole("link", { name: "Tokens and credentials" })).toHaveCount(0);
    await page.goto("/app/settings/tokens");
    await expect(page.getByTestId("forbidden-state")).toBeVisible();
    await page.goto("/app/jobs/new");
    await expect(page.getByTestId("forbidden-state")).toBeVisible();
  });

  test("a GitHub account that is not allowlisted gets the no-entitlement panel and no session", async ({ page, context }) => {
    await signIn(page, "stranger");
    await expect(page).toHaveURL(/\/login\?error=no_entitlement$/);
    await expect(page.getByText("This GitHub account isn't approved.")).toBeVisible();
    await expect(page.getByRole("link", { name: "Sign in with GitHub" })).toHaveCount(0);
    expect((await context.cookies()).filter((c) => SESSION.test(c.name))).toHaveLength(0);
    expect((await page.request.get("/bff/session")).status()).toBe(401);
  });

  test("a disabled account gets its own outcome, not a retry loop", async ({ page }) => {
    await signIn(page, "disabled");
    await expect(page).toHaveURL(/\/login\?error=user_disabled$/);
    await expect(page.getByText("This account has been disabled.")).toBeVisible();
  });

  test("GitHub denying access, and GitHub being down, map to their closed outcomes", async ({ page }) => {
    await chooseIdentity(page.context(), "deny");
    await page.goto("/auth/github");
    await expect(page).toHaveURL(/\/login\?error=auth_error$/);
    await chooseIdentity(page.context(), "provider_500");
    await page.goto("/auth/github");
    await expect(page).toHaveURL(/\/login\?error=provider_unavailable$/);
    await expect(page.getByText("GitHub is not responding.")).toBeVisible();
  });

  test("state is verified: a forged callback creates no session", async ({ page, context }) => {
    await page.goto("/auth/github"); // sets the sealed state cookie, then GitHub redirects back with a real code
    await chooseIdentity(context, "operator");
    const r = await page.request.get("/auth/callback?code=forged&state=wrong", { maxRedirects: 0 });
    expect(r.status()).toBe(303);
    expect(r.headers().location).toContain("/login?error=auth_error");
    expect((await page.request.get("/bff/session")).status()).toBe(401);
  });

  test("next is clamped to same-origin /app paths", async ({ page }) => {
    await signIn(page, "operator", "https://evil.example/app");
    await expect(page).toHaveURL(/localhost:3100\/app$/);
    const ok = await page.context().newPage();
    await signIn(ok, "operator", "/app/jobs?status=failed");
    await expect(ok).toHaveURL(/\/app\/jobs\?status=failed$/);
  });

  test("unauthenticated console visits go to /login and keep next", async ({ page }) => {
    await page.goto("/app/workers");
    await expect(page).toHaveURL(/\/login\?next=%2Fapp%2Fworkers$/);
    await expect(page.getByRole("link", { name: "Sign in with GitHub" })).toHaveAttribute("href", "/auth/github?next=%2Fapp%2Fworkers");
  });
});

test.describe("session cookie and sign-out", () => {
  test("the cookie is HttpOnly, SameSite=Lax, sealed, and unreadable from script", async ({ page, context }) => {
    await openConsole(page, "operator");
    const c = (await context.cookies()).find((x) => SESSION.test(x.name))!;
    expect(c.httpOnly).toBe(true);
    expect(c.sameSite).toBe("Lax");
    expect(c.path).toBe("/");
    expect(c.value).not.toMatch(/cxs_/);
    expect(await page.evaluate(() => document.cookie)).not.toMatch(/clusterx_session/);
  });

  test("sign out revokes the session in Go: the old cookie stops working", async ({ page, context }) => {
    await openConsole(page, "viewer");
    const old = (await context.cookies()).find((x) => SESSION.test(x.name))!;
    await page.getByRole("button", { name: "Sign out" }).first().click();
    await expect(page).toHaveURL(/\/login\?reason=signed_out$/);
    await expect(page.getByText("You've been signed out.")).toBeVisible();
    expect((await page.request.get("/bff/session")).status()).toBe(401);
    await context.addCookies([old]); // replay the stolen cookie
    const r = await page.request.get("/bff/session");
    expect(r.status()).toBe(401);
    expect((await r.json()).code).toBe("session_expired");
    await page.goto("/app");
    await expect(page).toHaveURL(/\/login/);
  });

  test("an expired session returns to /login with the reason and resumes at the page after sign-in", async ({ page }) => {
    await openConsole(page, "expiring", "/app/jobs");
    sql(`UPDATE sessions SET expires_at = now() + interval '1 second', created_at = now() - interval '1 hour' WHERE user_id = (SELECT id FROM users WHERE github_user_id = ${USERS.expiring.id}) AND revoked_at IS NULL`);
    // Leave the console first: an open console polls and holds a stream, and notices the expiry itself (clearing its cookie), which made this wait racy under load.
    await page.goto("about:blank");
    await page.waitForTimeout(6000);
    await page.goto("/app/jobs");
    await expect(page).toHaveURL(/\/login\?reason=session_expired&next=%2Fapp%2Fjobs$/);
    await expect(page.getByText("Your session ended.")).toBeVisible();
    await page.getByRole("link", { name: "Sign in with GitHub" }).click();
    await expect(page).toHaveURL(/\/app\/jobs$/);
  });

  test("disabling the account ends the session: login shows user_disabled, no retry", async ({ page }) => {
    await openConsole(page, "victim", "/app/jobs");
    cli("user", "disable", "--github-id", String(USERS.victim.id));
    try {
      // The open stream is re-validated by Go (<= 30 s) and closes 4401; the console then leaves on its own.
      await expect(page).toHaveURL(/\/login\?error=user_disabled/, { timeout: 45_000 });
      await expect(page.getByText("This account has been disabled.")).toBeVisible();
      expect((await bff(page, "GET", "/jobs")).status()).toBe(401);
    } finally {
      cli("user", "enable", "--github-id", String(USERS.victim.id));
    }
  });
});
