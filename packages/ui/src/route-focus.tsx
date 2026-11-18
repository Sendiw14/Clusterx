"use client";
import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";

/** After a client-side route change: focus moves to `main` (tabindex -1) and scroll resets (M-ROUTE, a11y plan). */
export function RouteFocus() {
  const pathname = usePathname();
  const first = useRef(true);
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    const main = document.getElementById("main");
    if (main) {
      main.focus({ preventScroll: true });
      window.scrollTo(0, 0);
    }
  }, [pathname]);
  return null;
}
