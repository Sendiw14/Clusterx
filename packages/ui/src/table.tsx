import { useId, type ReactNode } from "react";
import { cx } from "./util";

export interface Column<T> {
  key: string;
  header: string;
  numeric?: boolean;
  /** The primary cell (ID plus status) is shown first in stacked card mode, without a label. */
  primary?: boolean;
  render: (row: T) => ReactNode;
}

/**
 * Native table with caption and `th scope=col`. Below 640 px it becomes labelled stacked cards (same fields, same
 * order, nothing hidden) through `data-label` attributes (styles.css).
 */
export function DataTable<T>({ caption, columns, rows, rowKey, busy, className }: { caption: ReactNode; columns: Column<T>[]; rows: T[]; rowKey: (r: T) => string; busy?: boolean; className?: string }) {
  const capId = useId();
  return (
    <div className="tbl-wrap" role="region" tabIndex={0} aria-labelledby={capId} aria-busy={busy || undefined} style={busy ? { opacity: 0.6 } : undefined}>
      <table className={cx("tbl", className)}>
        <caption id={capId}>{caption}</caption>
        <thead>
          <tr>
            {columns.map((c) => (
              <th key={c.key} scope="col" className={c.numeric ? "num" : undefined}>{c.header}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={rowKey(r)}>
              {columns.map((c) => (
                <td key={c.key} className={c.numeric ? "num" : undefined} data-label={c.primary ? "" : c.header} data-primary={c.primary ? "" : undefined}>
                  {c.render(r)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Cursor pagination: Previous/Next only (cursors are opaque), disabled with a visible reason, page size stated. */
export function Pagination({ hasPrev, hasNext, onPrev, onNext, note }: { hasPrev: boolean; hasNext: boolean; onPrev: () => void; onNext: () => void; note: ReactNode }) {
  return (
    <nav className="pager" aria-label="Pagination">
      <span className="meta">{note}</span>
      <span className="toolbar">
        <button type="button" className="btn btn-sm" aria-disabled={!hasPrev || undefined} onClick={() => hasPrev && onPrev()}>
          Previous {hasPrev ? null : <span className="meta">(first page)</span>}
        </button>
        <button type="button" className="btn btn-sm" aria-disabled={!hasNext || undefined} onClick={() => hasNext && onNext()}>
          Next {hasNext ? null : <span className="meta">(no more)</span>}
        </button>
      </span>
    </nav>
  );
}
