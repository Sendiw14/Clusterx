import { describe, expect, it } from "vitest";
import { GET as sessionGET } from "@/app/bff/session/route";
import { POST as logoutPOST } from "@/app/bff/logout/route";
import { GET as proxyGET, POST as proxyPOST } from "@/app/bff/api/v1/[...path]/route";
import { GET as streamGET } from "@/app/bff/stream/route";

describe("BFF skeleton: typed 501 not_implemented, no session or OAuth behaviour", () => {
  const routes: [string, () => Response][] = [["GET /bff/session", sessionGET], ["POST /bff/logout", logoutPOST], ["GET /bff/api/v1/*", proxyGET], ["POST /bff/api/v1/*", proxyPOST], ["GET /bff/stream", streamGET]];
  for (const [name, h] of routes) {
    it(`${name} answers the envelope {code, message, request_id} with no-store`, async () => {
      const r = h();
      expect(r.status).toBe(501);
      const j = await r.json();
      expect(Object.keys(j).sort()).toEqual(["code", "message", "request_id"]);
      expect(j.code).toBe("not_implemented");
      expect(j.request_id).toMatch(/^req_[0-9a-f]{12}$/);
      expect(r.headers.get("X-Request-Id")).toBe(j.request_id);
      expect(r.headers.get("Cache-Control")).toBe("no-store");
      expect(r.headers.get("Set-Cookie")).toBeNull();
    });
  }
});
