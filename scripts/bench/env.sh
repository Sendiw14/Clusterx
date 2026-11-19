# Sourced by the bench scripts: one isolated compose stack (project, image prefix, ports) on top of
# infra/compose plus the benchmark overlay. Override any variable from the environment.
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
export COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-clusterx-m8}
export COMPOSE_FILE=${COMPOSE_FILE:-compose.yaml:../../scripts/bench/compose.bench.yaml}
cd "$ROOT/infra/compose"
dc() { docker compose --env-file "$ROOT/.env" "$@"; }
psqlq() { dc exec -T postgres psql -U clusterx_admin -d clusterx -X -q -v ON_ERROR_STOP=1 "$@"; }
envval() { sed -n "s/^$1=//p" "$ROOT/.env"; }
ALL_WORKERS=(worker worker-2 worker-3 worker-4 worker-5)
