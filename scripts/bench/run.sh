#!/usr/bin/env bash
# One benchmark run: reset job tables (admin TRUNCATE with the M4 truncate guard bypassed via
# session_replication_role; benchmark stack only), start N workers, submit the fixed 1,000-job workload, wait for all
# jobs to be terminal, sample docker stats and lock waits, write the reconciliation report.
#   scripts/bench/run.sh <workers 1..5> <out dir> [kill]   ("kill": SIGKILL worker-2 after 300 terminal jobs)
set -euo pipefail
N=$1; OUT=$(mkdir -p "$2" && cd "$2" && pwd); KILL=${3:-}
JOBS=${JOBS:-1000}; CONC=${CONC:-16}
. "$(dirname "$0")/env.sh"
API=http://127.0.0.1:$(envval CLUSTERX_HTTP_PORT)
TOKEN_FILE=$(mktemp); trap 'rm -f "$TOKEN_FILE"' EXIT
sed -n 's/^CLUSTERX_OPERATOR_TOKEN=//p' .secrets.env | tr -d '\n' > "$TOKEN_FILE"

dc --profile workers stop "${ALL_WORKERS[@]}" >/dev/null 2>&1 || true
psqlq -c "SET session_replication_role = replica; TRUNCATE jobs, job_attempts, job_events, submission_idempotency, job_tombstones;
  SELECT pg_stat_reset(); SELECT pg_stat_statements_reset();" >/dev/null
dc --profile workers up -d "${ALL_WORKERS[@]:0:$N}" 2>/dev/null
for _ in $(seq 60); do
  live=$(psqlq -Atc "SELECT count(*) FROM workers WHERE status = 'active' AND last_heartbeat_at > now() - interval '5 seconds'")
  [ "$live" -ge "$N" ] && break; sleep 1
done
echo "live_workers=$live" > "$OUT/env.txt"
{ date -u +%FT%TZ; docker version --format 'docker server {{.Server.Version}}'; colima list 2>/dev/null; sysctl -n machdep.cpu.brand_string hw.ncpu hw.memsize 2>/dev/null; } >> "$OUT/env.txt"

# Cumulative cgroup v2 counters per container (CPU usage_usec, memory.peak since container start, memory.current),
# read inside the Docker VM (Colima) or on the Linux host.
cg() {
  local ids cmd
  ids=$(docker ps -q --no-trunc -f "label=com.docker.compose.project=$COMPOSE_PROJECT_NAME" | tr '\n' ' ')
  cmd='for i in '"$ids"'; do d=/sys/fs/cgroup/docker/$i; [ -d $d ] || d=/sys/fs/cgroup/system.slice/docker-$i.scope; echo $i $(sed -n "s/^usage_usec //p" $d/cpu.stat) $(cat $d/memory.peak) $(cat $d/memory.current); done'
  if command -v colima >/dev/null; then colima ssh -- sh -c "$cmd"; else sh -c "$cmd"; fi |
    while read -r id cpu peak cur; do echo "$(docker inspect -f '{{.Name}}' "$id" | tr -d /) $cpu $peak $cur"; done
}
cg > "$OUT/cgroup-before.txt"
( while :; do docker stats --no-stream --format "$(date +%s),{{.Name}},{{.CPUPerc}},{{.MemUsage}}" | grep "$COMPOSE_PROJECT_NAME" ; done ) > "$OUT/docker-stats.csv" 2>/dev/null &
SPID=$!
( while :; do psqlq -Atc "SELECT extract(epoch FROM now())::int, (SELECT count(*) FROM pg_locks WHERE NOT granted), (SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock'), (SELECT count(*) FROM pg_stat_activity WHERE datname = 'clusterx')" -F, ; sleep 1; done ) > "$OUT/lock-samples.csv" 2>/dev/null &
LPID=$!
KPID=
if [ "$KILL" = kill ]; then
  ( until [ "$(psqlq -Atc "SELECT count(*) FROM jobs WHERE terminal_at IS NOT NULL")" -ge 300 ]; do sleep 0.2; done
    echo "kill_at_epoch=$(perl -MTime::HiRes=time -e 'printf "%.3f", time') running_on_worker2=$(psqlq -Atc "SELECT count(*) FROM job_attempts a JOIN workers w ON w.id = a.worker_id WHERE a.state = 'active' AND w.name = 'worker-2'")"
    docker kill --signal KILL "$(dc ps -q worker-2)" >/dev/null ) > "$OUT/kill.txt" &
  KPID=$!
fi
START=$(date +%s); since=$(date -u +%FT%TZ)
(cd "$ROOT/scripts/bench/driver" && GOWORK=off go run . -api "$API" -token-file "$TOKEN_FILE" -n "$JOBS" -c "$CONC" -run "w$N$KILL-$START" -out "$OUT/admission.csv") | tee "$OUT/driver.txt"
for _ in $(seq 900); do
  left=$(psqlq -Atc "SELECT count(*) FROM jobs WHERE terminal_at IS NULL")
  [ "$left" -eq 0 ] && break; sleep 1
done
END=$(date +%s)
[ -z "$KPID" ] || wait "$KPID" || true
kill "$SPID" "$LPID" 2>/dev/null || true
cg > "$OUT/cgroup-after.txt"
echo "wall_s_driver_start_to_all_terminal=$((END - START)) non_terminal_left=$left" | tee -a "$OUT/driver.txt"
psqlq -f - < "$ROOT/scripts/bench/reconcile.sql" > "$OUT/reconcile.txt"
echo "lock_waits_over_100ms_logged=$(dc logs --since "$since" postgres 2>&1 | grep -c 'still waiting for' || true)" >> "$OUT/reconcile.txt"
cat "$OUT/reconcile.txt"
