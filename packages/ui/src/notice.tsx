import type { ReactNode } from "react";
import { Icon, type IconName } from "./icon";
import { cx } from "./util";

export type NoticeVariant = "info" | "success" | "warning" | "error";
const ICON: Record<NoticeVariant, IconName> = { info: "i-info", success: "i-check", warning: "i-alert", error: "i-alert" };
const CLASS: Record<NoticeVariant, string> = { info: "info", success: "ok", warning: "warn", error: "err" };

/**
 * Inline banner. Lead phrase stands without color. `role="status"` for non-blocking, `role="alert"` for blocking
 * errors; never both. Pass `role="none"` to render without a live role (static page content).
 */
export function Notice({
  variant = "info",
  lead,
  children,
  actions,
  role,
  icon,
  className,
  id,
}: {
  variant?: NoticeVariant;
  lead: string;
  children?: ReactNode;
  actions?: ReactNode;
  role?: "status" | "alert" | "none";
  icon?: IconName;
  className?: string;
  id?: string;
}) {
  const r = role ?? (variant === "error" ? "alert" : "status");
  return (
    <div id={id} className={cx("notice", CLASS[variant], className)} role={r === "none" ? undefined : r}>
      <Icon name={icon ?? ICON[variant]} />
      <div className="grow">
        <strong>{lead}</strong> {children}
        {actions ? <div className="actions">{actions}</div> : null}
      </div>
    </div>
  );
}
