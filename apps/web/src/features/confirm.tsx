"use client";
import type { ReactNode } from "react";
import { Button, Dialog } from "@clusterx/ui";

/** Confirmation for destructive or hard-to-undo actions: names the object and the effect; primary button repeats the verb. */
export function ConfirmDialog({ open, title, verb, danger, pending, onConfirm, onCancel, error, children }: { open: boolean; title: string; verb: string; danger?: boolean; pending?: boolean; onConfirm: () => void; onCancel: () => void; error?: string | null; children: ReactNode }) {
  return (
    <Dialog
      open={open}
      onClose={() => { if (!pending) onCancel(); }}
      title={title}
      footer={
        <>
          <Button onClick={onCancel} unavailable={pending}>Cancel</Button>
          <Button variant={danger ? "danger-solid" : "primary"} pending={pending} onClick={onConfirm}>{pending ? `${verb}…` : verb}</Button>
        </>
      }
    >
      {children}
      {error ? <p className="field-error" role="alert">{error}</p> : null}
    </Dialog>
  );
}
