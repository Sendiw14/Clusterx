/**
 * BFF configuration (ADR-0006). Plain Node module: shared by the Next route handlers and `server.ts` (stream relay), so it
 * must not import `server-only` or the `@/` alias. Production fails closed: anything missing or unsafe is a problem and
 * the BFF answers `upstream_auth` (502) instead of limping along.
 *
 * Environment (all CLUSTERX_*, none is ever sent to the browser):
 *   CLUSTERX_ENV                    production (default when unset) | development | test
 *   CLUSTERX_APP_ORIGIN             exact origin of this app, e.g. https://console.example.com
 *   CLUSTERX_API_URL                Go control plane base URL
 *   CLUSTERX_BFF_KEY_FILE           path to the Ed25519 private JWK (never in the bundle)
 *   CLUSTERX_WEB_SESSION_KEY        base64, >= 32 bytes: seals cookies and derives the CSRF key
 *   CLUSTERX_GITHUB_CLIENT_ID / _SECRET
 *   CLUSTERX_GITHUB_AUTHORIZE_URL / _TOKEN_URL / _USER_URL   non-production only (mock OAuth server in tests)
 */
export interface WebConfig {
  env: "production" | "development" | "test";
  appOrigin: string;
  apiUrl: string;
  secureCookies: boolean;
  sessionKey: Buffer;
  keyFile: string;
  github: { clientId: string; clientSecret: string; authorizeUrl: string; tokenUrl: string; userUrl: string };
}

export type ConfigResult = { ok: true; config: WebConfig } | { ok: false; problems: string[] };

const GH = { authorizeUrl: "https://github.com/login/oauth/authorize", tokenUrl: "https://github.com/login/oauth/access_token", userUrl: "https://api.github.com/user" };
const isLocalHost = (h: string) => h === "localhost" || h === "127.0.0.1" || h === "[::1]" || h.endsWith(".localhost");

export function loadConfig(e: Record<string, string | undefined> = process.env): ConfigResult {
  const problems: string[] = [];
  const raw = e.CLUSTERX_ENV ?? "production";
  const env = raw === "development" || raw === "test" ? raw : raw === "production" ? "production" : (problems.push("CLUSTERX_ENV must be production, development or test"), "production" as const);
  const prod = env === "production";

  let appOrigin = "";
  try {
    const u = new URL(e.CLUSTERX_APP_ORIGIN ?? "");
    if (u.origin === "null" || (u.pathname !== "/" && u.pathname !== "") || u.search || u.hash) throw new Error("not an origin");
    if (u.protocol !== "https:" && !(!prod && u.protocol === "http:" && isLocalHost(u.hostname))) problems.push("CLUSTERX_APP_ORIGIN must be https (http only for localhost outside production)");
    appOrigin = u.origin;
  } catch {
    problems.push("CLUSTERX_APP_ORIGIN must be a valid origin");
  }

  let apiUrl = "";
  try {
    const u = new URL(e.CLUSTERX_API_URL ?? "http://localhost:8080");
    if (u.protocol !== "https:" && u.protocol !== "http:") throw new Error("scheme");
    apiUrl = u.origin;
  } catch {
    problems.push("CLUSTERX_API_URL must be a valid http(s) URL");
  }

  const sessionKey = Buffer.from(e.CLUSTERX_WEB_SESSION_KEY ?? "", "base64");
  if (sessionKey.length < 32) problems.push("CLUSTERX_WEB_SESSION_KEY must be base64 of at least 32 bytes");
  const keyFile = e.CLUSTERX_BFF_KEY_FILE ?? "";
  if (!keyFile) problems.push("CLUSTERX_BFF_KEY_FILE is required");

  const clientId = e.CLUSTERX_GITHUB_CLIENT_ID ?? "";
  const clientSecret = e.CLUSTERX_GITHUB_CLIENT_SECRET ?? "";
  if (!clientId || !clientSecret) problems.push("CLUSTERX_GITHUB_CLIENT_ID and CLUSTERX_GITHUB_CLIENT_SECRET are required");

  const overrides = [e.CLUSTERX_GITHUB_AUTHORIZE_URL, e.CLUSTERX_GITHUB_TOKEN_URL, e.CLUSTERX_GITHUB_USER_URL];
  if (prod && overrides.some(Boolean)) problems.push("OAuth endpoint overrides are not allowed in production");
  const github = {
    clientId,
    clientSecret,
    authorizeUrl: (!prod && e.CLUSTERX_GITHUB_AUTHORIZE_URL) || GH.authorizeUrl,
    tokenUrl: (!prod && e.CLUSTERX_GITHUB_TOKEN_URL) || GH.tokenUrl,
    userUrl: (!prod && e.CLUSTERX_GITHUB_USER_URL) || GH.userUrl,
  };

  if (problems.length) return { ok: false, problems };
  return { ok: true, config: { env, appOrigin, apiUrl, secureCookies: appOrigin.startsWith("https:"), sessionKey, keyFile, github } };
}

let cached: ConfigResult | undefined;
/** Process-wide config; read once so a late env change cannot flip a running server open. */
export function config(): ConfigResult {
  return (cached ??= loadConfig());
}
export function resetConfigForTests(): void {
  cached = undefined;
}
