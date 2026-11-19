import { createCipheriv, createDecipheriv, createHmac, randomBytes, timingSafeEqual } from "node:crypto";

const sub = (key: Buffer, label: string) => createHmac("sha256", key).update(label).digest();

/** AES-256-GCM seal. `purpose` is bound as AAD so an OAuth-state cookie can never be replayed as a session cookie. */
export function seal(key: Buffer, purpose: string, value: object): string {
  const iv = randomBytes(12);
  const c = createCipheriv("aes-256-gcm", sub(key, "cookie-enc-v1"), iv);
  c.setAAD(Buffer.from(purpose));
  const ct = Buffer.concat([c.update(JSON.stringify(value), "utf8"), c.final()]);
  return Buffer.concat([iv, c.getAuthTag(), ct]).toString("base64url");
}

export function open<T>(key: Buffer, purpose: string, token: string | undefined): T | null {
  if (!token || token.length > 4096) return null;
  try {
    const b = Buffer.from(token, "base64url");
    if (b.length < 29) return null;
    const d = createDecipheriv("aes-256-gcm", sub(key, "cookie-enc-v1"), b.subarray(0, 12));
    d.setAAD(Buffer.from(purpose));
    d.setAuthTag(b.subarray(12, 28));
    return JSON.parse(Buffer.concat([d.update(b.subarray(28)), d.final()]).toString("utf8")) as T;
  } catch {
    return null;
  }
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
