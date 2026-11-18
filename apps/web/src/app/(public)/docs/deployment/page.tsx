import type { Metadata } from "next";
import { DocPage, PrevNext } from "@/features/docs";

export const metadata: Metadata = { title: "Deployment: Docs" };

export default function Page() {
  return (
    <DocPage title="Deployment" lead="A single-VM reference deployment." pending="TLS, secrets, networking, backups, restore and upgrade steps are documented after they have been exercised. Recovery point and recovery time numbers are not yet measured.">
      <h2 id="shape">Shape</h2>
      <p>One Linux VM runs the control plane, PostgreSQL, the web app and workers behind a TLS reverse proxy. Local demonstration and production configuration are kept separate.</p>
      <PrevNext href="/docs/deployment" />
    </DocPage>
  );
}
