"use client";
import { useEffect, useId, useRef, type ReactNode } from "react";
import { IconButton } from "./button";
import { cx } from "./util";

export interface DialogProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  /** Blocking dialogs (token reveal, session expired) ignore Esc and backdrop clicks. */
  blocking?: boolean;
  alert?: boolean;
  variant?: "dialog" | "drawer" | "cmd";
  /** Visible heading is hidden but kept for the accessible name (menus). */
  hideTitle?: boolean;
  className?: string;
}

/**
 * Native `<dialog>` via showModal(): focus trap, Esc, focus restored to the trigger by the platform.
 * Focus handling is synchronous; the animation is decoration (M-OVERLAY).
 */
export function Dialog({ open, onClose, title, description, children, footer, blocking, alert, variant = "dialog", hideTitle, className }: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const descId = useId();

  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className={cx(variant === "drawer" && "drawer", variant === "cmd" && "cmd", className)}
      role={alert ? "alertdialog" : undefined}
      aria-labelledby={titleId}
      aria-describedby={description ? descId : undefined}
      onCancel={(e) => {
        if (blocking) e.preventDefault();
      }}
      onClose={() => {
        if (open) onClose();
      }}
      onClick={(e) => {
        if (!blocking && e.target === ref.current) onClose();
      }}
    >
      <div className="dlg-h" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: "var(--space-3)" }}>
        <h2 id={titleId} className={hideTitle ? "sr-only" : undefined}>{title}</h2>
        {blocking ? null : <IconButton icon="i-x" label={`Close: ${title}`} onClick={onClose} />}
      </div>
      <div className="dlg-b">
        {description ? <p id={descId} className="muted">{description}</p> : null}
        {children}
      </div>
      {footer ? <div className="dlg-f">{footer}</div> : null}
    </dialog>
  );
}
