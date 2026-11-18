"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { Button, ButtonLink } from "./button";
import { Dialog } from "./dialog";
import { Icon } from "./icon";
import { Wordmark } from "./layout";
import { cx } from "./util";

export const PUBLIC_NAV = [
  { href: "/architecture", label: "Architecture" },
  { href: "/docs", label: "Docs" },
  { href: "/changelog", label: "Changelog" },
  { href: "/demo", label: "Demo" },
] as const;

export const isCurrent = (pathname: string, href: string): boolean => pathname === href || (href !== "/" && pathname.startsWith(`${href}/`));

/** Brand header: wordmark, primary nav, Sign in. Below 768 px a Menu button opens a modal drawer with the same items. */
export function PublicHeader({ signedIn = false }: { signedIn?: boolean }) {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  useEffect(() => setOpen(false), [pathname]);
  const cta = signedIn ? { href: "/app", label: "Open console" } : { href: "/login", label: "Sign in" };
  return (
    <header className="pub-h">
      <div className="wrap in">
        <Wordmark />
        <nav className="pub-nav" aria-label="Primary">
          {PUBLIC_NAV.map((n) => (
            <Link key={n.href} href={n.href} aria-current={isCurrent(pathname, n.href) ? "page" : undefined}>{n.label}</Link>
          ))}
          <ButtonLink href={cta.href} size="sm" className="pub-cta" aria-current={isCurrent(pathname, cta.href) ? "page" : undefined}>{cta.label}</ButtonLink>
        </nav>
        <Button size="sm" className="pub-menu-btn" icon="i-menu" aria-haspopup="dialog" aria-expanded={open} onClick={() => setOpen(true)}>Menu</Button>
      </div>
      <Dialog open={open} onClose={() => setOpen(false)} title="Site menu" variant="drawer" hideTitle>
        <nav className="drawer-nav" aria-label="Site menu">
          {PUBLIC_NAV.map((n) => (
            <Link key={n.href} href={n.href} aria-current={isCurrent(pathname, n.href) ? "page" : undefined}>{n.label}</Link>
          ))}
          <Link href={cta.href}>{cta.label}</Link>
        </nav>
      </Dialog>
    </header>
  );
}

const DOC_PAGES = [
  { href: "/docs/quickstart", label: "Quickstart" },
  { href: "/docs/concepts", label: "Concepts" },
  { href: "/docs/api", label: "API reference" },
  { href: "/docs/workers", label: "Worker SDK" },
  { href: "/docs/reliability", label: "Reliability" },
  { href: "/docs/deployment", label: "Deployment" },
] as const;
const GITHUB_URL = "https://github.com/Sendiw14/Clusterx";

/** Footer: four link groups, release status (never a fabricated version) and the UTC rule. */
export function PublicFooter() {
  return (
    <footer className="pub-f">
      <div className="wrap">
        <div className="cols">
          <div>
            <Wordmark />
            <p className="muted" style={{ marginTop: "var(--space-3)", maxWidth: "20rem" }}>Self-hosted distributed job processing. At-least-once, fenced, inspectable.</p>
          </div>
          <nav aria-label="Product">
            <h2>Product</h2>
            <ul>
              <li><Link href="/architecture">Architecture</Link></li>
              <li><Link href="/demo">Demo</Link></li>
              <li><Link href="/changelog">Changelog</Link></li>
              <li><Link href="/login">Sign in</Link></li>
            </ul>
          </nav>
          <nav aria-label="Documentation">
            <h2>Docs</h2>
            <ul>
              <li><Link href="/docs">Overview</Link></li>
              {DOC_PAGES.map((d) => <li key={d.href}><Link href={d.href}>{d.label}</Link></li>)}
            </ul>
          </nav>
          <nav aria-label="Project">
            <h2>Project</h2>
            <ul>
              <li><Link href="/docs/security">Security</Link></li>
              <li>
                <a href={GITHUB_URL} rel="noopener noreferrer">GitHub <Icon name="i-ext" /><span className="sr-only">(external)</span></a>
              </li>
              <li><span>License: Apache-2.0</span></li>
            </ul>
          </nav>
        </div>
        <div className="fine">
          <span>Release status: not yet released. No version has shipped.</span>
          <span>Times are shown in UTC unless a time zone is set.</span>
        </div>
      </div>
    </footer>
  );
}

export function DocsSidebar() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  useEffect(() => setOpen(false), [pathname]);
  const links = (
    <ul className="nav" style={{ listStyle: "none" }}>
      <li><Link href="/docs" aria-current={pathname === "/docs" ? "page" : undefined}>Overview</Link></li>
      {[...DOC_PAGES, { href: "/docs/security", label: "Security" }].map((d) => (
        <li key={d.href}><Link href={d.href} aria-current={pathname === d.href ? "page" : undefined}>{d.label}</Link></li>
      ))}
    </ul>
  );
  return (
    <>
      <div className="docs-menu-btn">
        <Button size="sm" icon="i-menu" aria-haspopup="dialog" aria-expanded={open} onClick={() => setOpen(true)}>Docs menu</Button>
      </div>
      <nav className={cx("docs-side")} aria-label="Documentation">{links}</nav>
      <Dialog open={open} onClose={() => setOpen(false)} title="Docs menu" variant="drawer">
        <nav aria-label="Documentation menu">{links}</nav>
      </Dialog>
    </>
  );
}
