import { notImplemented } from "@/lib/bff-contract";

export const dynamic = "force-dynamic";

/**
 * POST /bff/logout. When implemented this is a mutating route: it requires the exact Origin and X-CSRF-Token
 * (ADR-0006 section 3) before anything else. 501 until M5.
 */
export function POST() {
  return notImplemented("Sign out");
}
