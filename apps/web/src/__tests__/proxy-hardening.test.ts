import { describe, expect, it } from "vitest";
import { readCapped } from "@/server/body";
import { proxyPath } from "@/server/proxy-path";

const P = (s: string) => proxyPath(`/bff/api/v1/${s}`);

describe("proxyPath (RV6-01)", () => {
  it("accepts exactly the OpenAPI routes the console uses and rebuilds them", () => {
    const id = "0193a0f2-7c1e-7a10-8000-000000000001";
    for (const ok of ["jobs", `jobs/${id}`, `jobs/${id}/attempts`, `jobs/${id}/events`, `jobs/${id}/cancel`, `jobs/${id}/retry`, "workers", `workers/${id}`, `workers/${id}/drain`, "metrics/overview", "metrics/timeseries", "audit", "me", "handlers", "config", "tokens", `tokens/${id}/revoke`, "worker-credentials", `worker-credentials/${id}/revoke`]) {
      expect(P(ok), ok).toBe(`/api/v1/${ok}`);
    }
  });

  it("rejects everything else, including sessions, stream and health", () => {
    for (const bad of ["", "sessions", "sessions/current", "stream", "health/ready", "unknown", "jobs/", "jobs//cancel", "jobs/a/b/c", "jobs/a/cancel/x", "metrics", "tokens/x", "tokens/x/revoke/y", "jobs/has space"]) {
      expect(P(bad), bad).toBeNull();
    }
    expect(proxyPath("/api/v1/jobs")).toBeNull();
    expect(proxyPath("/bff/api/v1")).toBeNull();
  });

  it("rejects encoded separators, dots and traversal in any case (the reviewer's probes)", () => {
    for (const bad of [
      "x%2F..%2F..%2F..%2Fhealth%2Fready", "x%2F..%2Fstream", "x%2F..%2Fsessions%2Fcurrent", "jobs%2F..%2Fme", "jobs%2f..%2fme",
      "jobs%5C..%5Cme", "jobs%5c..%5cme", "jobs/%2e%2e/me", "jobs/%2E%2E/me", "%2e%2e/health/ready", "jobs%3Fx=1", "jobs%23frag", "jobs/..%2fsessions", "jobs/../me", "jobs/./x", "jobs%00", "jobs/%zz",
    ]) {
      expect(P(bad), bad).toBeNull();
    }
  });

  it("decodes harmless escapes and re-encodes the result", () => {
    expect(P("jobs/a%2Db")).toBe("/api/v1/jobs/a-b");
  });
});

function body(totalBytes: number, chunk = 64 * 1024) {
  let sent = 0;
  let pulls = 0;
  const stream = new ReadableStream<Uint8Array>({
    pull(c) {
      pulls++;
      const n = Math.min(chunk, totalBytes - sent);
      if (n <= 0) return c.close();
      sent += n;
      c.enqueue(new Uint8Array(n));
    },
  });
  return { stream, read: () => sent, pulls: () => pulls };
}

describe("readCapped (RV6-02)", () => {
  const MAX = 1024 * 1024;
  it("refuses a Content-Length over the cap without reading", async () => {
    const b = body(2 * MAX);
    expect(await readCapped({ headers: new Headers({ "content-length": String(2 * MAX) }), body: b.stream }, MAX)).toBeNull();
    expect(b.read()).toBeLessThanOrEqual(64 * 1024);
  });
  it("refuses a malformed Content-Length", async () => {
    expect(await readCapped({ headers: new Headers({ "content-length": "abc" }), body: body(10).stream }, MAX)).toBeNull();
  });
  it("cuts a streamed 2 MiB body off at the cap (reads about 1 MiB, not 2)", async () => {
    const b = body(2 * MAX);
    expect(await readCapped({ headers: new Headers(), body: b.stream }, MAX)).toBeNull();
    expect(b.read()).toBeLessThanOrEqual(MAX + 2 * 64 * 1024);
    expect(b.read()).toBeLessThan(2 * MAX);
  });
  it("cuts off a body that is bigger than its declared Content-Length", async () => {
    const b = body(2 * MAX);
    expect(await readCapped({ headers: new Headers({ "content-length": "10" }), body: b.stream }, MAX)).toBeNull();
  });
  it("returns the bytes of a body at or under the cap, and empty for no body", async () => {
    const ok = await readCapped({ headers: new Headers(), body: body(MAX).stream }, MAX);
    expect(ok?.byteLength).toBe(MAX);
    expect((await readCapped({ headers: new Headers(), body: null }, MAX))?.byteLength).toBe(0);
  });
});
