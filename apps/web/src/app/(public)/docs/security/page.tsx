import type { Metadata } from "next";
import { DocPage, PrevNext } from "@/features/docs";

export const metadata: Metadata = { title: "Security: Docs" };

export default function Page() {
  return (
    <DocPage title="Security" lead="Authentication, roles, worker trust and the demo boundary." pending="The threat-model summary and reporting policy are added with the security review. This page makes no compliance claims.">
      <h2 id="humans">Human sign-in</h2>
      <p>Sign-in is by GitHub OAuth with a server-held session. Access is restricted to an explicit allowlist, and each person is either an <strong>operator</strong> or a read-only <strong>viewer</strong>.</p>
      <h2 id="roles">Roles</h2>
      <p>Operators can submit, cancel and retry jobs, drain workers and manage credentials. Viewers are read-only and see sensitive fields masked. Authorization is enforced by the control plane on every request.</p>
      <h2 id="demo">The public demo</h2>
      <p>The demo runs entirely on bundled fixture data in your browser. It makes no request to a running system and has no write routes.</p>
      <PrevNext href="/docs/security" />
    </DocPage>
  );
}
