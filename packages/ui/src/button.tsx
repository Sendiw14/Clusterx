"use client";
import Link from "next/link";
import type { AnchorHTMLAttributes, ButtonHTMLAttributes, MouseEvent, ReactNode } from "react";
import { cx } from "./util";
import { Icon, type IconName } from "./icon";

type Variant = "default" | "primary" | "danger" | "danger-solid" | "ghost";
type Size = "sm" | "md" | "lg";

const variantClass: Record<Variant, string> = {
  default: "",
  primary: "btn-primary",
  danger: "btn-danger",
  "danger-solid": "btn-danger-solid",
  ghost: "btn-ghost",
};
const sizeClass: Record<Size, string> = { sm: "btn-sm", md: "", lg: "btn-lg" };

export function btnClass(variant: Variant = "default", size: Size = "md", extra?: string): string {
  return cx("btn", variantClass[variant], sizeClass[size], extra);
}

export interface ButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "disabled"> {
  variant?: Variant;
  size?: Size;
  icon?: IconName;
  /** S-PENDING: spinner, `data-pending`, `aria-disabled`, repeat clicks ignored. Change `children` to say what is happening. */
  pending?: boolean;
  /**
   * Temporarily unavailable. Rendered as `aria-disabled` (stays focusable). Callers MUST show the reason as visible
   * text next to the button (component-specs section 0, rule 6).
   */
  unavailable?: boolean;
}

export function Button({ variant, size, icon, pending, unavailable, onClick, children, className, type = "button", ...rest }: ButtonProps) {
  const blocked = Boolean(pending || unavailable);
  return (
    <button
      {...rest}
      type={type}
      className={btnClass(variant, size, className)}
      aria-disabled={blocked || undefined}
      data-pending={pending ? "true" : undefined}
      onClick={(e: MouseEvent<HTMLButtonElement>) => {
        if (blocked) {
          e.preventDefault();
          return;
        }
        onClick?.(e);
      }}
    >
      {icon ? <Icon name={icon} /> : null}
      <span className="spin" aria-hidden="true" />
      {children}
    </button>
  );
}

export interface ButtonLinkProps extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href"> {
  href: string;
  variant?: Variant;
  size?: Size;
  icon?: IconName;
  children: ReactNode;
  /** Same-origin routes use next/link; set for GitHub and other external targets (adds the external glyph and text). */
  external?: boolean;
}

export function ButtonLink({ href, variant, size, icon, external, children, className, ...rest }: ButtonLinkProps) {
  const cls = btnClass(variant, size, className);
  if (external) {
    return (
      <a {...rest} href={href} className={cls} rel="noopener noreferrer">
        {icon ? <Icon name={icon} /> : null}
        {children}
        <Icon name="i-ext" />
        <span className="sr-only">(external)</span>
      </a>
    );
  }
  return (
    <Link {...rest} href={href} className={cls}>
      {icon ? <Icon name={icon} /> : null}
      {children}
    </Link>
  );
}

export function IconButton({ label, icon, ...rest }: Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label"> & { label: string; icon: IconName }) {
  return (
    <button type="button" {...rest} aria-label={label} className={cx("icon-btn", rest.className)}>
      <Icon name={icon} />
    </button>
  );
}
