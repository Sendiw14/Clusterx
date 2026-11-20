import { mkdirSync, writeFileSync } from "node:fs";
import os from "node:os";
import { resolve } from "node:path";
import { expect, test } from "@playwright/test";
import { openConsole } from "./console";
import { API_URL, USERS, cli, sql } from "./stack/stack";

/**
 * Console budget measurements (AUDIT-M9 F-09, NFR-PERF-02/03). Opt-in: needs a stack without the mutation rate limit.
 *   pnpm --filter web build
 *   CLUSTERX_PERF=1 CLUSTERX_E2E_RATE_MUTATIONS=0 CLUSTERX_E2E_PG_DIR=<dir> pnpm --filter web exec playwright test e2e/perf-console.spec.ts --project=chromium
 * It seeds ~3,000 jobs and 100 workers on the throwaway E2E stack, measures in the real browser against the production build,
 * and writes raw samples to docs/testing/evidence/web/console-perf-<project>.json. Lab numbers on one machine: no budget is frozen here.
 */
test.skip(!process.env.CLUSTERX_PERF, "set CLUSTERX_PERF=1 (and CLUSTERX_E2E_RATE_MUTATIONS=0) to measure console budgets");
test.describe.configure({ mode: "serial" });
test.setTimeout(600_000);

const OUT = resolve(import.meta.dirname, "../../../docs/testing/evidence/web");
const JOBS = 3000;
const WORKERS = 100;

const pct = (a: number[], p: number) => { const s = [...a].sort((x, y) => x - y); return s.length ? s[Math.min(s.length - 1, Math.ceil((p / 100) * s.length) - 1)]! : NaN; };
const stats = (a: number[]) => ({ n: a.length, min: Math.min(...a), p50: pct(a, 50), p95: pct(a, 95), p99: pct(a, 99), max: Math.max(...a), mean: a.reduce((x, y) => x + y, 0) / a.length });
const round = (n: number) => Math.round(n * 10) / 10;
const results: Record<string, unknown> = {};
const raw: Record<string, number[]> = {};
const record = (k: string, samples: number[]) => { raw[k] = samples.map(round); results[k] = Object.fromEntries(Object.entries(stats(samples)).map(([a, b]) => [a, round(b)])); };

let secret = "";
const api = (path: string, init: RequestInit = {}) => fetch(`${API_URL}${path}`, { ...init, headers: { Authorization: `Bearer ${secret}`, "Content-Type": "application/json", ...(init.headers ?? {}) } });
let seq = 0;
const submit = (handler: string, payload: unknown, extra: Record<string, unknown> = {}) =>
  api("/api/v1/jobs", { method: "POST", headers: { "Idempotency-Key": `perf-${Date.now()}-${seq++}` }, body: JSON.stringify({ handler_name: handler, handler_version: 1, payload, ...extra }) });

