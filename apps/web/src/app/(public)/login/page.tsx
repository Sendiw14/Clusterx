import type { Metadata } from "next";
import Link from "next/link";
import { Icon, Notice, type NoticeVariant } from "@clusterx/ui";
import "@/features/login.css";

export const metadata: Metadata = { title: "Sign in", robots: { index: false, follow: false } };

interface Outcome { variant: NoticeVariant; lead: string; text: string; icon?: "i-lock" | "i-alert" | "i-info"; role: "status" | "alert" }
/** Closed set from ADR-0006 section 3. Anything else renders nothing (never echoes the query). */
const ERRORS: Record<string, Outcome> = {
  auth_error: { variant: "error", lead: "Sign-in did not complete.", text: "GitHub declined the request, or the sign-in link expired. Start again from this page.", icon: "i-alert", role: "alert" },
  no_entitlement: { variant: "error", lead: "This GitHub account isn't approved.", text: "Access is by approval: an operator adds your GitHub account to the allowlist. Signing in again will not change this.", icon: "i-lock", role: "alert" },
  user_disabled: { variant: "error", lead: "This account has been disabled.", text: "Ask an operator to re-enable it. No retry is offered because it would fail the same way.", icon: "i-lock", role: "alert" },
  provider_unavailable: { variant: "warning", lead: "GitHub is not responding.", text: "Try again in a minute. Your access is unaffected.", icon: "i-alert", role: "alert" },
  upstream_auth: { variant: "error", lead: "The console could not reach its own API.", text: "This is an infrastructure fault, not a problem with your account, so you are not asked to sign in again.", icon: "i-alert", role: "alert" },
};
const REASONS: Record<string, Outcome> = {
  signed_out: { variant: "info", lead: "You've been signed out.", text: "Your session was ended on this device.", icon: "i-info", role: "status" },
  session_expired: { variant: "warning", lead: "Your session ended.", text: "Sign in to continue where you left off.", icon: "i-alert", role: "status" },
};

export default async function LoginPage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const sp = await searchParams;
  const pick = (v: string | string[] | undefined) => (typeof v === "string" ? v : undefined);
  const err = pick(sp.error);
  const reason = pick(sp.reason);
  const outcome = (err && Object.hasOwn(ERRORS, err) ? ERRORS[err] : undefined) ?? (reason && Object.hasOwn(REASONS, reason) ? REASONS[reason] : undefined);
  const disabled = err === "user_disabled" || err === "no_entitlement";
  // Clamped again by the BFF; here it only decides whether to carry the parameter.
  const nextRaw = pick(sp.next);
  const next = nextRaw && nextRaw.startsWith("/app") && nextRaw.length <= 512 ? nextRaw : undefined;

  return (
    <div className="login">
      <div className="login-card">
        <p className="eyebrow">Console access</p>
        <h1>Sign in to the console</h1>
        {outcome ? <div className="panel"><Notice variant={outcome.variant} lead={outcome.lead} role={outcome.role} icon={outcome.icon}>{outcome.text}</Notice></div> : null}
        {!disabled ? <p className="muted">Sign in with the GitHub account an operator approved. CLUSTERX has no passwords and no shared accounts.</p> : null}
        {!disabled ? (
          <div style={{ marginTop: "var(--space-5)" }}>
            {/* A plain link: the BFF sets the state/PKCE cookie and redirects to GitHub. No script needed. */}
            <a className="btn btn-primary gh-btn" href={`/auth/github${next ? `?next=${encodeURIComponent(next)}` : ""}`}>Sign in with GitHub</a>
            <p className="help" style={{ marginTop: "var(--space-2)" }}><Icon name="i-info" /> You are sent to GitHub and back. CLUSTERX never sees your GitHub password.</p>
          </div>
        ) : null}
        <ul className="fine-list">
          <li>Read-only <strong>viewer</strong> and <strong>operator</strong> roles are assigned by an operator, not chosen at sign-in.</li>
          <li>Just looking around? <Link href="/demo">Open the demo</Link>. It uses fixture data and never touches a running system.</li>
          <li><Link href="/docs/security">How access and roles work</Link></li>
        </ul>
      </div>
    </div>
  );
}
