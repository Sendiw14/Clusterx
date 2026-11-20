#!/usr/bin/env bash
# AC-17 drill on the isolated benchmark stack (scripts/bench/up.sh): populate jobs in every state with running
# attempts, back up, keep writing, declare a disaster (control plane + workers stopped), restore into a NEW
# database, point the control plane at it, restart workers, wait until every job is terminal, check the
# invariants, then switch back. Prints timestamps for RPO/RTO. Never run against a stack you care about.
#   scripts/ops/drill-backup-restore.sh <evidence-dir> <backup-dir>
set -euo pipefail
EV=$(mkdir -p "$1" && cd "$1" && pwd); BK=$(mkdir -p "$2" && cd "$2" && pwd) # archives hold job data: keep them out of git
. "$(dirname "$0")/../bench/env.sh"   # cd infra/compose, dc, psqlq, envval
OPS="$ROOT/scripts/ops"
API=http://127.0.0.1:$(envval CLUSTERX_HTTP_PORT)
HDR=$(mktemp); trap 'rm -f "$HDR"' EXIT; chmod 600 "$HDR"
printf 'Authorization: Bearer %s\nContent-Type: application/json\n' "$(sed -n 's/^CLUSTERX_OPERATOR_TOKEN=//p' .secrets.env)" > "$HDR"
ts() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
sub() { curl -sS -H @"$HDR" -H "Idempotency-Key: drill-$1" -d "$2" "$API/api/v1/jobs"; }
id_of() { python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])'; }
q() { dc exec -T postgres psql -U clusterx_admin -d "$1" -X -q -v ON_ERROR_STOP=1 "${@:2}"; }
log() { echo "$*" | tee -a "$EV/drill.txt"; }
: > "$EV/drill.txt"

# Fresh workload state; 3 workers.
dc --profile workers stop "${ALL_WORKERS[@]}" >/dev/null 2>&1 || true
psqlq -c "SET session_replication_role = replica; TRUNCATE jobs, job_attempts, job_events, submission_idempotency, job_tombstones" >/dev/null
dc --profile workers up -d worker worker-2 worker-3 2>/dev/null
sleep 5
for i in $(seq 20); do sub "ok-$i" '{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"d'$i'"}}' >/dev/null; done
for i in $(seq 5); do sub "perm-$i" '{"handler_name":"fail_n_times","handler_version":1,"payload":{"n":1,"permanent":true}}' >/dev/null; done
for i in $(seq 20); do sub "long-$i" '{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":20000}}' >/dev/null; done
for i in $(seq 5); do sub "sched-$i" '{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"s"},"run_after":"'"$(date -u -v+1H +%FT%TZ)"'"}' >/dev/null; done
for i in $(seq 3); do sub "retry-$i" '{"handler_name":"fail_n_times","handler_version":1,"payload":{"n":2}}' >/dev/null; done
for i in $(seq 3); do
  id=$(sub "cancel-$i" '{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":20000},"priority":0}' | id_of)
  curl -sS -o /dev/null -X POST -H @"$HDR" "$API/api/v1/jobs/$id/cancel"
done
sleep 3
log "== state at backup"
q clusterx -c "SELECT status, count(*) FROM jobs GROUP BY 1 ORDER BY 1" | tee -a "$EV/drill.txt"
q clusterx -Atc "SELECT 'active_attempts=' || count(*) FROM job_attempts WHERE state = 'active'" | tee -a "$EV/drill.txt"
q clusterx -Atc "SELECT 'last_commit_before_backup=' || max(created_at) FROM jobs" | tee -a "$EV/drill.txt"

T_BACKUP=$(ts); log "backup_start_epoch=$T_BACKUP"
"$OPS/backup.sh" "$BK" | tee -a "$EV/drill.txt"
ARCHIVE=$(ls -t "$BK"/clusterx-clusterx-*.dump | head -1)

# Writes after the backup: these are lost by a restore (the RPO window).
for i in $(seq 10); do sub "after-$i" '{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"late"}}' >/dev/null; done
sleep 1
T_DISASTER=$(ts); log "disaster_epoch=$T_DISASTER (control plane + workers stopped; restoring from backup)"
dc --profile workers stop control-plane "${ALL_WORKERS[@]}" >/dev/null 2>&1

NEWDB=clusterx_restored_$(date -u +%H%M%S)
"$OPS/restore.sh" "$ARCHIVE" "$NEWDB" | tee -a "$EV/drill.txt"
cp .secrets/database_url "$EV/.database_url.orig"; chmod 600 "$EV/.database_url.orig"
sed "s#/clusterx?#/$NEWDB?#" "$EV/.database_url.orig" > .secrets/database_url
dc up -d --no-deps --force-recreate control-plane 2>/dev/null
until [ "$(curl -s -o /dev/null -w '%{http_code}' "$API/health/ready")" = 200 ]; do sleep 0.2; done
T_READY=$(ts); log "control_plane_ready_on_restored_db_epoch=$T_READY"
dc --profile workers up -d worker worker-2 worker-3 2>/dev/null
log "== restored DB immediately after start (before lease recovery)"
q "$NEWDB" -c "SELECT status, count(*) FROM jobs GROUP BY 1 ORDER BY 1" | tee -a "$EV/drill.txt"
# Wait for every non-scheduled job to finish (expired leases are recovered after the 30 s lease TTL).
for _ in $(seq 300); do
  left=$(q "$NEWDB" -Atc "SELECT count(*) FROM jobs WHERE terminal_at IS NULL AND status <> 'scheduled'")
  [ "$left" -eq 0 ] && break; sleep 1
done
T_DRAINED=$(ts); log "all_non_scheduled_terminal_epoch=$T_DRAINED left=$left"
log "== restored DB after recovery"
q "$NEWDB" -f - < "$ROOT/scripts/bench/reconcile.sql" | sed -n '/== jobs by status/,/== end-to-end latency (/p' | tee -a "$EV/drill.txt"
q "$NEWDB" -Atc "SELECT 'post_backup_jobs_present=' || count(*) FROM job_events WHERE event_type = 'admitted' AND occurred_at > to_timestamp($T_BACKUP)" | tee -a "$EV/drill.txt"
q "$NEWDB" -Atc "SELECT 'stuck_running=' || count(*) FROM jobs WHERE status = 'running' AND lease_expires_at < now()" | tee -a "$EV/drill.txt"
q "$NEWDB" -Atc "SELECT 'lease_expired_attempts=' || count(*) FROM job_attempts WHERE state = 'lease_expired'" | tee -a "$EV/drill.txt"
log "rpo_window_s=$(echo "$T_DISASTER - $T_BACKUP" | bc) rto_service_s=$(echo "$T_READY - $T_DISASTER" | bc) rto_work_drained_s=$(echo "$T_DRAINED - $T_DISASTER" | bc)"

# Switch back to the original database.
cp "$EV/.database_url.orig" .secrets/database_url; rm -f "$EV/.database_url.orig"
dc up -d --no-deps --force-recreate control-plane 2>/dev/null
dc --profile workers stop "${ALL_WORKERS[@]}" >/dev/null 2>&1 || true
log "switched back to $(sed 's#.*/\([a-z_]*\)?.*#\1#' .secrets/database_url)"
