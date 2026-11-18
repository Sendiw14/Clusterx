import { useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from "react";
import { cx } from "./util";
import { Icon } from "./icon";

interface FieldShell {
  label: string;
  help?: ReactNode;
  error?: string;
  required?: boolean;
}

function Shell({ id, label, help, error, required, children }: FieldShell & { id: string; children: ReactNode }) {
  return (
    <div className="field">
      <label htmlFor={id}>
        {label}
        {required ? <span className="meta"> (required)</span> : null}
      </label>
      {children}
      {help ? <span className="help" id={`${id}-help`}>{help}</span> : null}
      {error ? (
        <span className="field-error" id={`${id}-err`}>
          <Icon name="i-alert" /> {error}
        </span>
      ) : null}
    </div>
  );
}

const describe = (id: string, help?: ReactNode, error?: string) => [help ? `${id}-help` : null, error ? `${id}-err` : null].filter(Boolean).join(" ") || undefined;

export function TextInput({ label, help, error, required, className, ...rest }: FieldShell & InputHTMLAttributes<HTMLInputElement>) {
  const id = useId();
  return (
    <Shell id={id} label={label} help={help} error={error} required={required}>
      <input {...rest} id={id} required={required} className={cx("input", className)} aria-invalid={error ? true : undefined} aria-describedby={describe(id, help, error)} />
    </Shell>
  );
}

export function Select({ label, help, error, required, className, children, ...rest }: FieldShell & SelectHTMLAttributes<HTMLSelectElement>) {
  const id = useId();
  return (
    <Shell id={id} label={label} help={help} error={error} required={required}>
      <select {...rest} id={id} required={required} className={cx("select", className)} aria-invalid={error ? true : undefined} aria-describedby={describe(id, help, error)}>
        {children}
      </select>
    </Shell>
  );
}

export function Textarea({ label, help, error, required, className, ...rest }: FieldShell & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const id = useId();
  return (
    <Shell id={id} label={label} help={help} error={error} required={required}>
      <textarea {...rest} id={id} required={required} className={cx("input textarea", className)} aria-invalid={error ? true : undefined} aria-describedby={describe(id, help, error)} />
    </Shell>
  );
}

/** Removable filter chip: text plus a 24 px remove button. Chips are never the only way to set a filter. */
export function FilterChip({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <span className="chip">
      {label}
      <button type="button" aria-label={`Remove ${label} filter`} onClick={onRemove}>
        <Icon name="i-x" />
      </button>
    </span>
  );
}

/** Toggle group for view switches that are not navigation (Chart/Table, Graph/List): `aria-pressed`, not tabs. */
export function Segmented({ label, options, value, onChange }: { label: string; options: { value: string; label: string }[]; value: string; onChange: (v: string) => void }) {
  return (
    <span className="seg" role="group" aria-label={label}>
      {options.map((o) => (
        <button key={o.value} type="button" aria-pressed={value === o.value} onClick={() => onChange(o.value)}>
          {o.label}
        </button>
      ))}
    </span>
  );
}
