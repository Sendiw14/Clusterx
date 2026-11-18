import { notImplemented } from "@/lib/bff-contract";

export const dynamic = "force-dynamic";

/** Browser to BFF stream (ADR-0006 section 4). A WebSocket relay needs a dedicated server in M6; 501 until then. */
export function GET() {
  return notImplemented("The live stream");
}
