"use client";
import { useEffect, useState } from "react";
import type { Heading } from "@/lib/docs/markdown";

/**
 * "On this page". `rail` (desktop, sticky) highlights the section in view with aria-current="location";
 * `inline` is a closed details element for narrow screens. Plain links work without script.
 */
export function OnThisPage({ headings, variant }: { headings: Heading[]; variant: "rail" | "inline" }) {
  const [active, setActive] = useState<string | null>(null);
  useEffect(() => {
    if (variant !== "rail" || !headings.length || typeof IntersectionObserver === "undefined") return;
    const els = headings.map((h) => document.getElementById(h.id)).filter((e): e is HTMLElement => !!e);
    const io = new IntersectionObserver((entries) => {
      const vis = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
      if (vis) setActive(vis.target.id);
    }, { rootMargin: "-60px 0px -60% 0px" });
    els.forEach((e) => io.observe(e));
    return () => io.disconnect();
  }, [headings, variant]);

  if (!headings.length) return null;
  const list = (
    <ul>
      {headings.map((h) => (
        <li key={h.id} data-depth={h.depth}>
          <a href={`#${h.id}`} aria-current={active === h.id ? "location" : undefined}>{h.text}</a>
        </li>
      ))}
    </ul>
  );
  if (variant === "inline") {
    return <details className="otp-details"><summary>On this page</summary><nav aria-label="On this page (inline)">{list}</nav></details>;
  }
  return <nav className="otp" aria-label="On this page"><h2>On this page</h2>{list}</nav>;
}
