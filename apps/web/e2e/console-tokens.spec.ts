import { expect, test, type Page } from "@playwright/test";
import { openConsole } from "./console";
import { API_URL } from "./stack/stack";

async function createToken(page: Page, name: string) {
  await page.getByRole("button", { name: "Create API token" }).click();
  const dlg = page.getByRole("dialog", { name: "Create API token" });
  await dlg.getByLabel(/^Name/).fill(name);
  await dlg.getByRole("button", { name: "Create", exact: true }).click();
  const reveal = page.getByRole("dialog", { name: /Copy your new API token now/ });
  await expect(reveal).toBeVisible();
  return reveal;
}

test.describe("API tokens and worker credentials (operator)", () => {
  test("create, reveal exactly once, use, list without the secret, revoke", async ({ page }) => {
    await openConsole(page, "operator", "/app/settings/tokens");
    const name = `e2e-token-${Date.now()}`;
    const reveal = await createToken(page, name);
    const secret = (await reveal.getByTestId("secret-value").innerText()).trim();
    expect(secret).toMatch(/^cxt_/);
    // The reveal blocks: Esc does not close it and Done stays disabled until the user confirms.
    await page.keyboard.press("Escape");
    await expect(reveal).toBeVisible();
    await expect(reveal.getByRole("button", { name: /Done, I have stored it/ })).toHaveAttribute("aria-disabled", "true");
    await reveal.getByLabel("I have copied it somewhere safe").check();
    await reveal.getByRole("button", { name: /Done, I have stored it/ }).click();
    await expect(reveal).toBeHidden();

    // The secret is gone from the DOM and never comes back, not even after a reload.
    expect(await page.content()).not.toContain(secret);
    await page.reload();
    expect(await page.content()).not.toContain(secret);
    const row = page.locator("table.tbl tbody tr", { hasText: name });
    await expect(row).toBeVisible();
    await expect(row).toContainText(secret.split("_")[1]!.slice(0, 4)); // prefix only
    const list = await page.request.get("/bff/api/v1/tokens");
    expect(JSON.stringify(await list.json())).not.toContain(secret);

    // The token really works against the API (viewer scopes only: reads).
    expect((await fetch(`${API_URL}/api/v1/jobs`, { headers: { Authorization: `Bearer ${secret}` } })).status).toBe(200);

    await row.getByRole("button", { name: "Revoke" }).click();
    const confirm = page.getByRole("dialog", { name: "Revoke this token?" });
    await expect(confirm).toContainText(name);
    await confirm.getByRole("button", { name: "Revoke", exact: true }).click();
    await expect(confirm).toBeHidden();
    await expect(page.locator("table.tbl tbody tr", { hasText: name })).toHaveCount(0); // hidden unless "Show revoked"
    await page.getByLabel("Show revoked").first().check();
    await expect(page.locator("table.tbl tbody tr", { hasText: name }).first()).toContainText("revoked");
    const after = await fetch(`${API_URL}/api/v1/jobs`, { headers: { Authorization: `Bearer ${secret}` } });
    expect(after.status).toBe(401);
    expect((await after.json()).code).toBe("unauthenticated");
  });

  test("rotation creates the replacement first, shows it once, then offers to revoke the old one", async ({ page }) => {
    await openConsole(page, "operator", "/app/settings/tokens");
    const name = `e2e-rotate-${Date.now()}`;
    const first = await createToken(page, name);
    const oldSecret = (await first.getByTestId("secret-value").innerText()).trim();
    await first.getByLabel("I have copied it somewhere safe").check();
    await first.getByRole("button", { name: /Done, I have stored it/ }).click();

    await page.locator("table.tbl tbody tr", { hasText: name }).getByRole("button", { name: "Rotate" }).click();
    await page.getByRole("dialog", { name: "Rotate this token?" }).getByRole("button", { name: "Create replacement" }).click();
    const reveal = page.getByRole("dialog", { name: /Copy your new API token now/ });
    await expect(reveal).toContainText(`${name} (rotated)`);
    const newSecret = (await reveal.getByTestId("secret-value").innerText()).trim();
    expect(newSecret).not.toBe(oldSecret);
    await reveal.getByLabel("I have copied it somewhere safe").check();
    await reveal.getByRole("button", { name: /Done, I have stored it/ }).click();
    const confirm = page.getByRole("dialog", { name: "Revoke this token?" });
    await expect(confirm).toContainText(name);
    await confirm.getByRole("button", { name: "Revoke", exact: true }).click();
    await expect.poll(async () => (await fetch(`${API_URL}/api/v1/jobs`, { headers: { Authorization: `Bearer ${oldSecret}` } })).status).toBe(401);
    expect((await fetch(`${API_URL}/api/v1/jobs`, { headers: { Authorization: `Bearer ${newSecret}` } })).status).toBe(200);
  });

  test("worker credentials: create with one-time reveal, then revoke", async ({ page }) => {
    await openConsole(page, "operator", "/app/settings/tokens");
    await page.getByRole("tab", { name: "Worker credentials" }).click();
    await page.getByRole("button", { name: "Create worker credential" }).click();
    const dlg = page.getByRole("dialog", { name: "Create worker credential" });
    const name = `e2e-cred-${Date.now()}`;
    await dlg.getByLabel(/^Name/).fill(name);
    await dlg.getByRole("button", { name: "Create", exact: true }).click();
    const reveal = page.getByRole("dialog", { name: /Copy your new worker credential now/ });
    const secret = (await reveal.getByTestId("secret-value").innerText()).trim();
    expect(secret).toMatch(/^cxw_/);
    await reveal.getByLabel("I have copied it somewhere safe").check();
    await reveal.getByRole("button", { name: /Done, I have stored it/ }).click();
    expect(await page.content()).not.toContain(secret);
    const row = page.getByRole("tabpanel", { name: "Worker credentials" }).locator("table.tbl tbody tr", { hasText: name });
    await row.getByRole("button", { name: "Revoke" }).click();
    await page.getByRole("dialog", { name: "Revoke this credential?" }).getByRole("button", { name: "Revoke", exact: true }).click();
    await expect(page.getByRole("tabpanel", { name: "Worker credentials" }).locator("table.tbl tbody tr", { hasText: name })).toHaveCount(0);
  });

  test("an empty name is refused before any request", async ({ page }) => {
    await openConsole(page, "operator", "/app/settings/tokens");
    await page.getByRole("button", { name: "Create API token" }).click();
    const dlg = page.getByRole("dialog", { name: "Create API token" });
    await dlg.getByRole("button", { name: "Create", exact: true }).click();
    await expect(dlg.getByText("Give it a name you will recognise.")).toBeVisible();
  });
});
