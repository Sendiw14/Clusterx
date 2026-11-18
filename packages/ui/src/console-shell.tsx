"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useId, useState, type ReactNode } from "react";
import { Button } from "./button";
import { Dialog } from "./dialog";
import { Icon } from "./icon";
import { Wordmark } from "./layout";
import { LiveIndicator, type LiveState } from "./status";
import { ToastProvider } from "./toast";

export type Role = "operator" | "viewer" | null;

interface NavItem { href: string; label: string; operatorOnly?: boolean }
const MAIN: NavItem[] = [
  { href: "/app", label: "Overview" },
  { href: "/app/jobs", label: "Jobs" },
  { href: "/app/workers", label: "Workers" },
  { href: "/app/observability", label: "Observability" },
  { href: "/app/activity", label: "Activity" },
];
const SETTINGS: NavItem[] = [
  { href: "/app/settings", label: "Configuration" },
  { href: "/app/settings/tokens", label: "Tokens and credentials", operatorOnly: true },
  { href: "/app/settings/preferences", label: "Preferences" },
];

/** The UI hides what a role cannot use as a courtesy; Go is the authorization boundary. Unknown role hides operator-only items. */
function visible(items: NavItem[], role: Role): NavItem[] {
  return items.filter((i) => !i.operatorOnly || role === "operator");
}

function current(pathname: string, href: string): boolean {
  if (href === "/app") return pathname === "/app";
  if (href === "/app/settings") return pathname === "/app/settings";
  return pathname === href || pathname.startsWith(`${href}/`);
}

function NavLists({ role, pathname }: { role: Role; pathname: string }) {
  return (
    <nav aria-label="Console">
      <ul className="nav">
        {visible(MAIN, role).map((i) => (
          <li key={i.href}><Link href={i.href} aria-current={current(pathname, i.href) ? "page" : undefined}>{i.label}</Link></li>
        ))}
      </ul>
      <p className="nav-sep" aria-hidden="true">Settings</p>
      <ul className="nav">
        {visible(SETTINGS, role).map((i) => (
          <li key={i.href}><Link href={i.href} aria-current={current(pathname, i.href) ? "page" : undefined}>{i.label}</Link></li>
        ))}
      </ul>
    </nav>
  );
}

const TITLES: Record<string, string> = {
  app: "Console", jobs: "Jobs", workers: "Workers", observability: "Observability", activity: "Activity", settings: "Settings", tokens: "Tokens and credentials", preferences: "Preferences", new: "New job",
};

