import { afterEach, describe, expect, it, vi } from "vitest";
import { FixtureProvider } from "@/data/fixture-provider";
import { FIXTURE_ATTEMPTS, FIXTURE_EVENTS, FIXTURE_JOBS, FIXTURE_WORKERS } from "@/data/fixtures/data";
import { LiveProvider } from "@/data/live-provider";

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); });

describe("FixtureProvider (demo)", () => {
  it("every fixture ID carries the synthetic demo- prefix", () => {
    const ids = [
      ...FIXTURE_JOBS.map((j) => j.id), ...FIXTURE_WORKERS.map((w) => w.id),
      ...Object.values(FIXTURE_ATTEMPTS).flat().flatMap((a) => [a.id, a.job_id, a.worker_id]),
      ...Object.values(FIXTURE_EVENTS).flat().map((e) => e.job_id),
    ];
    expect(ids.length).toBeGreaterThan(20);
    for (const id of ids) expect(id, id).toMatch(/^demo-/);
  });
  it("makes no network call and has no timers", async () => {
    const f = vi.fn();
    vi.stubGlobal("fetch", f);
    const p = new FixtureProvider();
    await Promise.all([p.getOverview(), p.listJobs({}), p.getJob("demo-job-0001"), p.listAttempts("demo-job-0002"), p.listJobEvents("demo-job-0002"), p.listWorkers({}), p.getWorker("demo-worker-1"), p.getConfig()]);
    expect(f).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });
  it("filters by status, handler and priority, sorts newest first, paginates by opaque cursor", async () => {
    const p = new FixtureProvider();
    const failed = await p.listJobs({ status: ["failed"] });
    expect(failed.ok && failed.data.items.map((j) => j.status)).toEqual(["failed", "failed"]);
    const thumb = await p.listJobs({ handler_name: "thumbnail", priority_min: 5 });
    expect(thumb.ok && thumb.data.items.every((j) => j.handler_name === "thumbnail" && j.priority >= 5)).toBe(true);
    const page1 = await p.listJobs({ limit: 5 });
    if (!page1.ok) throw new Error("x");
    expect(page1.data.items).toHaveLength(5);
    const times = page1.data.items.map((j) => j.created_at);
    expect([...times].sort().reverse()).toEqual(times);
    const page2 = await p.listJobs({ limit: 5, cursor: page1.data.next_cursor! });
    if (!page2.ok) throw new Error("x");
    expect(page2.data.items.map((j) => j.id)).not.toContain(page1.data.items[0]!.id);
    const last = await p.listJobs({ limit: 5, cursor: page2.data.next_cursor! });
    expect(last.ok && last.data.next_cursor).toBeNull();
  });
  it("list items carry no payload fields; viewer view removes payload, result, error and log text from the objects", async () => {
    const op = new FixtureProvider("operator");
    const vw = new FixtureProvider("viewer");
    const list = await op.listJobs({});
    expect(list.ok && list.data.items.some((j) => "payload" in j || "result" in j)).toBe(false);
    const a = await op.getJob("demo-job-0003");
    const b = await vw.getJob("demo-job-0003");
    if (!a.ok || !b.ok) throw new Error("x");
    expect(a.data.payload).not.toBeNull();
    expect(b.data.payload).toBeNull();
    expect(b.data.result).toBeNull();
    expect(b.data.error_message).toBeNull();
    expect(b.data.masked_fields).toEqual(["payload", "result", "error_message"]);
    const att = await vw.listAttempts("demo-job-0003");
    if (!att.ok) throw new Error("x");
    expect(att.data.items.every((x) => x.log_tail === null && x.error_message === null && x.masked_fields.includes("log_tail"))).toBe(true);
    expect(JSON.stringify(att.data)).not.toContain("POST demo://upstream");
    const w = await vw.getWorker("demo-worker-1");
    expect(w.ok && w.data.hostname).toBeNull();
  });
  it("unknown IDs are typed 404s", async () => {
    const p = new FixtureProvider();
    const j = await p.getJob("demo-job-9999");
    expect(!j.ok && j.error.code).toBe("job_not_found");
    const w = await p.getWorker("nope");
    expect(!w.ok && w.error.code).toBe("worker_not_found");
  });
});

describe("LiveProvider (console, through the BFF)", () => {
  const env = (code: string, rid = "req_1") => new Response(JSON.stringify({ code, message: "m", request_id: rid }), { status: code === "not_implemented" ? 501 : 503, headers: { "content-type": "application/json", "X-Request-Id": rid } });

  it("calls only same-origin /bff/api/v1 paths with credentials same-origin and no Authorization header", async () => {
    const seen: Request[] = [];
    vi.stubGlobal("fetch", vi.fn(async (r: Request) => { seen.push(r); return Response.json({ items: [], next_cursor: null, as_of: "2026-01-01T00:00:00Z" }); }));
    const p = new LiveProvider("http://localhost/bff");
    const r = await p.listJobs({ status: ["failed", "queued"], handler_name: "x", priority_min: 3, limit: 50 });
    expect(r.ok).toBe(true);
    const u = new URL(seen[0]!.url, "http://localhost");
    expect(u.pathname).toBe("/bff/api/v1/jobs");
    expect(u.searchParams.getAll("status")).toEqual(["failed", "queued"]);
    expect(u.searchParams.get("handler_name")).toBe("x");
    expect(seen[0]!.headers.get("authorization")).toBeNull();
    expect(seen[0]!.credentials).toBe("same-origin");
    vi.unstubAllGlobals();
  });
  it("maps the typed 501 envelope to a failure with code, status and request id", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => env("not_implemented", "req_77")));
    const r = await new LiveProvider("http://localhost/bff").getOverview();
    expect(r).toEqual({ ok: false, error: expect.objectContaining({ code: "not_implemented", status: 501, requestId: "req_77" }) });
    vi.unstubAllGlobals();
  });
  it("503 keeps Retry-After; non-JSON error bodies fall back to internal without echoing text", async () => {
    vi.stubGlobal("fetch", vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: "unavailable", message: "m", request_id: "r" }), { status: 503, headers: { "Retry-After": "30" } }))
      .mockResolvedValueOnce(new Response("<html>boom</html>", { status: 500 })));
    const a = await new LiveProvider("http://localhost/bff").getConfig();
    expect(!a.ok && a.error.retryAfterSeconds).toBe(30);
    const b = await new LiveProvider("http://localhost/bff").getConfig();
    expect(!b.ok && b.error.code).toBe("internal");
    vi.unstubAllGlobals();
  });
  it("network failure is code network; a hung request becomes timeout after 10 s", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValueOnce(new TypeError("fail")));
    const n = await new LiveProvider("http://localhost/bff").getConfig();
    expect(!n.ok && n.error.code).toBe("network");
    vi.useFakeTimers();
    vi.stubGlobal("fetch", vi.fn((r: Request) => new Promise((_, rej) => r.signal.addEventListener("abort", () => rej(new DOMException("aborted", "AbortError"))))));
    const pending = new LiveProvider("http://localhost/bff").getConfig();
    await vi.advanceTimersByTimeAsync(10_001);
    const t = await pending;
    expect(!t.ok && t.error.code).toBe("timeout");
    vi.unstubAllGlobals();
  });
});
