import type { Metadata } from "next";
import { DocPage, PrevNext } from "@/features/docs";

export const metadata: Metadata = { title: "Worker SDK: Docs" };

export default function Page() {
  return (
    <DocPage title="Worker SDK" lead="Register trusted Go handlers and report results." pending="Install steps and compiled examples are added when the SDK examples are published, so every snippet on this page is one that compiles.">
      <h2 id="trust">Trust model</h2>
      <p>Workers run trusted, precompiled handlers. Each worker has its own credential, issued by an operator. A revoked credential is refused on any new claim, renewal or result.</p>
      <h2 id="cancel">Cancellation is cooperative</h2>
      <p>A deadline or cancellation stops the engine from accepting a result. It does not stop a handler that ignores its context, so handlers must honor cancellation.</p>
      <PrevNext href="/docs/workers" />
    </DocPage>
  );
}
