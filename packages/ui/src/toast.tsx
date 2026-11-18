"use client";
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

interface ToastApi {
  /** Supplementary confirmation only: never the sole signal of an outcome, never a secret. */
  show: (message: string) => void;
}
const Ctx = createContext<ToastApi>({ show: () => {} });
export const useToast = (): ToastApi => useContext(Ctx);

/** At most one toast at a time; visible 4 s plus 1 s per 20 words; paused while hovered or focused. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [msg, setMsg] = useState<string | null>(null);
  const paused = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const remaining = useRef(0);

  const clear = useCallback(() => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = null;
  }, []);
  const arm = useCallback(
    (ms: number) => {
      clear();
      remaining.current = ms;
      timer.current = setTimeout(() => {
        if (paused.current) return;
        setMsg(null);
      }, ms);
    },
    [clear],
  );
  const show = useCallback(
    (m: string) => {
      setMsg(m);
      arm(4000 + Math.ceil(m.split(/\s+/).length / 20) * 1000);
    },
    [arm],
  );
  useEffect(() => clear, [clear]);
  const api = useMemo(() => ({ show }), [show]);

  return (
    <Ctx.Provider value={api}>
      {children}
      <div
        className="toast-region"
        role="status"
        onMouseEnter={() => { paused.current = true; clear(); }}
        onMouseLeave={() => { paused.current = false; arm(1500); }}
        onFocus={() => { paused.current = true; clear(); }}
        onBlur={() => { paused.current = false; arm(1500); }}
      >
        {msg ? <div className="toast-live">{msg}</div> : null}
      </div>
    </Ctx.Provider>
  );
}
