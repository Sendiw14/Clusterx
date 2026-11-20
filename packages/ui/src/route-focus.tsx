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
    // A closing dialog (menu drawer) restores focus to its trigger first; main must win after that. Wait until no modal dialog is
    // open (bounded: 60 frames), then two more frames. A fixed two-frame wait lost the race on a slow Firefox run.
    let raf = 0;
    let waited = 0;
    let settled = 0;
    const tick = () => {
      if (document.querySelector("dialog[open]") && waited++ < 60) {
        raf = requestAnimationFrame(tick);
        return;
      }
      if (++settled < 2) {
        raf = requestAnimationFrame(tick);
        return;
      }
      const main = document.getElementById("main");
      if (main) {
        main.focus({ preventScroll: true });
        window.scrollTo(0, 0);
      }
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [pathname]);
  return null;
}
