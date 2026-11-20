import { mintAssertion, type AssertionSpec } from "./assertion.ts";
import type { WebConfig } from "./config.ts";

export interface GoResult {
  status: number;
  headers: Headers;
  body: Uint8Array;
  /** Parsed `{code, details.reason}` of a Go error body, if there is one. */
  code?: string;
  reason?: string;
  expiry?: string;
  /** `details.role` of a Go 403: the role Go authorized with (the database role). */
  role?: string;
}

/** Headers passed back to the browser unchanged (ADR-0006 section 3, BFF-02). */
export const PASS_HEADERS = ["Content-Type", "X-Request-Id", "Retry-After", "Idempotent-Replayed", "Location", "ETag", "Cache-Control", "Date"];

function parse(status: number, headers: Headers, body: Uint8Array): GoResult {
  const r: GoResult = { status, headers, body };
  if (status >= 400 && (headers.get("content-type") ?? "").includes("json")) {
    try {
      const j = JSON.parse(new TextDecoder().decode(body)) as { code?: string; details?: { reason?: string; expiry?: string; role?: string } };
      r.code = j.code;
      r.reason = j.details?.reason;
      r.expiry = j.details?.expiry;
      r.role = j.details?.role;
    } catch {
      /* not an envelope */
    }
  }
  return r;
}

export interface GoRequest {
  method: string;
  /** `/api/v1/...` plus query string, verbatim. */
  path: string;
  body?: Uint8Array;
  headers?: Record<string, string>;
  as: AssertionSpec;
}

/**
 * Calls the Go control plane with a freshly minted assertion. `assertion_expired` is retried once with a new assertion
 * (new jti), transparently (ADR-0006 section 2). Throws only when Go is unreachable or the key is unusable.
 */
export async function callGo(cfg: WebConfig, req: GoRequest, timeoutMs = 30_000): Promise<GoResult> {
  for (let attempt = 0; ; attempt++) {
    const token = await mintAssertion(cfg.keyFile, req.as);
    const res = await fetch(cfg.apiUrl + req.path, {
      method: req.method,
      headers: { ...req.headers, Authorization: `Bearer ${token}`, "X-Request-Id": req.as.rid },
      body: req.body ? Buffer.from(req.body) : undefined,
      redirect: "manual",
      signal: AbortSignal.timeout(timeoutMs),
    });
    const out = parse(res.status, res.headers, new Uint8Array(await res.arrayBuffer()));
    if (res.status === 401 && out.reason === "assertion_expired" && attempt === 0) continue;
    return out;
  }
}

/** Go says the human session ended. `user_disabled` gets its own login outcome (no re-login loop). */
export function sessionEnded(r: GoResult): "session_expired" | "user_disabled" | null {
  if (r.status !== 401 && r.status !== 403) return null;
  if (r.reason === "user_disabled") return "user_disabled";
  if (r.status === 401 && (r.reason === "session_expired" || r.reason === "session_revoked")) return "session_expired";
  return null;
}

/** Best-effort `DELETE /sessions/current` for a session cookie that is being replaced or cleared (RV6-06). Never throws; a session that already ended (401) is fine. */
export async function revokeSession(cfg: WebConfig, s: { uid: string; sid: string; role: "operator" | "viewer" }, rid: string): Promise<void> {
  try {
    await callGo(cfg, { method: "DELETE", path: "/api/v1/sessions/current", as: { userId: s.uid, sid: s.sid, role: s.role, rid } }, 3000);
  } catch (e) {
    console.error(`[bff] ${rid} session revoke failed: ${(e as Error).name}`);
  }
}
