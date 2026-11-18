import "server-only";
import type { BffSessionInfo } from "./bff-contract";

/**
 * Server-only session module (ADR-0006). M3 ships the interface and no implementation:
 *  - `unavailable`: the BFF cannot establish sessions in this build (always the case until M5). Console routes render
 *    their shell with an explicit banner and every data call answers 501, so nothing is shown that was not authorized.
 *  - `none`: no valid session. Console routes redirect to `/login?next=` (wired when sessions exist).
 *  - `active`: an authenticated principal and role from the BFF session store.
 * The UI never decides authorization: Go enforces it on every request. The role here only hides controls.
 */
export type Session =
  | { kind: "unavailable"; reason: "not_implemented" }
  | { kind: "none" }
  | ({ kind: "active" } & BffSessionInfo);

export async function getSession(): Promise<Session> {
  return { kind: "unavailable", reason: "not_implemented" };
}

export function roleOf(s: Session): "operator" | "viewer" | null {
  return s.kind === "active" ? s.role : null;
}
