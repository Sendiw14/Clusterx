# Web browser matrix and live E2E results (AUDIT-M9 F-08, MASTER §10)

Date: 2026-11-20. Host: Apple M4, macOS (Darwin 27.0, arm64), Node 26.13.1, Playwright 1.65.1, PostgreSQL 18.7, Go 1.27.4.
Suite: `apps/web/e2e/**` against the production build (`next build`, `node server.ts`) and the live stack (real Go control plane, two real workers, PostgreSQL, **mock OAuth**). One project per invocation (each starts its own stack), run with `pnpm --filter web e2e:<project>`; `pnpm --filter web e2e` runs the three in order.

## Engines tested

| Project | Engine build (Playwright's own, headless) | Result of the final run | Exit |
|---|---|---|---|
| `chromium` | Chromium 159.0.8154.6 (Chrome for Testing, `chromium-1255`) | 213 passed, 58 skipped | 0 |
| `firefox` | Firefox 159.0 (`firefox-1572`) | 213 passed, 58 skipped | 0 |
| `webkit` | WebKit 27.2 (`webkit-2388`) | 213 passed, 58 skipped | 0 |

The 58 skipped are opt-in specs: screenshot capture (54) and the console performance measurements (4). Axe scans: 116 per browser, **0 serious or critical** in each (`docs/design/evidence/axe-m6.json` for Chromium, `axe-firefox.json` and `axe-webkit.json` here). All three are one run each of the final source (`pnpm --filter web lint`, `typecheck`, `test` 127 passed and `build` also exit 0 in that run). Per-engine logs live only in the session scratchpad (ephemeral); the counts above are the record. Earlier runs on the way had failures that are listed below; there was no retry setting (`retries: 0`) and no test was skipped to get green.

**Not tested: branded Google Chrome, Microsoft Edge, Safari on macOS or iOS, any Android or iOS device, any Linux or Windows host.** Playwright's WebKit is not Safari; passing here does not show Safari behaviour. Supported-browser statement for 1.0 is therefore: engines exercised are Chromium, Firefox and WebKit (Playwright builds, desktop viewport plus the 320 to 1920 px width checks); everything else is untested.

## What failed on the first Firefox and WebKit runs, and what it was

| Failure | Browser | Cause | Fix |
|---|---|---|---|
| Clipboard specs: `Unknown permission: clipboard-read` | Firefox | Test setup: Playwright grants clipboard permissions only in Chromium | `clipboardContext`/`readClipboard` in `e2e/helpers.ts`: plain context elsewhere, text read back through a real keyboard paste. The app's Copy button worked |
| 320 px overflow check, `scrollWidth 344 and 347 > 320` on `/app/jobs/<id>`, twice in about six runs | Firefox | **A real responsive bug.** The event timeline's `li`, `p` and `span.meta` hold the actor and data values, which contain UUIDs. A hyphen followed by a digit is not a line-break opportunity, so a UUID with the wrong digits does not wrap and widens the page. It depends on random ids, hence the intermittency. Chromium and WebKit did not show it | `packages/ui/src/styles.css`: `.timeline li{min-width:0;overflow-wrap:anywhere}`. 6 of 6 passes afterwards (not a proof for a 1-in-3 flake), and the helper now reports the route and the elements past the edge |
| `allTextContents` on links of an open modal dialog: page crashed | Firefox | **Firefox/driver crash, not the app.** Reproduces on a bare page with one `<dialog>`: a role locator `waitFor`, then a read of a role locator inside the modal dialog, kills the content process. Normal user interaction and plain evaluation do not | Test reads the links with a page-level CSS locator |
| Menu, drawer and Docs-menu specs: trigger not focused after Esc | WebKit | **Platform behaviour.** Safari/WebKit does not focus a button on a mouse click, so a mouse-opened dialog has nothing to return focus to. Keyboard activation returns focus (probed) | Specs open the menu by keyboard (`activate` helper). The requirement is for keyboard and screen-reader users |
| Skip link not focused after Tab | WebKit | **Platform default.** Safari leaves links out of the Tab order unless Option is held | Spec uses Alt+Tab on WebKit. Worth knowing for users: Safari users reach the skip link with Option+Tab |
| `axe` serious `color-contrast` with the menu open (5 nodes) | WebKit, once | Scanned in the middle of the dialog's fade-in (opacity not final). Other specs already wait for the transition | `animationsDone(dlg)` before the scan in `behaviors.spec.ts` |
| Console error `.../app/workers?_rsc=... due to access control checks` | WebKit | A Next link prefetch cancelled by a full page navigation; WebKit reports the cancelled fetch as an unhandled rejection. Request status is `cancelled` | `watchConsole` ignores exactly that message shape |

## Flakes and hardening

1. **Firefox, focus moves to `main` after a client route change (mobile menu spec).** It failed in 2 of about 90 runs under load (the route-focus code waited a fixed two animation frames for the closing dialog's focus restore, and a slow frame sequence flipped the order). `packages/ui/src/route-focus.tsx` now waits until no modal `<dialog>` is open (at most 60 frames) and then two frames. 36 of 36 menu specs passed under load afterwards, and the final run is green.
2. **Chromium, `console-session-hardening` `/auth/end?reason=signed_out`: `page.goto` failed with `net::ERR_ABORTED`**, 4 of about 70 runs under load. **Cause not found.** The test now waits for `networkidle` and retries the navigation once on that error; its assertions are on the end state (cookie cleared, session revoked), which does not depend on which attempt gets there. A product-level race is not excluded.

## Notes

1. Real GitHub sign-in was not used anywhere: every console run signs in through the mock OAuth server on loopback HTTP.
2. Manual screen-reader, keyboard-only, zoom and forced-colors walkthroughs (AUDIT F-07) are **NOT RUN**: they need a human tester.
3. Console budget measurements ran in Chromium and, where noted in `console-budgets.md`, Firefox and WebKit.
