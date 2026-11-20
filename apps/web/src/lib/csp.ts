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

/**
 * First path segments that get the nonce policy from `proxy.ts` (console, sign-in, BFF auth routes). `next.config.ts` derives
 * the static policy's exclusion from the same list, so no path gets both policies or neither (RV6B-03).
 */
export const NONCE_SEGMENTS = ["app", "login", "auth"] as const;

/** Case-insensitive, like Next's `headers()` source matching, so a letter-case variant (`/APP`, `/Login/x`) cannot fall between the two policies. */
export const isNoncePath = (pathname: string): boolean => new RegExp(`^/(?:${NONCE_SEGMENTS.join("|")})(?:/|$)`, "i").test(pathname);

/** `headers()` source for the static policy: everything except the nonce paths. */
export const STATIC_POLICY_SOURCE = `/((?!${NONCE_SEGMENTS.map((x) => `${x}(?:/|$)`).join("|")}).*)`;
