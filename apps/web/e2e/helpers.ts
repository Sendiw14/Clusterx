import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";
import { recordAxe, type AxeRecord } from "./axe-report";

export const PUBLIC_ROUTES = ["/", "/architecture", "/docs", "/docs/quickstart", "/docs/concepts", "/docs/api", "/docs/workers", "/docs/reliability", "/docs/deployment", "/docs/security", "/changelog", "/login", "/demo"];
export const CONSOLE_ROUTES = ["/app", "/app/jobs", "/app/jobs/new", "/app/workers", "/app/observability", "/app/activity", "/app/settings", "/app/settings/tokens", "/app/settings/preferences"];
export const ALL_ROUTES = [...PUBLIC_ROUTES, ...CONSOLE_ROUTES];
export const GATED_ROUTES = ["/app/workflows", "/app/workflows/x", "/app/workflow-runs/x"];


/**
 * Collects console errors and page errors. The BFF skeleton answers 501 on purpose (M5 implements it), and Chromium
 * logs every non-2xx subresource as a console error. Only that exact case, for /bff/ URLs, is allowed.
 */
export function watchConsole(page: Page): string[] {
  const errors: string[] = [];
  page.on("console", (m) => {
    if (m.type() !== "error") return;
    const url = m.location().url;
    if (url.includes("/bff/") && /status of 501/.test(m.text())) return;
    errors.push(`${m.text()} @ ${url}`);
  });
  // WebKit reports a Next link prefetch that the browser cancelled (the page is being replaced by a full navigation) as an unhandled
  // rejection worded "due to access control checks". The request status is `cancelled`, not a failure of the app.
  page.on("pageerror", (e) => { if (!/_rsc=\w+ due to access control checks\.?$/.test(e.message)) errors.push(`pageerror: ${e.message}`); });
  return errors;
}

export async function gotoReady(page: Page, path: string) {
  const res = await page.goto(path, { waitUntil: "networkidle" });
  return res;
}


export async function runAxe(page: Page, route: string, width: number): Promise<AxeRecord> {
  const r = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  const count = (i: string) => r.violations.filter((v) => v.impact === i).length;
  const rec: AxeRecord = {
    route, width,
    critical: count("critical"), serious: count("serious"), moderate: count("moderate"), minor: count("minor"),
    violations: r.violations.map((v) => ({ id: v.id, impact: v.impact, nodes: v.nodes.length, help: v.help })),
  };
  recordAxe({ ...rec, project: test.info().project.name });
  return rec;
}

export async function noHorizontalOverflow(page: Page) {
  const o = await page.evaluate(() => {
    const cw = document.documentElement.clientWidth;
    const wide = [...document.body.querySelectorAll("*")].filter((e) => e.getBoundingClientRect().right > cw + 0.5).slice(0, 4).map((e) => `${e.tagName.toLowerCase()}${e.id ? `#${e.id}` : ""}.${String((e as HTMLElement).className).slice(0, 40)}`);
    return { sw: document.documentElement.scrollWidth, cw, wide };
  });
  expect(o.sw, `horizontal overflow on ${new URL(page.url()).pathname}: scrollWidth ${o.sw} > clientWidth ${o.cw}; first elements past the edge: ${o.wide.join(" | ")}`).toBeLessThanOrEqual(o.cw);
}

/** Playwright grants clipboard permissions only in Chromium; Firefox and WebKit reject them, so those get a plain context. */
export const clipboardContext = (browser: Browser) =>
  browser.newContext({ viewport: { width: 1440, height: 900 }, ...(test.info().project.name === "chromium" ? { permissions: ["clipboard-read", "clipboard-write"] } : {}) });

/** What the page just copied. Chromium reads it back through the API; elsewhere the text is pasted into a scratch textarea with the keyboard (a real paste). */
export async function readClipboard(page: Page): Promise<string> {
  if (test.info().project.name === "chromium") return page.evaluate(() => navigator.clipboard.readText());
  await page.evaluate(() => { const t = document.createElement("textarea"); t.id = "__paste"; document.body.append(t); t.focus(); });
  await page.keyboard.press("ControlOrMeta+V");
  return page.locator("#__paste").inputValue();
}

/**
 * Opens a menu or dialog the way a keyboard user does. Safari (WebKit) does not focus a button on a mouse click, so a dialog
 * opened by mouse has no trigger to return focus to; the focus-return requirement is about keyboard (and screen reader) users.
 */
export async function activate(page: Page, trigger: Locator) {
  await trigger.focus();
  await page.keyboard.press("Enter");
}

/** Waits for a dialog's fade-in (M-OVERLAY) to finish, so axe measures contrast on the final opacity, not mid-transition. */
export async function animationsDone(locator: Locator) {
  await locator.evaluate((el) => Promise.all(el.getAnimations({ subtree: true }).map((a) => a.finished.catch(() => undefined))));
}
