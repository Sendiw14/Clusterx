"use client";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Icon } from "@clusterx/ui";

interface Entry { u: string; p: string; h: string; t: string }
interface Hit extends Entry { score: number }

let indexPromise: Promise<Entry[]> | null = null;
function loadIndex(): Promise<Entry[]> {
  indexPromise ??= fetch("/docs/search-index.json").then((r) => { if (!r.ok) throw new Error(String(r.status)); return r.json() as Promise<Entry[]>; }).catch((e) => { indexPromise = null; throw e; });
  return indexPromise;
}

const words = (s: string) => s.toLowerCase().split(/[^a-z0-9_]+/).filter(Boolean);

/** Every query word must prefix-match a word in the entry. Heading hits outweigh page-title and body hits. */
export function search(index: Entry[], query: string, limit = 6): Hit[] {
  const q = words(query);
  if (!q.length) return [];
  const hits: Hit[] = [];
  for (const e of index) {
    const hw = words(e.h), pw = words(e.p), bw = words(e.t);
    let score = 0;
    let ok = true;
    for (const w of q) {
      const inH = hw.some((x) => x.startsWith(w)), inP = pw.some((x) => x.startsWith(w)), inB = bw.some((x) => x.startsWith(w));
      if (!inH && !inP && !inB) { ok = false; break; }
      score += (inH ? 10 : 0) + (inP ? 3 : 0) + (inB ? 1 : 0) + (hw.includes(w) ? 4 : 0);
    }
    if (ok) hits.push({ ...e, score });
  }
  return hits.sort((a, b) => b.score - a.score || a.h.length - b.h.length).slice(0, limit);
}

type Load = "idle" | "loading" | "ready" | "error";

/**
 * Client-side docs search over the static build-time index. ARIA combobox with a listbox: Arrow keys move,
 * Enter opens, Esc clears. "/" focuses the field from anywhere on a docs page (when it is visible).
 */
export function DocsSearch() {
  const router = useRouter();
  const uid = useId();
  const listId = `${uid}-list`;
  const input = useRef<HTMLInputElement>(null);
  const [q, setQ] = useState("");
  const [load, setLoad] = useState<Load>("idle");
  const [index, setIndex] = useState<Entry[]>([]);
  const [active, setActive] = useState(0);
  const [open, setOpen] = useState(false);

  const ensure = () => {
    if (load === "ready" || load === "loading") return;
    setLoad("loading");
    loadIndex().then((i) => { setIndex(i); setLoad("ready"); }).catch(() => setLoad("error"));
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement | null;
      if (e.key !== "/" || e.ctrlKey || e.metaKey || e.altKey) return;
      if (el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT" || el.isContentEditable)) return;
      const i = input.current;
      if (!i || i.getClientRects().length === 0) return;
      e.preventDefault();
      i.focus();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  const hits = useMemo(() => (load === "ready" ? search(index, q) : []), [index, q, load]);
  const showList = open && q.trim().length > 0;
  const go = (h: Hit) => { setOpen(false); router.push(h.u); };

  let status = "";
  if (q.trim()) status = load === "loading" ? "Loading search index" : load === "error" ? "Search is unavailable" : hits.length ? `${hits.length} result${hits.length === 1 ? "" : "s"}` : "No results";

  return (
    <div className="dsearch" role="search">
      <label htmlFor={`${uid}-in`} className="sr-only">Search the documentation</label>
      <div className="dsearch-field">
        <Icon name="i-search" />
        <input
          ref={input}
          id={`${uid}-in`}
          type="search"
          role="combobox"
          aria-expanded={showList && hits.length > 0}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={showList && hits[active] ? `${uid}-o${active}` : undefined}
          autoComplete="off"
          spellCheck={false}
          placeholder="Search docs"
          value={q}
          onFocus={() => { ensure(); setOpen(true); }}
          onChange={(e) => { setQ(e.target.value); setActive(0); setOpen(true); ensure(); }}
          onBlur={(e) => { if (!e.relatedTarget || !(e.currentTarget.closest(".dsearch")?.contains(e.relatedTarget as Node))) setOpen(false); }}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") { e.preventDefault(); setActive((a) => Math.min(a + 1, Math.max(hits.length - 1, 0))); }
            else if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)); }
            else if (e.key === "Enter" && hits[active] && showList) { e.preventDefault(); go(hits[active]!); }
            else if (e.key === "Escape") { if (q) { e.preventDefault(); e.stopPropagation(); setQ(""); } setOpen(false); }
          }}
        />
        <kbd aria-hidden="true">/</kbd>
      </div>
      <p className="sr-only" role="status">{status}</p>
      {showList ? (
        <div className="dsearch-pop">
          {hits.length ? (
            <ul id={listId} role="listbox" aria-label="Search results">
              {hits.map((h, i) => (
                <li key={h.u + i} id={`${uid}-o${i}`} role="option" aria-selected={i === active} onMouseEnter={() => setActive(i)} onMouseDown={(e) => e.preventDefault()} onClick={() => go(h)}>
                  <span className="ph">{h.p}</span>
                  <span className="hh">{h.h}</span>
                  <span className="tt">{h.t}</span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="dsearch-msg" id={listId}>
              {load === "loading" ? "Loading the search index..." : load === "error" ? <>Search index could not be loaded. <button type="button" className="link-btn" onClick={() => { setLoad("idle"); ensure(); }}>Retry</button></> : <>No results for &quot;{q}&quot;. Try a shorter word, such as <em>lease</em> or <em>retry</em>.</>}
            </p>
          )}
        </div>
      ) : null}
    </div>
  );
}
