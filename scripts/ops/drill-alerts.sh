#!/usr/bin/env bash
# Alert drill on the isolated benchmark stack: start the monitoring profile, induce every alert of
# infra/monitoring/alerts.yml and record Prometheus' /api/v1/alerts every 15 s with timestamps.
#   scripts/ops/drill-alerts.sh <evidence-dir>
# Phase A (one worker killed mid-work + failing jobs): WorkerOffline, NoLiveWorkers, LeaseExpiriesRepeated,
#   FailureRateHigh, QueueAgeHigh, JobsUnschedulable.  Phase B: DiskSpaceLow (tmpfs volume filled),
#   DatabaseUnavailable (postgres stopped), ControlPlaneDown (control plane stopped). Never run on a stack you care about.
set -euo pipefail
EV=$(mkdir -p "$1" && cd "$1" && pwd)
export COMPOSE_FILE=compose.yaml:../../scripts/bench/compose.bench.yaml:../monitoring/drill/compose.disk-drill.yaml
. "$(dirname "$0")/../bench/env.sh"
API=http://127.0.0.1:$(envval CLUSTERX_HTTP_PORT); PROM=http://127.0.0.1:$(envval CLUSTERX_PROMETHEUS_PORT)
HDR=$(mktemp); trap 'rm -f "$HDR"; kill $REC 2>/dev/null || true' EXIT; chmod 600 "$HDR"
printf 'Authorization: Bearer %s\nContent-Type: application/json\n' "$(sed -n 's/^CLUSTERX_OPERATOR_TOKEN=//p' .secrets.env)" > "$HDR"
sub() { curl -sS -o /dev/null -H @"$HDR" -H "Idempotency-Key: alertdrill-$1-$(date +%s)" -d "$2" "$API/api/v1/jobs"; }
log() { echo "$(date -u +%FT%TZ) $*" | tee -a "$EV/timeline.txt"; }
firing() { curl -s "$PROM/api/v1/alerts" | python3 -c 'import json,sys; print(" ".join(sorted({a["labels"]["alertname"] for a in json.load(sys.stdin)["data"]["alerts"] if a["state"]=="firing"})))'; }
wait_firing() { # alertname, timeout s
  for _ in $(seq $(( $2 / 5 ))); do firing | grep -qw "$1" && { log "FIRING $1"; return 0; }; sleep 5; done
  log "TIMEOUT waiting for $1"; return 1
}
: > "$EV/timeline.txt"; : > "$EV/alerts-api.jsonl"
dc --profile workers stop "${ALL_WORKERS[@]}" >/dev/null 2>&1 || true
psqlq -c "SET session_replication_role = replica; TRUNCATE jobs, job_attempts, job_events, submission_idempotency, job_tombstones" >/dev/null
dc --profile monitoring up -d prometheus node-exporter 2>/dev/null
until curl -sf "$PROM/-/ready" >/dev/null; do sleep 1; done
( while :; do printf '{"t":"%s","alerts":%s}\n' "$(date -u +%FT%TZ)" "$(curl -s "$PROM/api/v1/alerts" | python3 -c 'import json,sys; print(json.dumps([{k:a[k] for k in ("labels","state","activeAt","value")} for a in json.load(sys.stdin)["data"]["alerts"]]))')"; sleep 15; done ) >> "$EV/alerts-api.jsonl" &
REC=$!
sleep 20
log "baseline firing: [$(firing)]"
curl -s "$PROM/api/v1/targets" | python3 -c 'import json,sys; [print(t["labels"]["job"], t["health"], t["scrapeUrl"]) for t in json.load(sys.stdin)["data"]["activeTargets"]]' | tee -a "$EV/timeline.txt"

log "phase A: start worker-1; submit 40 permanent failures + 30 x sleep_ms 60 s"
dc --profile workers up -d worker >/dev/null 2>&1
# A worker stopped mid-drain re-registers draining once and exits 0 (sticky drain, RV5-02): start it again then.
until [ "$(psqlq -Atc "SELECT count(*) FROM workers WHERE status = 'active' AND last_heartbeat_at > now() - interval '5 seconds'")" -ge 1 ]; do
  sleep 2; dc --profile workers up -d worker >/dev/null 2>&1
done
for i in $(seq 40); do sub "perm-$i" '{"handler_name":"fail_n_times","handler_version":1,"payload":{"n":1,"permanent":true}}'; done
for i in $(seq 30); do sub "long-$i" '{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":60000}}'; done
until [ "$(psqlq -Atc "SELECT count(*) FROM job_attempts WHERE state = 'active'")" -ge 4 ]; do sleep 1; done
log "SIGKILL worker-1 (holding $(psqlq -Atc "SELECT count(*) FROM job_attempts WHERE state = 'active'") active attempts)"
docker kill --signal KILL "$(dc ps -q worker)" >/dev/null
for a in ClusterxWorkerOffline ClusterxLeaseExpiriesRepeated ClusterxFailureRateHigh ClusterxNoLiveWorkers ClusterxQueueAgeHigh ClusterxJobsUnschedulable; do
  wait_firing $a 600 || true
done
psqlq -c "SELECT status, count(*) FROM jobs GROUP BY 1 ORDER BY 1" | tee -a "$EV/timeline.txt"

log "phase B1: fill the 64 MiB drill volume to ~95%"
docker run --rm -v "${COMPOSE_PROJECT_NAME}_diskdrill:/fill" --entrypoint dd postgres:18.7-bookworm@sha256:ece55f7868da684e400f82f1030a050718adbdd65c23aa500c486f6e2950dc0f \
  if=/dev/zero of=/fill/blob bs=1M count=61 2>&1 | tail -1 | tee -a "$EV/timeline.txt"
wait_firing ClusterxDiskSpaceLow 300 || true
log "phase B2: stop postgres"
dc stop postgres >/dev/null 2>&1
wait_firing ClusterxDatabaseUnavailable 180 || true
log "admission during DB outage: HTTP $(curl -s -o /dev/null -w '%{http_code}' -H @"$HDR" -H 'Idempotency-Key: during-outage' -d '{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"x"}}' "$API/api/v1/jobs")"
dc start postgres >/dev/null 2>&1
log "phase B3: stop control-plane"
dc stop control-plane >/dev/null 2>&1
wait_firing ClusterxControlPlaneDown 240 || true
sleep 15
log "final firing: [$(firing)]"
curl -s "$PROM/api/v1/rules?type=alert" > "$EV/rules-api.json"

log "cleanup: start control-plane, stop monitoring, remove drill volume"
dc up -d --no-deps control-plane 2>/dev/null
dc --profile monitoring rm -sf prometheus node-exporter >/dev/null 2>&1
docker volume rm "${COMPOSE_PROJECT_NAME}_diskdrill" >/dev/null 2>&1 || true
