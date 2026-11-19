import { createServer } from "node:http";
import next from "next";
import { attachStreamRelay } from "./src/server/stream-relay.ts";

/**
 * Production entry (`pnpm start`). A plain `next start` cannot upgrade WebSockets, so this thin server hosts Next and
 * the `/bff/stream` relay (ADR-0006 section 4). Every other upgrade (Next's own dev HMR) is handed back to Next.
 */
const dev = process.env.NODE_ENV === "development";
const port = Number(process.env.PORT ?? 3000);
const hostname = process.env.HOSTNAME ?? "0.0.0.0";

const app = next({ dev, hostname, port, dir: import.meta.dirname });
await app.prepare();
const handle = app.getRequestHandler();
const server = createServer((req, res) => void handle(req, res));
attachStreamRelay(server, (req, socket, head) => void app.getUpgradeHandler()(req, socket, head));
server.listen(port, hostname, () => console.log(`[web] listening on ${hostname}:${port}`));

for (const sig of ["SIGINT", "SIGTERM"] as const) {
  process.on(sig, () => {
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(0), 2000).unref();
  });
}
