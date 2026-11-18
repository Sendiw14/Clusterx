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
    // Two frames: a closing dialog (menu drawer) restores focus to its trigger first; main must win after that.
    let raf2 = 0;
    const raf1 = requestAnimationFrame(() => {
      raf2 = requestAnimationFrame(() => {
        const main = document.getElementById("main");
        if (main) {
          main.focus({ preventScroll: true });
          window.scrollTo(0, 0);
        }
      });
    });
    return () => {
      cancelAnimationFrame(raf1);
      cancelAnimationFrame(raf2);
    };
  }, [pathname]);
  return null;
}
