"use client";
import { useState } from "react";
import { Button, Card, DataTable, EmptyState, MaskedField, Notice, Pagination, Select, Timestamp, TextInput, type Column } from "@clusterx/ui";
import { useConsoleProvider, useRole } from "@/data/context";
import type { AuditEvent, AuditQuery } from "@/data/console-provider";
import { Load } from "./common";
import { useResource } from "./use-resource";

const ANY = () => true;
const WINDOWS = { "1h": 3_600_000, "24h": 86_400_000, "7d": 7 * 86_400_000, "30d": 30 * 86_400_000 } as const;
type Win = keyof typeof WINDOWS;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** `/audit`: category, subject and window filters only (GAP-AUDIT-01). Viewers see privileged events with details masked. */
export function ActivityView() {
  const p = useConsoleProvider();
  const role = useRole();
  const [category, setCategory] = useState<NonNullable<AuditQuery["category"]> | "">("");
  const [subjectType, setSubjectType] = useState<NonNullable<AuditQuery["subject_type"]> | "">("");
  const [subjectId, setSubjectId] = useState("");
  const [win, setWin] = useState<Win>("7d");
  const [stack, setStack] = useState<string[]>([]);
  const cursor = stack.at(-1);
  const idBad = subjectId !== "" && !UUID.test(subjectId);
  const reset = () => setStack([]);

  const res = useResource(() => {
    const to = new Date();
    return p.listAudit({ category: category || undefined, subject_type: subjectType || undefined, subject_id: !idBad && subjectId ? subjectId : undefined, from: new Date(to.getTime() - WINDOWS[win]).toISOString(), to: to.toISOString(), cursor, limit: 50 });
  }, JSON.stringify([category, subjectType, subjectId, win, cursor]), ANY);

  const columns: Column<AuditEvent>[] = [
    { key: "t", header: "Time", primary: true, render: (e) => <Timestamp value={e.occurred_at} /> },
    { key: "a", header: "Action", render: (e) => <span className="mono">{e.action}</span> },
    { key: "o", header: "Outcome", render: (e) => <span className="badge" data-outcome={e.outcome}>{e.outcome}</span> },
    { key: "c", header: "Category", render: (e) => e.category },
    { key: "actor", header: "Actor", render: (e) => <span className="mono">{e.actor}</span> },
    { key: "s", header: "Subject", render: (e) => (e.subject_type ? <span className="mono">{e.subject_type} {e.subject_id?.slice(0, 8) ?? ""}</span> : <span className="meta">none</span>) },
    { key: "r", header: "Request", render: (e) => (e.request_id ? <span className="mono">{e.request_id}</span> : <span className="meta">none</span>) },
    { key: "d", header: "Details", render: (e) => (e.masked_fields.includes("details") ? <MaskedField /> : e.details ? <code className="mono">{JSON.stringify(e.details)}</code> : <span className="meta">none</span>) },
  ];

  return (
    <>
      <Card className="fbar">
        <form className="filters" role="search" aria-label="Filter activity" onSubmit={(e) => e.preventDefault()}>
          <Select label="Category" value={category} onChange={(e) => { setCategory(e.target.value as typeof category); reset(); }}>
            <option value="">All visible</option>
            <option value="privileged">Privileged</option>
            {role === "operator" ? <option value="security">Security</option> : null}
            {role === "operator" ? <option value="diagnostic">Diagnostic</option> : null}
          </Select>
          <Select label="Subject type" value={subjectType} onChange={(e) => { setSubjectType(e.target.value as typeof subjectType); reset(); }}>
            <option value="">Any</option>
            {["job", "worker", "worker_credential", "api_token", "user", "session"].map((t) => <option key={t} value={t}>{t.replace("_", " ")}</option>)}
          </Select>
          <TextInput label="Subject ID" placeholder="full UUID" className="mono" value={subjectId} onChange={(e) => { setSubjectId(e.target.value.trim()); reset(); }} error={idBad ? "Enter a full UUID." : undefined} spellCheck={false} autoComplete="off" />
          <Select label="Window" value={win} onChange={(e) => { setWin(e.target.value as Win); reset(); }}>
            {(Object.keys(WINDOWS) as Win[]).map((w) => <option key={w} value={w}>Last {w}</option>)}
          </Select>
          {category || subjectType || subjectId ? <div className="field"><span className="label">&nbsp;</span><Button onClick={() => { setCategory(""); setSubjectType(""); setSubjectId(""); reset(); }}>Clear filters</Button></div> : null}
        </form>
      </Card>
      {role === "viewer" ? <div style={{ marginTop: "var(--space-3)" }}><Notice variant="info" lead="Viewer scope." role="none">You see privileged events only, with their details hidden. Security and diagnostic events are for operators.</Notice></div> : null}
      <section className="card sect" aria-label="Activity events" style={{ marginTop: "var(--space-4)" }}>
        <Load res={res} rows={6} label="Loading activity" errorHeading="Activity could not be loaded">
          {(page) => page.items.length === 0 ? (
            <EmptyState title="No events in this window">Nothing matched. Widen the window or clear a filter. Audit events are kept for the configured retention period.</EmptyState>
          ) : (
            <>
              <DataTable busy={res.refreshing} caption={<>Audit events, {page.items.length} shown · newest first · as of <Timestamp value={page.as_of} /></>} columns={columns} rows={page.items} rowKey={(e) => e.id} />
              <Pagination hasPrev={stack.length > 0} hasNext={Boolean(page.next_cursor)} onPrev={() => setStack((s) => s.slice(0, -1))} onNext={() => page.next_cursor && setStack((s) => [...s, page.next_cursor as string])} note="50 per page · newest first" />
            </>
          )}
        </Load>
      </section>
    </>
  );
}
