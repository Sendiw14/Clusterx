"use client";
import { Card, DefinitionList } from "@clusterx/ui";
import { useDataProvider } from "@/data/context";
import { Load } from "./common";
import { useResource } from "./use-resource";

/** Read-only operational configuration straight from `GET /config`. */
export function ConfigView() {
  const provider = useDataProvider();
  const res = useResource(() => provider.getConfig(), `config-${provider.kind}`);
  return (
    <Load res={res} rows={4} label="Loading configuration" errorHeading="Configuration could not be loaded">
      {(c) => (
        <div style={{ display: "grid", gap: "var(--space-4)" }}>
          <Card>
            <div className="card-h"><h2>Environment</h2></div>
            <div className="card-pad"><DefinitionList items={[
              { term: "Environment", value: c.environment },
              { term: "API version", value: <span className="mono">{c.api_version}</span> },
              { term: "Contract", value: <span className="mono">{c.contract_version}</span> },
              { term: "Build", value: <span className="mono">{c.build.version} ({c.build.commit})</span> },
            ]} /></div>
          </Card>
          <Card>
            <div className="card-h"><h2>Limits</h2></div>
            <div className="card-pad"><DefinitionList items={[
              { term: "Payload", value: `${c.limits.payload_max_bytes / 1024} KiB maximum` },
              { term: "Attempts", value: `${c.limits.max_attempts_min} to ${c.limits.max_attempts_max} (default ${c.limits.max_attempts_default})` },
              { term: "Priority", value: `${c.limits.priority_min} to ${c.limits.priority_max} (9 is highest)` },
              { term: "Jobs list window", value: `${c.limits.jobs_list_window_max_days} days maximum` },
              { term: "Page size", value: `${c.limits.page_size_default} default, ${c.limits.page_size_max} maximum` },
            ]} /></div>
          </Card>
          <Card>
            <div className="card-h"><h2>Retention</h2></div>
            <div className="card-pad"><DefinitionList items={[
              { term: "Terminal jobs", value: `${c.retention.terminal_jobs_days} days` },
              { term: "Attempt logs", value: `${c.retention.attempt_logs_days} days` },
              { term: "Audit", value: `${c.retention.audit_days} days` },
            ]} /></div>
          </Card>
          <Card>
            <div className="card-h"><h2>Timing</h2></div>
            <div className="card-pad"><DefinitionList items={[
              { term: "Heartbeat", value: `${c.timing.heartbeat_interval_seconds} s` },
              { term: "Liveness timeout", value: `${c.timing.liveness_timeout_seconds} s` },
              { term: "Lease TTL", value: `${c.timing.lease_ttl_seconds} s` },
            ]} /></div>
          </Card>
          <p className="meta">Read once when the page opened.</p>
        </div>
      )}
    </Load>
  );
}
