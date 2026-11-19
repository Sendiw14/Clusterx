"use client";
import { useId, useMemo, useState } from "react";
import { DataTable, Segmented, Timestamp, type Column } from "@clusterx/ui";

export interface ChartPoint { t: string; v: number | null }
export interface ChartSeries { label: string; points: ChartPoint[]; color: string }

const W = 640;
const H = 200;
const PAD = { l: 44, r: 12, t: 10, b: 24 };

const fmt = (v: number, unit: string) => (unit === "s" ? (v < 90 ? `${v.toFixed(v < 10 ? 1 : 0)} s` : `${Math.round(v / 60)} min`) : String(Math.round(v)));

/** Break the line at null buckets: null means "no samples", never zero (GAP-OBS-02). */
function segments(points: ChartPoint[], x: (i: number) => number, y: (v: number) => number): string[] {
  const out: string[] = [];
  let cur = "";
  points.forEach((p, i) => {
    if (p.v === null) {
      if (cur) out.push(cur);
      cur = "";
    } else cur += `${cur ? "L" : "M"}${x(i).toFixed(1)} ${y(p.v).toFixed(1)}`;
  });
  if (cur) out.push(cur);
  return out;
}

/**
 * Line chart with its data table (WCAG: the table is the accessible equivalent; the SVG is `role=img` with a text
 * summary). Series share the time axis. `stale` dims the chart (S-STALE). No animation.
 */
export function SeriesChart({ title, unit = "", series, stale, stepLabel }: { title: string; unit?: string; series: ChartSeries[]; stale?: boolean; stepLabel: string }) {
  const [view, setView] = useState("chart");
  const id = useId();
  const n = series[0]?.points.length ?? 0;
  const max = useMemo(() => Math.max(1, ...series.flatMap((s) => s.points.map((p) => p.v ?? 0))), [series]);
  const x = (i: number) => PAD.l + (n <= 1 ? 0 : (i / (n - 1)) * (W - PAD.l - PAD.r));
  const y = (v: number) => PAD.t + (1 - v / max) * (H - PAD.t - PAD.b);
  const total = series.map((s) => `${s.label}: ${fmt(s.points.reduce((a, p) => a + (p.v ?? 0), 0), unit)}${unit === "s" ? " summed" : " total"}`).join("; ");
  const summary = n === 0 ? `${title}: no data in this window.` : `${title}, ${n} ${stepLabel} buckets. ${total}. Peak ${fmt(max, unit)}. The data table below has every value.`;

  const rows = useMemo(() => (series[0]?.points ?? []).map((p, i) => ({ t: p.t, vals: series.map((s) => s.points[i]?.v ?? null) })), [series]);
  const columns: Column<(typeof rows)[number]>[] = [
    { key: "t", header: "Bucket start", primary: true, render: (r) => <Timestamp value={r.t} /> },
    ...series.map((s, si) => ({ key: s.label, header: s.label, numeric: true, render: (r: (typeof rows)[number]) => (r.vals[si] === null ? <span className="meta">no samples</span> : fmt(r.vals[si] as number, unit)) })),
  ];

  return (
    <figure className={`chart${stale ? " is-stale" : ""}`} aria-labelledby={`${id}-t`} style={{ margin: 0 }}>
      <div className="toolbar" style={{ padding: "var(--space-3) var(--space-4)", justifyContent: "space-between" }}>
        <figcaption id={`${id}-t`} style={{ fontWeight: 600 }}>{title}</figcaption>
        <Segmented label={`${title} view`} value={view} onChange={setView} options={[{ value: "chart", label: "Chart" }, { value: "table", label: "Table" }]} />
      </div>
      {view === "chart" ? (
        <>
          <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={summary} data-testid="series-chart">
            <g className="grid axis">
              {[0, 0.5, 1].map((f) => (
                <g key={f}>
                  <line x1={PAD.l} x2={W - PAD.r} y1={y(max * f)} y2={y(max * f)} />
                  <text x={PAD.l - 6} y={y(max * f) + 4} textAnchor="end">{fmt(max * f, unit)}</text>
                </g>
              ))}
            </g>
            {series.map((s) => segments(s.points, x, y).map((d, k) => <path key={`${s.label}${k}`} d={d} fill="none" stroke={s.color} strokeWidth={2} />))}
            {series.map((s) => (n === 1 ? s.points.map((p, i) => (p.v === null ? null : <circle key={`${s.label}${i}`} cx={x(i)} cy={y(p.v)} r={3} fill={s.color} />)) : null))}
          </svg>
          <ul className="chart-legend" aria-label="Series">
            {series.map((s) => (<li key={s.label}><svg width="18" height="6" aria-hidden="true"><line x1="0" x2="18" y1="3" y2="3" stroke={s.color} strokeWidth="3" /></svg>{s.label}</li>))}
          </ul>
        </>
      ) : (
        <div style={{ maxHeight: "22rem", overflow: "auto" }}>
          <DataTable caption={`${title}: every ${stepLabel} bucket`} columns={columns} rows={rows} rowKey={(r) => r.t} />
        </div>
      )}
    </figure>
  );
}
