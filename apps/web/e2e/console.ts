import { expect, type APIRequestContext, type BrowserContext, type Page } from "@playwright/test";
import { USERS } from "./stack/stack";
import { MOCK_OAUTH_PORT } from "./stack/mock-oauth";

export type Who = keyof typeof USERS;

/** Tells the mock OAuth server which GitHub identity this browser context signs in as (cookie on the mock origin only). */
export async function chooseIdentity(context: BrowserContext, who: Who | "deny" | "provider_500" | "nobody") {
  const value = who === "deny" || who === "provider_500" ? who : who === "nobody" ? "0:nobody" : `${USERS[who].id}:${USERS[who].login}`;
  await context.addCookies([{ name: "mock_identity", value: encodeURIComponent(value), url: `http://localhost:${MOCK_OAUTH_PORT}` }]);
}

/** The real sign-in flow through the browser: BFF, mock GitHub, callback, session cookie. There is no shortcut login. */
export async function signIn(page: Page, who: Who, next = "/app") {
  await chooseIdentity(page.context(), who);
  await page.goto(`/auth/github?next=${encodeURIComponent(next)}`);
  await page.waitForURL((u) => u.pathname.startsWith("/app") || u.pathname === "/login", { timeout: 20_000 });
}

/** Signs in and waits until the console shell shows the identity. */
export async function openConsole(page: Page, who: Exclude<Who, "stranger" | "disabled">, path = "/app") {
  await signIn(page, who, path);
  await expect(page.locator("aside .side-foot")).toContainText(`@${USERS[who].login}`);
  return page;
}

export async function csrf(page: Page): Promise<string> {
  const r = await page.request.get("/bff/session");
  expect(r.status()).toBe(200);
  return (await r.json()).csrf_token;
}

/** An authenticated JSON call through the BFF exactly as the console makes it (Origin and CSRF header included). */
export async function bff(page: Page, method: "GET" | "POST", path: string, body?: unknown, extra: Record<string, string> = {}) {
  const headers: Record<string, string> = { ...extra };
  if (method !== "GET") {
    headers["X-CSRF-Token"] = await csrf(page);
    headers["Origin"] = "http://localhost:3100";
    headers["Content-Type"] = "application/json";
  }
  return page.request.fetch(`/bff/api/v1${path}`, { method, headers, data: body === undefined ? undefined : JSON.stringify(body) });
}

export async function waitJobStatus(page: Page, id: string, status: string, ms = 45_000) {
  await expect.poll(async () => (await (await bff(page, "GET", `/jobs/${id}`)).json()).status, { timeout: ms, intervals: [500] }).toBe(status);
}

export const live = (page: Page) => page.locator(".live[role=status]");
export type { APIRequestContext };
