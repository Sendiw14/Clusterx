/**
 * Provenance of the console capture on the home page (`public/images/console-jobs.png`). The image is a real screenshot of
 * `/app/jobs` from the live E2E stack: own PostgreSQL 18 cluster, the real control plane and a real worker running the
 * three example handlers, signed in through the real OAuth flow against a local mock. Regenerate with
 * `CAPTURE_M6=1 pnpm exec playwright test e2e/screenshots-m6.spec.ts` and update this record.
 */
export const CONSOLE_CAPTURE = {
  file: "/images/console-jobs.png",
  route: "/app/jobs",
  date: "2026-11-19",
  /** Commit whose tree the capture was taken from (short SHA); the only later change is the screenshot spec itself. */
  commit: "91e27a1",
  width: 952,
  height: 495,
} as const;
