"use client";
import { useId, useRef, useState, type KeyboardEvent, type ReactNode } from "react";

export interface TabItem { id: string; label: string; panel: ReactNode }

/** ARIA tabs with roving tabindex: Left/Right/Home/End move and activate. Panels stay in the DOM, inactive ones hidden. */
export function Tabs({ items, label, initial }: { items: TabItem[]; label: string; initial?: string }) {
  const base = useId();
  const [active, setActive] = useState(initial ?? items[0]?.id);
  const refs = useRef<Record<string, HTMLButtonElement | null>>({});

  function onKey(e: KeyboardEvent<HTMLDivElement>) {
    const i = items.findIndex((t) => t.id === active);
    let n = -1;
    if (e.key === "ArrowRight") n = (i + 1) % items.length;
    else if (e.key === "ArrowLeft") n = (i - 1 + items.length) % items.length;
    else if (e.key === "Home") n = 0;
    else if (e.key === "End") n = items.length - 1;
    if (n < 0) return;
    e.preventDefault();
    const t = items[n];
    if (!t) return;
    setActive(t.id);
    refs.current[t.id]?.focus();
  }

  return (
    <div className="tabs">
      <div role="tablist" aria-label={label} className="tablist" onKeyDown={onKey}>
        {items.map((t) => (
          <button
            key={t.id}
            ref={(el) => { refs.current[t.id] = el; }}
            role="tab"
            type="button"
            id={`${base}-t-${t.id}`}
            aria-selected={active === t.id}
            aria-controls={`${base}-p-${t.id}`}
            tabIndex={active === t.id ? 0 : -1}
            onClick={() => setActive(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>
      {items.map((t) => (
        <div key={t.id} role="tabpanel" id={`${base}-p-${t.id}`} aria-labelledby={`${base}-t-${t.id}`} hidden={active !== t.id} tabIndex={0}>
          {t.panel}
        </div>
      ))}
    </div>
  );
}
