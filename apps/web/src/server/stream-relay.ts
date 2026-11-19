import type { IncomingMessage, Server } from "node:http";
import type { Duplex } from "node:stream";
import { WebSocket, WebSocketServer } from "ws";
import { mintAssertion } from "./assertion.ts";
import { config } from "./config.ts";
import { readSession } from "./cookies.ts";
import { requestId } from "./crypto.ts";

/** ADR-0006 section 4: at most 5 browser sockets per session (Go also enforces 4429). */
export const MAX_SOCKETS_PER_SESSION = 5;
const PING_MS = 25_000;
const live = new Map<string, number>();

function refuse(socket: Duplex, status: number, text: string) {
  socket.end(`HTTP/1.1 ${status} ${text}\r\nConnection: close\r\nContent-Length: 0\r\nCache-Control: no-store\r\n\r\n`);
}

/** Valid on the wire (RFC 6455): 1005/1006/1015 must never be sent. */
const wireCode = (c: number) => (c === 1000 || (c >= 1001 && c <= 1014 && c !== 1004 && c !== 1005 && c !== 1006) || (c >= 3000 && c <= 4999) ? c : 1011);

/**
 * Browser -> BFF `/bff/stream` relay. The browser authenticates with its session cookie (Origin must equal the app
 * origin); the BFF opens the upstream socket to Go with a fresh `Authorization: Bearer <assertion>` and no Origin
 * header (server-to-server). Frames are copied unchanged; Go's close codes (4401/4403/4408/4429/1001...) are relayed
 * unchanged. The cookie never leaves the BFF. Nothing in the frames is persisted or logged.
 */
export function attachStreamRelay(server: Server, passthrough: (req: IncomingMessage, socket: Duplex, head: Buffer) => void): void {
  const wss = new WebSocketServer({ noServer: true, maxPayload: 1024 });
  server.on("upgrade", (req, socket, head) => {
    if (new URL(req.url ?? "/", "http://x").pathname !== "/bff/stream") return passthrough(req, socket, head);
    void accept(req, socket, head).catch((e) => {
      console.error(`[bff] stream relay failure: ${(e as Error).name}`);
      refuse(socket, 502, "Bad Gateway");
    });
  });

  async function accept(req: IncomingMessage, socket: Duplex, head: Buffer) {
    const rid = requestId();
    const cfg = config();
    if (!cfg.ok) {
      console.error(`[bff] ${rid} configuration problems: ${cfg.problems.join("; ")}`);
      return refuse(socket, 502, "Bad Gateway");
    }
    const c = cfg.config;
    if (req.headers.origin !== c.appOrigin) return refuse(socket, 403, "Forbidden");
    const s = readSession(c, req.headers.cookie);
    if (!s) return refuse(socket, 401, "Unauthorized");
    if ((live.get(s.sid) ?? 0) >= MAX_SOCKETS_PER_SESSION) return refuse(socket, 429, "Too Many Requests");
    const token = await mintAssertion(c.keyFile, { userId: s.uid, sid: s.sid, role: s.role, rid });
    const up = new WebSocket(`${c.apiUrl.replace(/^http/, "ws")}/api/v1/stream`, { headers: { Authorization: `Bearer ${token}`, "X-Request-Id": rid }, handshakeTimeout: 10_000, maxPayload: 64 * 1024 });
    up.once("unexpected-response", (_r, res) => {
      res.resume();
      refuse(socket, res.statusCode === 401 || res.statusCode === 403 || res.statusCode === 429 ? res.statusCode : 502, res.statusMessage || "Bad Gateway");
    });
    up.once("error", () => refuse(socket, 502, "Bad Gateway"));
    up.once("open", () => {
      up.removeAllListeners("error");
      wss.handleUpgrade(req, socket, head, (down) => {
        live.set(s.sid, (live.get(s.sid) ?? 0) + 1);
        let alive = true;
        const timer = setInterval(() => {
          if (!alive) return down.terminate();
          alive = false;
          down.ping();
        }, PING_MS);
        down.on("pong", () => { alive = true; });
        const end = () => {
          clearInterval(timer);
          const n = (live.get(s.sid) ?? 1) - 1;
          if (n <= 0) live.delete(s.sid);
          else live.set(s.sid, n);
        };
        down.on("message", (data, isBinary) => { if (!isBinary && up.readyState === WebSocket.OPEN) up.send(data.toString()); });
        up.on("message", (data, isBinary) => {
          if (down.readyState !== WebSocket.OPEN) return;
          if (down.bufferedAmount > 1_000_000) return down.close(1013);
          down.send(isBinary ? data : data.toString());
        });
        up.on("close", (code) => { if (down.readyState === WebSocket.OPEN) down.close(wireCode(code)); });
        up.on("error", () => { if (down.readyState === WebSocket.OPEN) down.close(1011); });
        down.on("close", () => { end(); if (up.readyState === WebSocket.OPEN || up.readyState === WebSocket.CONNECTING) up.close(1000); });
        down.on("error", () => down.terminate());
      });
    });
  }
}
