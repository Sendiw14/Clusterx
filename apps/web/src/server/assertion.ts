import { readFileSync } from "node:fs";
import { importJWK, SignJWT, type CryptoKey, type JWK } from "jose";
import { randomId } from "./crypto.ts";

/** Closed claim set from ADR-0006 section 7: iss, aud, sub, sid, role, purpose, iat, nbf, exp, jti, rid. Nothing else. */
export type AssertionSpec =
  | { login: true; githubId: number; rid: string }
  | { login?: false; userId: string; sid: string; role: "operator" | "viewer"; rid: string };

let loaded: { path: string; kid: string; key: CryptoKey | Uint8Array } | undefined;

async function signingKey(path: string) {
  if (loaded?.path === path) return loaded;
  const jwk = JSON.parse(readFileSync(path, "utf8")) as JWK;
  if (jwk.kty !== "OKP" || jwk.crv !== "Ed25519" || !jwk.d || !jwk.kid) throw new Error("BFF key file must be an Ed25519 private JWK with kid");
  loaded = { path, kid: jwk.kid, key: await importJWK(jwk, "EdDSA") };
  return loaded;
}

/** One fresh assertion per upstream request (fresh 128-bit jti, 60 s lifetime). Never logged, never sent to the browser. */
export async function mintAssertion(keyFile: string, spec: AssertionSpec): Promise<string> {
  const { kid, key } = await signingKey(keyFile);
  const base = spec.login ? { sub: `github:${spec.githubId}`, purpose: "login" } : { sub: `user:${spec.userId}`, sid: spec.sid, role: spec.role };
  return new SignJWT({ ...base, rid: spec.rid })
    .setProtectedHeader({ alg: "EdDSA", typ: "clusterx-assertion+jwt", kid })
    .setIssuer("clusterx-bff")
    .setAudience("clusterx-control-plane")
    .setIssuedAt()
    .setNotBefore("0s")
    .setExpirationTime("60s")
    .setJti(randomId(16))
    .sign(key);
}