test.beforeAll(async () => {
  const op = USERS.operator;
  secret = cli("token", "create", "--role", "operator", "--name", "perf", "--owner-github-id", String(op.id), "--owner-login", op.login).match(/^secret=(.+)$/m)![1]!;
  // 3,000 jobs: the benchmark's handler mix plus 10% scheduled an hour out, so the list has several statuses.
  let next = 0;
  await Promise.all(Array.from({ length: 16 }, async () => {
    for (;;) {
      const i = next++;
      if (i >= JOBS) return;
      const r = i % 10 === 0 ? await submit("hash_sha256", { data: `scheduled-${i}` }, { run_after: new Date(Date.now() + 3_600_000).toISOString() })
        : i % 10 < 6 ? await submit("hash_sha256", { data: `h-${i}` })
        : i % 10 < 8 ? await submit("sleep_ms", { ms: 100 })
        : i % 10 === 8 ? await submit("fail_n_times", { n: 1 })
        : await submit("fail_n_times", { n: 1, permanent: true });
      if (r.status !== 201) throw new Error(`seed ${i}: ${r.status} ${await r.text()}`);
    }
  }));
  for (let t = 0; t < 120; t++) {
    const open = await Promise.all(["queued", "running", "retry_wait"].map(async (s) => (await (await api(`/api/v1/jobs?status=${s}&limit=1`)).json() as { items: unknown[] }).items.length));
    if (open.every((n) => n === 0)) return;
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error("seeded jobs did not drain");
});

test("jobs table filter and sort: response time (BFF, as the browser sees it) and view update time", async ({ page }) => {
  await openConsole(page, "operator", "/app/jobs");
  await expect(page.getByRole("table")).toBeVisible();
  const urls = ["status=failed", "status=succeeded", "status=scheduled", "handler_name=hash_sha256", "priority_min=5", "sort=created_at", "sort=-priority", "sort=available_at", "status=succeeded&handler_name=hash_sha256&sort=-priority", ""];
  const bff: number[] = [];
  const direct: number[] = [];
  for (let round_ = 0; round_ < 20; round_++) {
    for (const q of urls) {
      bff.push(await page.evaluate(async (qs) => { const t = performance.now(); const r = await fetch(`/bff/api/v1/jobs?limit=50&${qs}`); await r.json(); if (!r.ok) throw new Error(String(r.status)); return performance.now() - t; }, q));
      const t = performance.now(); const r = await api(`/api/v1/jobs?limit=50&${q}`); await r.json(); direct.push(performance.now() - t);
    }
  }
  record("jobs_query_bff_ms", bff);
  record("jobs_query_direct_api_ms", direct);

  // The real control: change Sort / Status in the page and time until the new table is painted.
  const view: number[] = [];
  const sorts = ["created_at", "-priority", "available_at", "-available_at", "-created_at"];
  const statuses = ["failed", "succeeded", "scheduled", ""];
  for (let i = 0; i < 40; i++) {
    const [label, value] = i % 2 === 0 ? ["Sort", sorts[(i / 2) % sorts.length]!] : ["Status", statuses[((i - 1) / 2) % statuses.length]!];
    const sel = page.getByLabel(label, { exact: true });
    await page.evaluate(() => performance.clearResourceTimings()); // the default buffer holds 250 entries
    const before = 0;
    // Real user input first (so React sees a genuine change), timing starts from the page-side handler.
    const t0 = await page.evaluate(() => performance.now());
    await sel.selectOption(value);
    const ms = await page.evaluate(({ t0: start, n }) => new Promise<number>((res, rej) => {
      const tick = () => {
        const done = performance.getEntriesByType("resource").length > n && !document.querySelector("[data-testid=skeleton]") && !document.querySelector(".tbl-wrap[aria-busy]");
        if (done) return res(performance.now() - start);
        if (performance.now() - start > 20_000) return rej(new Error("timeout"));
        requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    }), { t0, n: before });
    view.push(ms);
  }
  record("jobs_filter_sort_ui_ms_includes_playwright_input", view);
  expect(bff.length).toBe(200);
});

test("route transitions between console pages (client-side navigation, live data)", async ({ page }) => {
  await openConsole(page, "operator", "/app");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  const routes = ["/app/jobs", "/app/workers", "/app/observability", "/app/activity", "/app", "/app/settings"];
  const shell: Record<string, number[]> = {};
  const ready: Record<string, number[]> = {};
  for (let r = 0; r < 10; r++) {
    for (const href of routes) {
      const link = page.locator(`nav[aria-label=Console] a[href="${href}"]`).first();
      if ((await link.getAttribute("aria-current")) === "page") continue;
      const t0 = await page.evaluate(() => performance.now());
      await link.click();
      const m = await page.evaluate(({ t0: start, h }) => new Promise<{ shell: number; ready: number }>((res, rej) => {
        let shell = -1;
        const tick = () => {
          const now = performance.now();
          if (shell < 0 && location.pathname === h && document.querySelector("main h1")) shell = now - start;
          if (shell >= 0 && !document.querySelector("[data-testid=skeleton]") && !document.querySelector("[aria-busy=true]")) return res({ shell, ready: now - start });
          if (now - start > 20_000) return rej(new Error(`timeout ${h}`));
          requestAnimationFrame(tick);
        };
        requestAnimationFrame(tick);
      }), { t0, h: href });
      (shell[href] ??= []).push(m.shell);
      (ready[href] ??= []).push(m.ready);
    }
  }
  const all = (o: Record<string, number[]>) => Object.values(o).flat();
  record("route_transition_shell_ms_all_routes", all(shell));
  record("route_transition_data_ready_ms_all_routes", all(ready));
  for (const h of routes) { const v = ready[h]; if (v) record(`route_transition_data_ready_ms${h.replace(/\//g, "_")}`, v); }
  expect(all(ready).length).toBeGreaterThan(40);
});

test("topology and workers table with 100 registered workers", async ({ page }) => {
  // Direct SQL on the throwaway cluster: 100 worker rows (70 live, 15 stale, 10 draining, 5 deregistered), all with the example capability.
  sql(`WITH c AS (SELECT id FROM worker_credentials LIMIT 1)
       INSERT INTO workers (name, credential_id, current_incarnation_id, status, capacity, protocol_version, sdk_version, hostname, last_heartbeat_at, drain_requested_at, drain_deadline_at, deregistered_at)
       SELECT 'perf-worker-' || lpad(g::text, 3, '0'), c.id, gen_random_uuid(),
              CASE WHEN g <= 85 THEN 'active' WHEN g <= 95 THEN 'draining' ELSE 'deregistered' END, 4, 1, 'perf', 'host-' || g,
              CASE WHEN g <= 70 THEN now() WHEN g <= 85 THEN now() - interval '10 minutes' ELSE now() END,
              CASE WHEN g BETWEEN 86 AND 95 THEN now() END, CASE WHEN g BETWEEN 86 AND 95 THEN now() + interval '1 hour' END, CASE WHEN g > 95 THEN now() END
       FROM generate_series(1, ${WORKERS}) g, c`);
  sql("INSERT INTO worker_capabilities (worker_id, handler_name, handler_version) SELECT id, 'hash_sha256', 1 FROM workers WHERE name LIKE 'perf-worker-%'");
  const keepAlive = () => sql("UPDATE workers SET last_heartbeat_at = now() WHERE name LIKE 'perf-worker-%' AND (substr(name, 13)::int <= 70)");
  await openConsole(page, "operator", "/app/jobs");
  const topo: number[] = [];
  const workersView: number[] = [];
  const toDetail: number[] = [];
  let rows = 0, nodes = 0;
  for (let i = 0; i < 15; i++) {
    keepAlive();
    // Overview with the topology panel: from the Jobs page to the topology list painted.
    let t0 = await page.evaluate(() => performance.now());
    await page.locator('nav[aria-label=Console] a[href="/app"]').first().click();
    topo.push(await page.evaluate((s) => new Promise<number>((res, rej) => {
      const tick = () => {
        if (document.querySelector("section[aria-labelledby=topology-h] ul[aria-label=Workers] li") && !document.querySelector("section[aria-labelledby=topology-h] [data-testid=skeleton]")) return res(performance.now() - s);
        if (performance.now() - s > 20_000) return rej(new Error("topology timeout"));
        requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    }), t0));
    // Interaction: open a worker from the topology list.
    t0 = await page.evaluate(() => performance.now());
    await page.locator("section[aria-labelledby=topology-h] ul[aria-label=Workers] li a").first().click();
    toDetail.push(await page.evaluate((s) => new Promise<number>((res, rej) => {
      const tick = () => {
        if (location.pathname.startsWith("/app/workers/") && document.querySelector("main h1") && !document.querySelector("[data-testid=skeleton]")) return res(performance.now() - s);
        if (performance.now() - s > 20_000) return rej(new Error("detail timeout"));
        requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    }), t0));
    // The full workers table (limit 100 rows).
    t0 = await page.evaluate(() => performance.now());
    await page.locator('nav[aria-label=Console] a[href="/app/workers"]').first().click();
    workersView.push(await page.evaluate((s) => new Promise<number>((res, rej) => {
      const tick = () => {
        if (location.pathname === "/app/workers" && document.querySelectorAll("table tbody tr").length >= 50 && !document.querySelector("[data-testid=skeleton]")) return res(performance.now() - s);
        if (performance.now() - s > 20_000) return rej(new Error("workers timeout"));
        requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    }), t0));
    rows = await page.locator("table tbody tr").count();
    nodes = await page.evaluate(() => document.getElementsByTagName("*").length);
    await page.locator('nav[aria-label=Console] a[href="/app/jobs"]').first().click();
    await expect(page.getByRole("heading", { level: 1 })).toContainText("Jobs");
  }
  record("topology_panel_ready_ms_100_workers", topo);
  record("topology_to_worker_detail_ms_100_workers", toDetail);
  record("workers_table_ready_ms_100_workers", workersView);
  results.workers_table_dom = { rowsShown: rows, domNodes: nodes };
});

test("live-update staleness: change committed to change visible", async ({ page }) => {
  await openConsole(page, "operator", "/app/jobs");
  await expect(page.locator(".live[role=status]")).toHaveAttribute("data-live", "live");
  // (a) a new job: time until the jobs list shows "Jobs changed." (the stream hint), then Refresh to reset.
  const hint: number[] = [];
  for (let i = 0; i < 15; i++) {
    await page.evaluate(() => { (window as unknown as { __seen: number }).__seen = 0; new MutationObserver(() => { const w = window as unknown as { __seen: number }; if (!w.__seen && document.body.innerText.includes("Jobs changed.")) w.__seen = performance.timeOrigin + performance.now(); }).observe(document.body, { childList: true, subtree: true, characterData: true }); });
    const r = await submit("hash_sha256", { data: `live-${i}` });
    const committed = Date.now();
    expect(r.status).toBe(201);
    await page.waitForFunction(() => (window as unknown as { __seen: number }).__seen > 0, undefined, { timeout: 20_000 });
    hint.push((await page.evaluate(() => (window as unknown as { __seen: number }).__seen)) - committed);
    await page.getByRole("button", { name: "Refresh list" }).click();
    await expect(page.getByText("Jobs changed.")).toHaveCount(0);
    await page.waitForTimeout(1200);
  }
  record("live_hint_to_jobs_list_notice_ms", hint);

  // (b) a job's detail page: the server finishes it, how long until the page says Succeeded. The server time is a 10 ms poll of the API.
  const detail: number[] = [];
  for (let i = 0; i < 12; i++) {
    const r = await submit("hash_sha256", { data: `detail-${i}` }, { run_after: new Date(Date.now() + 3500).toISOString() });
    const id = ((await r.json()) as { id: string }).id;
    await page.goto(`/app/jobs/${id}`);
    await expect(page.locator(".live[role=status]")).toHaveAttribute("data-live", "live");
    await page.evaluate(() => { const w = window as unknown as { __ok: number }; w.__ok = 0; new MutationObserver(() => { if (!w.__ok && [...document.querySelectorAll(".status")].some((e) => /Succeeded/.test(e.textContent ?? ""))) w.__ok = performance.timeOrigin + performance.now(); }).observe(document.body, { childList: true, subtree: true, characterData: true }); });
    let server = 0;
    for (let t = 0; t < 2000 && !server; t++) { const j = (await (await api(`/api/v1/jobs/${id}`)).json()) as { status: string }; if (j.status === "succeeded") server = Date.now(); else await new Promise((x) => setTimeout(x, 10)); }
    expect(server).toBeGreaterThan(0);
    await page.waitForFunction(() => (window as unknown as { __ok: number }).__ok > 0, undefined, { timeout: 20_000 });
    detail.push((await page.evaluate(() => (window as unknown as { __ok: number }).__ok)) - server);
  }
  record("live_job_detail_succeeded_visible_after_server_ms", detail);
});

test.afterAll(async ({ browser }) => {
  const project = test.info().project.name;
  const cpus = os.cpus();
  const meta = {
    when: new Date().toISOString(), project, browser: `${browser.browserType().name()} ${browser.version()}`,
    host: { platform: `${os.platform()} ${os.release()} ${os.arch()}`, cpu: cpus[0]?.model, cores: cpus.length, memGiB: Math.round(os.totalmem() / 2 ** 30), node: process.version },
    stack: "E2E harness: production Next build + node server.ts, Go control plane and 2 real workers on the same machine, PostgreSQL 18 (fsync off), loopback. Headless browser, no CPU or network throttling.",
    dataset: { jobs: JOBS, workersSeeded: WORKERS, note: "~10% scheduled, ~54% hash_sha256 succeeded, ~20% sleep_ms succeeded, 10% fail_n_times (retried then succeeded), 10% permanent failures" },
    notes: "Times are measured inside the page with performance.now() and animation frames (paint granularity ~16 ms). jobs_filter_sort_ui includes Playwright's selectOption overhead. Lab numbers on a developer laptop: not user-experience or capacity claims.",
  };
  mkdirSync(OUT, { recursive: true });
  writeFileSync(resolve(OUT, `console-perf-${project}.json`), JSON.stringify({ meta, summary: results, samples: raw }, null, 2) + "\n");
});