/** Console / Jobs / <short id>. Intermediate crumbs are links; the last is plain text with aria-current. */
export function Breadcrumbs({ pathname }: { pathname: string }) {
  const segs = pathname.split("/").filter(Boolean);
  const crumbs = segs.map((s, i) => {
    const href = "/" + segs.slice(0, i + 1).join("/");
    const isId = i >= 2 && segs[i - 1] !== "settings" && s !== "new";
    return { href, label: isId ? s.slice(0, 8) : (TITLES[s] ?? s), mono: isId };
  });
  if (crumbs.length === 1) crumbs[0] = { href: "/app", label: "Console", mono: false }, crumbs.push({ href: "/app", label: "Overview", mono: false });
  else if (crumbs[0]) crumbs[0] = { href: "/app", label: "Console", mono: false };
  return (
    <nav aria-label="Breadcrumb">
      <ol className="crumbs">
        {crumbs.map((c, i) => {
          const last = i === crumbs.length - 1;
          return (
            <li key={c.href + i}>
              {last ? <span aria-current="page" className={c.mono ? "mono" : undefined}>{c.label}</span> : <Link href={c.href}>{c.label}</Link>}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

export type LookupResult =
  | { kind: "job"; id: string }
  | { kind: "worker"; id: string }
  | { kind: "none" }
  | { kind: "expired" }
  | { kind: "error"; message: string };

const ID_RE = /^[0-9a-f]{8}(-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})?$/i;

/** Ctrl/Cmd+K dialog: page list plus exact-ID lookup. No free-text search exists in the API. */
function CommandDialog({ open, onClose, role, lookup }: { open: boolean; onClose: () => void; role: Role; lookup?: (id: string) => Promise<LookupResult> }) {
  const router = useRouter();
  const inputId = useId();
  const [q, setQ] = useState("");
  const [msg, setMsg] = useState("");
  const pages = [...visible(MAIN, role), ...visible(SETTINGS, role)].filter((p) => p.label.toLowerCase().includes(q.trim().toLowerCase()));
  const isId = ID_RE.test(q.trim());

  async function find() {
    if (!lookup) return setMsg("ID lookup needs a connected backend. It is not available in this build.");
    setMsg("Looking up…");
    const r = await lookup(q.trim());
    if (r.kind === "job") { onClose(); router.push(`/app/jobs/${r.id}`); }
    else if (r.kind === "worker") { onClose(); router.push(`/app/workers/${r.id}`); }
    else if (r.kind === "none") setMsg("No job or worker with that ID.");
    else if (r.kind === "expired") setMsg("That job expired by retention. It is no longer stored.");
    else setMsg(r.message);
  }

  return (
    <Dialog open={open} onClose={onClose} title="Jump to" variant="cmd" hideTitle>
      <div className="field">
        <label htmlFor={inputId}>Jump to a page, or enter an exact job or worker ID</label>
        <input id={inputId} className="input" type="text" autoComplete="off" spellCheck={false} placeholder="e.g. 3f9a21c4" value={q} onChange={(e) => { setQ(e.target.value); setMsg(""); }}
          onKeyDown={(e) => { if (e.key === "Enter" && isId) { e.preventDefault(); void find(); } }} />
      </div>
      <ul className="nav" style={{ listStyle: "none" }}>
        {pages.map((p) => (<li key={p.href}><Link href={p.href} onClick={onClose}>{p.label}</Link></li>))}
        {isId ? <li><button type="button" className="link-btn" onClick={() => void find()}>Look up job or worker {q.trim()}</button></li> : null}
      </ul>
      <p className="help" role="status">{msg || `${pages.length} page${pages.length === 1 ? "" : "s"} match${isId ? "; ID lookup available" : ""}`}</p>
      <p className="help">Search is by exact ID only: the API has no free-text search. Press Esc to close.</p>
    </Dialog>
  );
}

export interface ConsoleShellProps {
  role: Role;
  /** GitHub login when a session exists. */
  login?: string;
  live: LiveState;
  liveDetail?: string;
  /** Persistent, non-dismissible banner above the page (for example "backend not connected"). */
  banner?: ReactNode;
  lookup?: (id: string) => Promise<LookupResult>;
  /** Sign-out is rendered only when a handler exists (no session means nothing to sign out of). */
  onSignOut?: () => void;
  children: ReactNode;
}

export function ConsoleShell({ role, login, live, liveDetail, banner, lookup, onSignOut, children }: ConsoleShellProps) {
  const pathname = usePathname();
  const [drawer, setDrawer] = useState(false);
  const [cmd, setCmd] = useState(false);
  useEffect(() => setDrawer(false), [pathname]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") { e.preventDefault(); setCmd(true); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const foot = (
    <div className="side-foot">
      {login ? <span><strong>@{login}</strong> <span className="meta">{role ?? ""}</span></span> : <span className="meta">Not signed in</span>}
      {onSignOut ? <button type="button" className="link-btn" onClick={onSignOut}>Sign out</button> : null}
    </div>
  );

  return (
    <ToastProvider>
      <div className="shell">
        <aside className="side" aria-label="Console sidebar">
          <div className="side-in">
            <Wordmark href="/app" />
            <NavLists role={role} pathname={pathname} />
            {foot}
          </div>
        </aside>
        <div className="shell-main">
          <header className="top">
            <Button size="sm" className="menu-btn" icon="i-menu" aria-haspopup="dialog" aria-expanded={drawer} onClick={() => setDrawer(true)}>Menu</Button>
            <Breadcrumbs pathname={pathname} />
            <span className="grow" />
            {role === "viewer" ? <span className="role-banner"><Icon name="i-lock" />Read-only access (viewer)</span> : null}
            <button type="button" className="cmd-btn" aria-label="Jump to a job or worker by ID" aria-haspopup="dialog" onClick={() => setCmd(true)}>
              <Icon name="i-search" /><span className="long">Jump to ID</span><kbd>Ctrl K</kbd>
            </button>
            <LiveIndicator state={live} detail={liveDetail} />
          </header>
          {banner ? <div className="main-banner">{banner}</div> : null}
          <main id="main" className="main" tabIndex={-1}>{children}</main>
        </div>
      </div>
      <Dialog open={drawer} onClose={() => setDrawer(false)} title="Console menu" variant="drawer" hideTitle>
        <NavLists role={role} pathname={pathname} />
        {foot}
      </Dialog>
      <CommandDialog open={cmd} onClose={() => setCmd(false)} role={role} lookup={lookup} />
    </ToastProvider>
  );
}


