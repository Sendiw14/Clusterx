import { NextResponse, type NextRequest } from "next/server";

/**
 * Console only: hands the requested path to the server-rendered layout so an ended session can return to it after
 * sign-in. Any client-supplied `x-pathname` is overwritten, and the value is clamped again before use.
 */
export function proxy(req: NextRequest) {
  const h = new Headers(req.headers);
  h.set("x-pathname", req.nextUrl.pathname + req.nextUrl.search);
  return NextResponse.next({ request: { headers: h } });
}

export const config = { matcher: ["/app/:path*"] };
