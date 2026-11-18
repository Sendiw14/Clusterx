import type { ReactNode } from "react";
import Link from "next/link";
import { Icon } from "./icon";
import { cx } from "./util";

export function SkipLink({ href = "#main" }: { href?: string }) {
  return <a className="skip" href={href}>Skip to content</a>;
}

export function Wordmark({ href = "/", inDrawer }: { href?: string; inDrawer?: boolean }) {
  return (
    <Link className="wordmark" href={href} aria-label={inDrawer ? undefined : "CLUSTERX home"}>
      <Icon name="mark" />
      CLUSTERX
    </Link>
  );
}

export function Eyebrow({ children }: { children: ReactNode }) {
  return <p className="eyebrow">{children}</p>;
}

export function Card({ children, className, pad }: { children: ReactNode; className?: string; pad?: boolean }) {
  return <div className={cx("card", pad && "card-pad", className)}>{children}</div>;
}

export function PageHeader({ title, lead, actions }: { title: string; lead?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="page-h">
      <div>
        <h1>{title}</h1>
        {lead ? <p>{lead}</p> : null}
      </div>
      {actions ? <div className="toolbar">{actions}</div> : null}
    </div>
  );
}

export function DefinitionList({ items }: { items: { term: string; value: ReactNode }[] }) {
  return (
    <dl className="dl">
      {items.map((i) => (
        <div key={i.term} style={{ display: "contents" }}>
          <dt>{i.term}</dt>
          <dd>{i.value}</dd>
        </div>
      ))}
    </dl>
  );
}

/** KPI row as a list: value, label and note per item. Values change instantly (no count-up animation). */
export function KpiList({ items, label }: { items: { label: string; value: ReactNode; note?: ReactNode; icon?: ReactNode }[]; label: string }) {
  return (
    <ul className="kpis" aria-label={label}>
      {items.map((k) => (
        <li key={k.label}>
          <span className="kpi-v">{k.value}</span>
          <span className="kpi-l">{k.icon}{k.label}</span>
          {k.note ? <span className="kpi-n">{k.note}</span> : null}
        </li>
      ))}
    </ul>
  );
}
