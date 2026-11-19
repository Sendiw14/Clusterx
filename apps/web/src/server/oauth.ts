import * as client from "openid-client";
import type { WebConfig } from "./config.ts";

/**
 * GitHub OAuth authorization code + PKCE (S256) + state with `openid-client` (a maintained OAuth 2 library; GitHub is
 * not OIDC so the server metadata is explicit). Endpoints come from config: non-production tests point them at a local
 * mock server (plain http then needs `allowInsecureRequests`, applied only when the endpoint really is http).
 */
function oauthConfig(c: WebConfig): client.Configuration {
  const g = c.github;
  const cfg = new client.Configuration({ issuer: new URL(g.authorizeUrl).origin, authorization_endpoint: g.authorizeUrl, token_endpoint: g.tokenUrl }, g.clientId, g.clientSecret, client.ClientSecretPost(g.clientSecret));
  if (new URL(g.tokenUrl).protocol === "http:" && c.env !== "production") client.allowInsecureRequests(cfg);
  cfg.timeout = 10;
  return cfg;
}

export async function beginAuthorization(c: WebConfig): Promise<{ url: string; state: string; verifier: string }> {
  const verifier = client.randomPKCECodeVerifier();
  const state = client.randomState();
  const url = client.buildAuthorizationUrl(oauthConfig(c), {
    redirect_uri: `${c.appOrigin}/auth/callback`,
    scope: "read:user",
    state,
    code_challenge: await client.calculatePKCECodeChallenge(verifier),
    code_challenge_method: "S256",
  });
  return { url: url.href, state, verifier };
}

export type Identity = { ok: true; id: number; login: string } | { ok: false; reason: "auth_error" | "provider_unavailable" };

/** Code exchange plus the GitHub user lookup. `auth_error` = denied/invalid (user can retry), `provider_unavailable` = GitHub down. */
export async function completeAuthorization(c: WebConfig, callbackUrl: URL, expect: { state: string; verifier: string }): Promise<Identity> {
  const cfg = oauthConfig(c);
  let token: string;
  try {
    // Rebuild the URL on the configured origin: behind a proxy the request's own host is not trustworthy.
    const current = new URL(`${c.appOrigin}${callbackUrl.pathname}${callbackUrl.search}`);
    const t = await client.authorizationCodeGrant(cfg, current, { pkceCodeVerifier: expect.verifier, expectedState: expect.state }, { redirect_uri: `${c.appOrigin}/auth/callback` });
    token = t.access_token;
  } catch (e) {
    const rejected = e instanceof client.ResponseBodyError || e instanceof client.AuthorizationResponseError || e instanceof client.ClientError;
    return { ok: false, reason: rejected ? "auth_error" : "provider_unavailable" };
  }
  try {
    const r = await client.fetchProtectedResource(cfg, token, new URL(c.github.userUrl), "GET", undefined, new Headers({ Accept: "application/json" }));
    if (r.status >= 500) return { ok: false, reason: "provider_unavailable" };
    if (!r.ok) return { ok: false, reason: "auth_error" };
    const u = (await r.json()) as { id?: unknown; login?: unknown };
    if (typeof u.id !== "number" || !Number.isSafeInteger(u.id) || u.id < 1 || typeof u.login !== "string" || u.login.length < 1 || u.login.length > 39) return { ok: false, reason: "auth_error" };
    return { ok: true, id: u.id, login: u.login };
  } catch {
    return { ok: false, reason: "provider_unavailable" };
  }
}
