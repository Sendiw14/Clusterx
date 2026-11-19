/**
 * Positive allowlist for the `/bff/api/v1/*` proxy (RV6-01). The console only ever calls the OpenAPI routes below; any other
 * path (sessions, stream, health, anything Go may add later) is not reachable. The path is parsed from the RAW request
 * pathname: Next decodes `%2F` inside a catch-all segment, which is how `x%2F..%2Fsessions%2Fcurrent` used to slip past a
 * segment deny-list. Plain Node module (no `@/` alias) so it is unit-testable.
 */
const ID = "[A-Za-z0-9_-]{1,64}";
const ROUTES: RegExp[] = [
  "jobs", `jobs/${ID}`, `jobs/${ID}/attempts`, `jobs/${ID}/events`, `jobs/${ID}/cancel`, `jobs/${ID}/retry`,
  "workers", `workers/${ID}`, `workers/${ID}/drain`,
  "metrics/overview", "metrics/timeseries", "audit", "me", "handlers", "config",
  "tokens", `tokens/${ID}/revoke`, "worker-credentials", `worker-credentials/${ID}/revoke`,
].map((t) => new RegExp(`^${t}$`));

const PREFIX = "/bff/api/v1/";

/** Returns the upstream path (`/api/v1/...`, no query) or null when the request must be a 404. */
export function proxyPath(rawPathname: string): string | null {
  if (!rawPathname.startsWith(PREFIX)) return null;
  const rest = rawPathname.slice(PREFIX.length);
  // Encoded separators and dots are never legitimate in an allowlisted path (ids are [A-Za-z0-9_-]).
  if (/%(2f|5c|2e)/i.test(rest)) return null;
  const segments: string[] = [];
  for (const raw of rest.split("/")) {
    let s: string;
    try {
      s = decodeURIComponent(raw);
    } catch {
      return null;
    }
    if (!s || s === "." || s === ".." || /[/\\?#\u0000-\u001f]/.test(s)) return null;
    segments.push(s);
  }
  const sub = segments.join("/");
  if (!ROUTES.some((r) => r.test(sub))) return null;
  const path = `/api/v1/${segments.map(encodeURIComponent).join("/")}`;
  // Belt and braces: the URL parser must agree with the string we built.
  if (new URL(path, "http://x").pathname !== path || !path.startsWith("/api/v1/")) return null;
  return path;
}
