/**
 * BFF-owned contract types (ADR-0006). Types only (safe for client and demo graphs). Implementation lives in `src/server`.
 * BFF-owned failures use the same `{code, message, details?, request_id}` envelope as the Go API.
 */
export type BffErrorCode = "session_expired" | "csrf_failed" | "upstream_auth" | "upstream_unavailable";

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
