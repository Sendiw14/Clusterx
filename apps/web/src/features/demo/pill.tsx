import { LiveIndicator } from "@clusterx/ui";

/** Fixed "Demo data" pill: replaces the live indicator in the demo. Nothing in the demo is labelled live. */
export function PillDemo() {
  return <LiveIndicator state="demo" />;
}
