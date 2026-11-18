"use client";

/** Last-resort boundary (root layout failed). Self-contained: no shared components, no stack trace. */
export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <html lang="en">
      <body style={{ fontFamily: "system-ui, sans-serif", margin: 0, padding: "4rem 1rem", maxWidth: "40rem", marginInline: "auto" }}>
        <main id="main">
          <h1>Something went wrong</h1>
          <p>The page failed to render. Nothing was changed.{error.digest ? <> Reference <code>{error.digest}</code>.</> : null}</p>
          <p><button type="button" onClick={reset}>Retry</button> {/* eslint-disable-next-line @next/next/no-html-link-for-pages -- last-resort boundary: no router available */}
          <a href="/">Home</a></p>
        </main>
      </body>
    </html>
  );
}
