import { PillDemo } from "@/features/demo/pill";
import { DemoTour } from "@/features/demo/tour";
import { FIXTURE_LABEL } from "@/data/fixtures/data";

export default function DemoPage() {
  return (
    <div className="section-page">
      <div className="wrap-wide" style={{ display: "grid", gap: "var(--space-5)", gridTemplateColumns: "minmax(0, 1fr)" }}>
        <section className="demo-banner" aria-label="Demo notice">
          <PillDemo />
          <p><strong>Illustrative demo: seeded fixture data, read-only, not live.</strong> Everything here runs in your browser from bundled sample records. It makes no request to a running system and has no controls that change anything. The real engine runs locally with Docker Compose.</p>
        </section>
        <div>
          <h1 className="page-title">Console tour</h1>
          <p className="lede" style={{ marginTop: "var(--space-3)" }}>The same views the console is built from, on sample data: overview, jobs (including a failed job and the retry that was created from it), workers, and a step-by-step replay of a worker failure. {FIXTURE_LABEL}.</p>
        </div>
        <DemoTour />
      </div>
    </div>
  );
}
