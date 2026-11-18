"use client";
import { useEffect, useRef, useState } from "react";
import { Button } from "./button";
import { cx } from "./util";

type CopyState = "idle" | "copied" | "blocked";

/** Clipboard copy with polite status. Blocked clipboard shows manual-copy guidance (component-specs 13). */
export function CopyButton({ text, label = "Copy", size = "sm", className, ariaLabel }: { text: string; label?: string; size?: "sm" | "md"; className?: string; ariaLabel?: string }) {
  const [state, setState] = useState<CopyState>("idle");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  async function copy() {
    let ok = false;
    try {
      await navigator.clipboard.writeText(text);
      ok = true;
    } catch {
      ok = false;
    }
    setState(ok ? "copied" : "blocked");
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setState("idle"), ok ? 1800 : 6000);
  }

  return (
    <span className={cx("copy", className)}>
      <Button size={size} icon={state === "copied" ? "i-check" : "i-copy"} onClick={copy} aria-label={ariaLabel}>
        {state === "copied" ? "Copied" : label}
      </Button>
      <span className="sr-only" role="status">{state === "copied" ? "Copied" : ""}</span>
      {state === "blocked" ? <span className="help" role="status"> Copy blocked. Select the text and copy manually.</span> : null}
    </span>
  );
}

/** Dark code block. Header with language label and Copy; the `pre` is focusable so keyboard users can scroll it. */
export function CodeBlock({ code, language, label, expectedOutput }: { code: string; language: string; label: string; expectedOutput?: boolean }) {
  return (
    <div className="code" data-expected-output={expectedOutput ? "true" : undefined}>
      <div className="code-h">
        <span>{expectedOutput ? "Expected output" : language}</span>
        {expectedOutput ? null : <CopyButton text={code} ariaLabel={`Copy: ${label}`} />}
      </div>
      <pre tabIndex={0} aria-label={label}>
        <code>{code}</code>
      </pre>
    </div>
  );
}
