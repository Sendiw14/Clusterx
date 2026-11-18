import type { Metadata } from "next";
import { DocPage, PrevNext } from "@/features/docs";

export const metadata: Metadata = { title: "API reference: Docs" };

export default function Page() {
  return (
    <DocPage title="API reference" lead="The versioned REST contract under /api/v1." pending="A reference generated from the frozen OpenAPI document, with per-operation examples, is produced with the documentation milestone. This page is deliberately not hand-written.">
      <h2 id="errors">Error model</h2>
      <p>Every error has the same shape: <code>code</code>, <code>message</code>, optional <code>details</code> and a <code>request_id</code>. Quote the request ID when reporting a problem.</p>
      <h2 id="status">Status codes</h2>
      <p>The API answers 400, 401, 403, 404, 409, 413, 422, 429, 500 and 503 with that error shape. A 410 means a job expired by retention, which is different from not found.</p>
      <h2 id="auth">Authentication</h2>
      <p>Direct clients use scoped, revocable API tokens. Tokens never go in browser code. The console uses a server-side session instead.</p>
      <PrevNext href="/docs/api" />
    </DocPage>
  );
}
