import type { IncomingMessage, Server } from "node:http";
import type { Duplex } from "node:stream";
import { WebSocket, WebSocketServer } from "ws";
import { mintAssertion } from "./assertion.ts";
import { config } from "./config.ts";
import { readSession } from "./cookies.ts";
import { requestId } from "./crypto.ts";

/**
 * ADR-0006 section 4: at most 5 browser sockets per session (Go also enforces 4429), a per-process ceiling (RV6-08) and a
 * per-user cap across sessions (RV6B-05(b)): one allowlisted user with many sessions (each a fresh OAuth login) cannot push
 * everyone else's stream to 429 and the polling fallback. 15 is three full sessions; the ceiling still bounds the process.
 */
export const MAX_SOCKETS_PER_SESSION = 5;
export const MAX_SOCKETS_PER_USER = 15;
export const MAX_SOCKETS_TOTAL = 256;
const PING_MS = 25_000;
const live = new Map<string, number>();
const liveUser = new Map<string, number>();
let total = 0;

const bump = (m: Map<string, number>, k: string, d: 1 | -1) => {
  const n = (m.get(k) ?? 0) + d;
  if (n <= 0) m.delete(k);
  else m.set(k, n);
};

/**
 * Takes a relay slot synchronously, BEFORE the async dial to Go (RV6-08: check-then-act raced). Returns an idempotent
 * release, or null when the session, the user or the process is full.
 */
export function reserveSlot(sid: string, uid: string): (() => void) | null {
  if (total >= MAX_SOCKETS_TOTAL || (live.get(sid) ?? 0) >= MAX_SOCKETS_PER_SESSION || (liveUser.get(uid) ?? 0) >= MAX_SOCKETS_PER_USER) return null;
  bump(live, sid, 1);
  bump(liveUser, uid, 1);
  total++;
  let done = false;
  return () => {
    if (done) return;
    done = true;
    total--;
    bump(live, sid, -1);
    bump(liveUser, uid, -1);
  };
}
export const slotsInUse = () => total;

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
    // A peer that resets while we answer or dial must not surface as an uncaught `error` (RV6-03).
    socket.on("error", () => {});
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
    const release = reserveSlot(s.sid, s.uid);
    if (!release) return refuse(socket, 429, "Too Many Requests");
    // The browser socket owns the slot for exactly its lifetime. Until the handshake is handed to `ws`, its closing also
    // kills the upstream socket (RV6-03: a tab closed or a handshake `ws` aborts after the dial used to leak Go's slot).
    const conn: { up?: WebSocket; down?: WebSocket } = {};
    let handedOff = false;
    socket.once("close", () => {
      release();
      if (!handedOff) conn.up?.terminate();
    });
    // A client that sends FIN without closing (half-open) emits `end` but not `close`, which would hold the slot and the dial
    // until Go answers or the 10 s handshake timeout (RV6B-05(a)). Before the hand-off nothing is worth keeping: close now.
    socket.once("end", () => { if (!handedOff) socket.destroy(); });
    if (socket.destroyed) return release();
    const token = await mintAssertion(c.keyFile, { userId: s.uid, sid: s.sid, role: s.role, rid });
    if (socket.destroyed) return;
    const ws = new WebSocket(`${c.apiUrl.replace(/^http/, "ws")}/api/v1/stream`, { headers: { Authorization: `Bearer ${token}`, "X-Request-Id": rid }, handshakeTimeout: 10_000, maxPayload: 64 * 1024 });
    conn.up = ws;
    // Always listening: an `error` on a socket without a listener would throw and take the process down.
    ws.on("error", () => {
      if (handedOff) return void (conn.down?.readyState === WebSocket.OPEN && conn.down.close(1011));
      refuse(socket, 502, "Bad Gateway");
    });
    ws.once("unexpected-response", (_r, res) => {
      res.resume();
      refuse(socket, res.statusCode === 401 || res.statusCode === 403 || res.statusCode === 429 ? res.statusCode : 502, res.statusMessage || "Bad Gateway");
    });
    ws.once("open", () => {
      if (socket.destroyed) return ws.terminate();
      try {
        wss.handleUpgrade(req, socket, head, (d) => {
          handedOff = true;
          conn.down = d;
          pipe(ws, d);
        });
      } catch {
        ws.terminate();
        refuse(socket, 502, "Bad Gateway");
      }
    });
  }

  function pipe(up: WebSocket, down: WebSocket) {
    let alive = true;
    const timer = setInterval(() => {
      if (!alive) return down.terminate();
      alive = false;
      down.ping();
    }, PING_MS);
    down.on("pong", () => { alive = true; });
    down.on("message", (data, isBinary) => { if (!isBinary && up.readyState === WebSocket.OPEN) up.send(data.toString()); });
    up.on("message", (data, isBinary) => {
      if (down.readyState !== WebSocket.OPEN) return;
      if (down.bufferedAmount > 1_000_000) return down.close(1013);
      down.send(isBinary ? data : data.toString());
    });
    up.on("close", (code) => { if (down.readyState === WebSocket.OPEN) down.close(wireCode(code)); });
    down.on("close", () => { clearInterval(timer); if (up.readyState === WebSocket.OPEN || up.readyState === WebSocket.CONNECTING) up.close(1000); });
    down.on("error", () => down.terminate());
  }
}
