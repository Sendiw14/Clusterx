import { bffError } from "@/server/bff";
import { requestId } from "@/server/crypto";

export const dynamic = "force-dynamic";

/**
 * The browser stream is a WebSocket upgrade handled by `server.ts` (the relay) before Next sees the request
 * (ADR-0006 section 4). A plain HTTP GET here means the app is running without that server (for example `next dev`):
 * answer 426 so the client falls back to polling.
 */
export function GET() {
  return bffError(426, "upstream_unavailable", "The live stream needs the CLUSTERX web server. Polling is used instead.", requestId(), { headers: { Upgrade: "websocket" } });
}
