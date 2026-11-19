import { createCipheriv, createDecipheriv, createHash, createHmac, randomBytes, timingSafeEqual } from "node:crypto";

const sub = (key: Buffer, label: string) => createHmac("sha256", key).update(label).digest();
/** 1-byte key version: first byte of SHA-256(master). Lets `open` pick the right key during a rotation window (RV6-07). */
const keyVersion = (key: Buffer) => createHash("sha256").update(key).digest()[0]!;

/**
 * AES-256-GCM seal. Layout: key version (1 byte) | IV (12) | tag (16) | ciphertext. `purpose` is bound as AAD so an
 * OAuth-state cookie can never be replayed as a session cookie. `key` is the current key; sealing never uses an old one.
 */
export function seal(key: Buffer, purpose: string, value: object): string {
  const iv = randomBytes(12);
  const c = createCipheriv("aes-256-gcm", sub(key, "cookie-enc-v1"), iv);
  c.setAAD(Buffer.from(purpose));
  const ct = Buffer.concat([c.update(JSON.stringify(value), "utf8"), c.final()]);
  return Buffer.concat([Buffer.from([keyVersion(key)]), iv, c.getAuthTag(), ct]).toString("base64url");
}

/** `keys` is the keyring, current first; a key is tried only when its version byte matches the value's. */
export function open<T>(keys: Buffer | Buffer[], purpose: string, token: string | undefined): T | null {
  if (!token || token.length > 4096) return null;
  const b = Buffer.from(token, "base64url");
  if (b.length < 30) return null;
  for (const key of Array.isArray(keys) ? keys : [keys]) {
    if (keyVersion(key) !== b[0]) continue;
    try {
      const d = createDecipheriv("aes-256-gcm", sub(key, "cookie-enc-v1"), b.subarray(1, 13));
      d.setAAD(Buffer.from(purpose));
      d.setAuthTag(b.subarray(13, 29));
      return JSON.parse(Buffer.concat([d.update(b.subarray(29)), d.final()]).toString("utf8")) as T;
    } catch {
      /* wrong key (version collision) or tampered: try the next key */
    }
  }
  return null;
}

/** ADR-0006 section 3: X-CSRF-Token = HMAC-SHA256(csrf_key, sid), base64url. csrf_key is derived from the session key. */
export const csrfToken = (key: Buffer, sid: string): string => createHmac("sha256", sub(key, "csrf-v1")).update(sid).digest("base64url");

export function safeEqual(a: string, b: string): boolean {
  const x = Buffer.from(a);
  const y = Buffer.from(b);
  return x.length === y.length && timingSafeEqual(x, y);
}

export const randomId = (bytes = 16): string => randomBytes(bytes).toString("base64url");
export const requestId = (): string => `req_${randomBytes(6).toString("hex")}`;
