import type { Metadata } from "next";
import type { ReactNode } from "react";
import { PublicFooter, PublicHeader } from "@clusterx/ui";

export const metadata: Metadata = { title: "Demo", description: "An isolated, read-only tour of the CLUSTERX console on seeded fixture data." };

export default function DemoLayout({ children }: { children: ReactNode }) {
  return (
    <>
      <PublicHeader />
      <main id="main" tabIndex={-1}>{children}</main>
      <PublicFooter />
    </>
  );
}
