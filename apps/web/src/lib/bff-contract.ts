/**
 * BFF-owned contract types (ADR-0006). Interfaces only: the OAuth flow, session store and proxy are implemented in M5.
 * BFF-owned failures use the same `{code, message, details?, request_id}` envelope as the Go API.
 */
export type BffErrorCode = "not_implemented" | "session_expired" | "csrf_failed" | "upstream_auth" | "upstream_unavailable" | "forbidden" | "bad_request";

export interface BffErrorBody {
  code: BffErrorCode;
  message: string;
  details?: Record<string, unknown>;
  request_id: string;
}

/** `GET /bff/session` response (ADR-0006 section 3). 401 from that route means signed out. */
export interface BffSessionInfo {
  user: { id: string; login: string };
  role: "operator" | "viewer";
  expires_at: string;
  idle_expires_at: string;
  csrf_token: string;
}

export function requestId(): string {
  return `req_${crypto.randomUUID().replace(/-/g, "").slice(0, 12)}`;
}

/** Typed 501 envelope returned by every BFF route until M5. Not a stub that pretends to work: it says so. */
export function notImplemented(what: string): Response {
  const rid = requestId();
  const body: BffErrorBody = {
    code: "not_implemented",
    message: `${what} is not implemented in this build. The BFF is a skeleton until the authentication milestone.`,
    request_id: rid,
  };
  return Response.json(body, { status: 501, headers: { "X-Request-Id": rid, "Cache-Control": "no-store" } });
}
