import { asUser, bffError, gate, passThrough } from "@/server/bff";
import { proxyPath } from "@/server/proxy-path";
import { readCapped } from "@/server/body";

export const dynamic = "force-dynamic";

/** Request headers the BFF forwards. Never the browser cookie or Authorization. */
const FORWARD = ["content-type", "accept", "idempotency-key"];
const MAX_BODY = 1024 * 1024;

/**
 * Authenticated proxy to the Go /api/v1 (ADR-0006): fresh assertion per request, status, body and allowlisted headers
 * passed through unchanged. Mutations require the exact Origin and X-CSRF-Token (checked in `gate`).
 */
async function handler(req: Request) {
  const mutating = !["GET", "HEAD"].includes(req.method);
  const g = gate(req, { mutating });
  if (g instanceof Response) return g;
  const url = new URL(req.url);
  // Positive allowlist on the raw pathname (RV6-01): sessions, stream, health and everything else are 404.
  const upstream = proxyPath(url.pathname);
  if (!upstream) return bffError(404, "not_found_route", "No such route.", g.rid);
  const headers: Record<string, string> = {};
  for (const h of FORWARD) {
    const v = req.headers.get(h);
    if (v) headers[h] = v;
  }
  let body: Uint8Array | undefined;
  if (mutating && req.method !== "DELETE") {
    const read = await readCapped(req, MAX_BODY);
    if (!read) return bffError(413, "payload_too_large", "The request body is too large.", g.rid);
    body = read;
  }
  const out = await asUser(g, { method: req.method, path: `${upstream}${url.search}`, body, headers });
  return "res" in out ? out.res : passThrough(out.go);
}
export { handler as GET, handler as POST, handler as PUT, handler as PATCH, handler as DELETE };
