import type { ReactNode } from "react";
import { PublicFooter, PublicHeader } from "@clusterx/ui";

export default function PublicLayout({ children }: { children: ReactNode }) {
  return (
    <>
      {/* Public pages are static and carry no session material, so the header cannot know who is signed in and always offers Sign in. */}
      <PublicHeader />
      <main id="main" tabIndex={-1}>{children}</main>
      <PublicFooter />
    </>
  );
}
