import type { ReactNode } from "react";
import { PublicFooter, PublicHeader } from "@clusterx/ui";

export default function PublicLayout({ children }: { children: ReactNode }) {
  return (
    <>
      {/* No session implementation yet (M5): the header always offers Sign in. */}
      <PublicHeader />
      <main id="main" tabIndex={-1}>{children}</main>
      <PublicFooter />
    </>
  );
}
