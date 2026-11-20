import { expect, test } from "@playwright/test";
import { openConsole, csrf } from "./console";
import { ORIGIN, cookieOf, raw } from "./raw";
import { cli, sql, USERS } from "./stack/stack";

/** RV6B-02 (REVIEW-M6-RV-01 round 2, ADR-0006 section 2): the sealed role hint is refreshed from GET /me. */

test.describe("RV6B-02: the sealed role hint follows the database role", () => {
  test.describe.configure({ mode: "serial" }); // both tests change the role of the one `demoted` user
  const uid = () => sql(`SELECT id FROM users WHERE github_user_id = ${USERS.demoted.id}`).trim();
  const rows = (id: string) => Number(sql(`SELECT count(*) FROM audit_events WHERE action = 'assertion.role_mismatch' AND actor = 'user:${id}'`).trim());
  const setRole = (role: "operator" | "viewer") => cli("user", "upsert", "--github-id", String(USERS.demoted.id), "--login", USERS.demoted.login, "--role", role);

  test("a demotion mid-session: the 403 refreshes the cookie, later requests carry the viewer hint and write no more mismatch rows", async ({ page }) => {
    await openConsole(page, "demoted");
    const token = await csrf(page);
    await page.goto("about:blank"); // the open console would generate traffic of its own
    const id = uid();
    const before = rows(id);
    const cookie0 = await cookieOf(page);
    setRole("viewer");
    try {
      const post = await page.request.fetch("/bff/api/v1/jobs", { method: "POST", headers: { "X-CSRF-Token": token, Origin: ORIGIN, "Content-Type": "application/json" }, data: "{}" });
      expect(post.status()).toBe(403);
      expect(await cookieOf(page), "cookie re-sealed with the database role").not.toBe(cookie0);
      const afterPost = rows(id) - before;
      expect(afterPost, "mismatch rows written by the demotion itself").toBeLessThanOrEqual(1);
      for (let i = 0; i < 5; i++) expect((await page.request.get("/bff/api/v1/jobs?limit=1")).status()).toBe(200);
      const s = await page.request.get("/bff/session");
      expect((await s.json()).role).toBe("viewer");
      expect(rows(id) - before, "no further rows after the refresh").toBe(afterPost);
    } finally {
      setRole("operator");
    }
  });

  test("a promotion mid-session: /bff/session refreshes the cookie once, then the rows stop", async ({ page }) => {
    setRole("viewer");
    await openConsole(page, "demoted");
    await page.goto("about:blank");
    const id = uid();
    const before = rows(id);
    const cookie0 = await cookieOf(page);
    setRole("operator");
    const s = await page.request.get("/bff/session");
    expect((await s.json()).role).toBe("operator");
    expect(await cookieOf(page)).not.toBe(cookie0);
    const afterSession = rows(id) - before;
    expect(afterSession).toBeLessThanOrEqual(1);
    for (let i = 0; i < 5; i++) await page.request.get("/bff/api/v1/jobs?limit=1");
    await page.request.get("/bff/session");
    expect(rows(id) - before).toBe(afterSession);
  });

  test("an unchanged role sets no cookie", async ({ page }) => {
    await openConsole(page, "viewer");
    const cookie = await cookieOf(page);
    const r = await raw("GET", "/bff/session", { Cookie: cookie });
    expect(r.status).toBe(200);
    expect(r.headers["set-cookie"]).toBeUndefined();
  });
});
