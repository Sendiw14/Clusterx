"use client";
import { useEffect, useRef, type CSSProperties, type ElementType, type ReactNode } from "react";
import { cx } from "./util";

/**
 * M-SECTION: opacity 0 to 1 and translateY 8 to 0 px over 260 ms when 20 percent visible. Plays once; never replays
 * on scroll up. Content is complete without JS (`.js` is set by the root layout script) and under reduced motion the
 * CSS shows the final state at mount. Elements already in the viewport are shown immediately.
 */
export function Reveal({ as, children, className, style }: { as?: ElementType; children: ReactNode; className?: string; style?: CSSProperties }) {
  const Tag = as ?? "div";
  const ref = useRef<HTMLElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (typeof IntersectionObserver === "undefined") { el.classList.add("in"); return; }
    const io = new IntersectionObserver((entries) => {
      for (const e of entries) if (e.isIntersecting) { e.target.classList.add("in"); io.disconnect(); }
    }, { threshold: 0.2 });
    io.observe(el);
    return () => io.disconnect();
  }, []);
  return <Tag ref={ref} className={cx("reveal", className)} style={style}>{children}</Tag>;
}
