import { notImplemented } from "@/lib/bff-contract";

export const dynamic = "force-dynamic";

/** GET /bff/session -> BffSessionInfo (ADR-0006). 501 until M5. */
export function GET() {
  return notImplemented("Session lookup");
}
