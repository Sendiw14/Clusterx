import { notImplemented } from "@/lib/bff-contract";

export const dynamic = "force-dynamic";

/**
 * Authenticated proxy to the Go /api/v1 (ADR-0006): mints a fresh service assertion per request, never forwards the
 * browser cookie, passes status, body and the allowlisted headers through unchanged. Mutating methods require
 * Origin plus X-CSRF-Token. Not implemented until M5: every method answers the typed 501 envelope.
 */
const handler = () => notImplemented("The API proxy");
export { handler as GET, handler as POST, handler as PUT, handler as PATCH, handler as DELETE };
