/**
 * Reads a request body of at most `max` bytes (RV6-02). A `Content-Length` over the cap is refused before anything is
 * read; otherwise the stream is counted while it is read and cancelled the moment it passes the cap (covers chunked
 * bodies and a lying Content-Length). Returns null when the body is too large.
 */
export async function readCapped(req: { headers: Headers; body: ReadableStream<Uint8Array> | null }, max: number): Promise<Uint8Array | null> {
  const declared = req.headers.get("content-length");
  if (declared !== null && (!/^\d+$/.test(declared) || Number(declared) > max)) {
    void req.body?.cancel().catch(() => {});
    return null;
  }
  if (!req.body) return new Uint8Array(0);
  const reader = req.body.getReader();
  const chunks: Uint8Array[] = [];
  let n = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    n += value.byteLength;
    if (n > max) {
      void reader.cancel().catch(() => {});
      return null;
    }
    chunks.push(value);
  }
  const out = new Uint8Array(n);
  let o = 0;
  for (const c of chunks) { out.set(c, o); o += c.byteLength; }
  return out;
}
