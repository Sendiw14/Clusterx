import { notFound } from "next/navigation";

/**
 * Every unmatched URL (specific routes win over a catch-all) answers the brand 404 from a per-request render, so under the
 * nonce CSP of /app, /login and /auth (any letter case) Next stamps the request nonce on the 404's scripts and the page hydrates.
 * The root `not-found.tsx` stays static: calling `connection()` there made every public route dynamic (AUDIT-M9 F-05).
 */
export const dynamic = "force-dynamic";

export default function Unmatched(): never {
  notFound();
}
