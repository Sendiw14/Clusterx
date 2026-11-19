import { request as httpRequest, type IncomingMessage } from "node:http";
import type { Page } from "@playwright/test";
import { csrf } from "./console";
import { PORTS } from "./stack/stack";

/**
 * Raw HTTP helpers for the REVIEW-M6-RV-01 regression specs. Browsers and Playwright's client normalise `..` and re-encode
 * paths before sending, which is exactly why the earlier traversal spec could not see RV6-01, so these use `node:http`.
 */
export const HOST = "localhost";
export const PORT = PORTS.web;
export const ORIGIN = `http://localhost:${PORT}`;
export const COOKIE_NAME = "clusterx_session";

export async function cookieOf(page: Page): Promise<string> {
  const c = (await page.context().cookies()).find((x) => x.name === COOKIE_NAME);
  if (!c) throw new Error("no session cookie");
  return `${COOKIE_NAME}=${c.value}`;
}

export interface Raw { status: number; body: string; headers: IncomingMessage["headers"]; rawHeaders: string[] }

export function raw(method: string, path: string, headers: Record<string, string>, body?: Buffer): Promise<Raw> {
  return new Promise((res, rej) => {
    const r = httpRequest({ host: HOST, port: PORT, method, path, headers }, (m) => {
      const chunks: Buffer[] = [];
      m.on("data", (d: Buffer) => chunks.push(d));
      m.on("end", () => res({ status: m.statusCode ?? 0, body: Buffer.concat(chunks).toString("utf8").slice(0, 400), headers: m.headers, rawHeaders: m.rawHeaders }));
    });
    r.on("error", rej);
    if (body) r.write(body);
    r.end();
  });
}

export const mutating = async (page: Page) => ({ Cookie: await cookieOf(page), Origin: ORIGIN, "X-CSRF-Token": await csrf(page) });
