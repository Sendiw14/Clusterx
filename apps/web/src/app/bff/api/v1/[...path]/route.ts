import { asUser, bffError, gate, passThrough } from "@/server/bff";

export const dynamic = "force-dynamic";

/** Request headers the BFF forwards. Never the browser cookie or Authorization. */
const FORWARD = ["content-type", "accept", "idempotency-key"];
const MAX_BODY = 1024 * 1024;
/** BFF-only operations (login/logout/identity) are not reachable through the proxy; the stream has its own route. */
const DENIED = /^(sessions(\/|$)|stream$)/;

/**
 * Authenticated proxy to the Go /api/v1 (ADR-0006): fresh assertion per request, status, body and allowlisted headers
 * passed through unchanged. Mutations require the exact Origin and X-CSRF-Token (checked in `gate`).
 */
async function handler(req: Request, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  const mutating = !["GET", "HEAD"].includes(req.method);
  const g = gate(req, { mutating });
  if (g instanceof Response) return g;
  const sub = path.join("/");
  if (DENIED.test(sub) || path.some((p) => p === ".." || p === "." || p.includes("\\") || p.includes("%"))) return bffError(404, "not_found_route", "No such route.", g.rid);
  const url = new URL(req.url);
  const headers: Record<string, string> = {};
  for (const h of FORWARD) {
    const v = req.headers.get(h);
    if (v) headers[h] = v;
  }
  let body: Uint8Array | undefined;
  if (mutating && req.method !== "DELETE") {
    body = new Uint8Array(await req.arrayBuffer());
    if (body.length > MAX_BODY) return bffError(413, "payload_too_large", "The request body is too large.", g.rid);
  }
  const out = await asUser(g, { method: req.method, path: `/api/v1/${sub}${url.search}`, body, headers });
  return "res" in out ? out.res : passThrough(out.go);
}
export { handler as GET, handler as POST, handler as PUT, handler as PATCH, handler as DELETE };
