import { SPRITE_DEFS, type IconName } from "./sprite-data";

export type { IconName };

/** Renders the shared SVG sprite once per document (root layout). Static trusted markup authored in this repo. */
export function IconSprite() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="0"
      height="0"
      style={{ position: "absolute" }}
      aria-hidden="true"
      focusable="false"
      dangerouslySetInnerHTML={{ __html: `<defs>${SPRITE_DEFS}</defs>` }}
    />
  );
}

/** Decorative by default (aria-hidden). Pass `label` only for an icon that carries meaning alone. */
export function Icon({ name, size = "sm", label, className }: { name: IconName; size?: "sm" | "md"; label?: string; className?: string }) {
  return (
    <svg
      className={`icon${size === "md" ? " icon-md" : ""}${className ? ` ${className}` : ""}`}
      aria-hidden={label ? undefined : true}
      role={label ? "img" : undefined}
      aria-label={label}
      focusable="false"
    >
      <use href={`#${name}`} />
    </svg>
  );
}
