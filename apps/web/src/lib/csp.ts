/**
 * Content-Security-Policy builder (MASTER section 7), shared by `next.config.ts` (static policy for public pages) and
 * `proxy.ts` (per-request nonce policy for the console, /login and /auth, RV6-09). Plain module: no aliases, no Next imports.
 *
 * With a nonce, `script-src` is `'self' 'nonce-...' 'strict-dynamic'` and never `'unsafe-inline'`; Next stamps the nonce on
 * its own bootstrap and flight scripts when it sees it in the request's CSP header. Without one (statically rendered public
 * pages cannot carry a per-request nonce) `'unsafe-inline'` stays, and only there.
 */
export function buildCsp(opts: { nonce?: string; dev?: boolean }): string {
  const script = opts.nonce ? `script-src 'self' 'nonce-${opts.nonce}' 'strict-dynamic'` : "script-src 'self' 'unsafe-inline'";
  return [
    "default-src 'self'",
    `${script}${opts.dev ? " 'unsafe-eval'" : ""}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data:",
    "font-src 'self'",
    "connect-src 'self'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
    "manifest-src 'self'",
  ].join("; ");
}

/** Paths that get the nonce policy from `proxy.ts`. Everything else keeps the static policy from `next.config.ts`. */
export const NONCE_PATHS = ["/app/:path*", "/login", "/auth/:path*"];
