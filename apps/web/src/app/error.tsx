"use client";
import { ButtonLink, ErrorState } from "@clusterx/ui";

/** GLOBAL-500 / segment boundary: no stack trace, digest as the correlation id, Retry and Home. */
export default function ErrorBoundary({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <main id="main" tabIndex={-1} className="section-page">
      <div className="wrap">
        <ErrorState error={{ code: "internal", status: 500, requestId: error.digest }} onRetry={reset} blocking extra={<ButtonLink href="/">Home</ButtonLink>} heading="Something went wrong" />
      </div>
    </main>
  );
}
