# Sourced by backup.sh / restore.sh. Targets the postgres service of a compose stack:
#   CLUSTERX_COMPOSE_DIR  compose project directory (default infra/compose; infra/production for the production example)
#   CLUSTERX_ENV_FILE     env file passed to compose (default <repo>/.env; production: infra/production/.env.production)
#   COMPOSE_PROJECT_NAME  as for docker compose
#   CLUSTERX_DB_NAME (clusterx), CLUSTERX_DB_ADMIN (clusterx_admin: schema owner / migration role)
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
DIR=${CLUSTERX_COMPOSE_DIR:-$ROOT/infra/compose}
ENVF=${CLUSTERX_ENV_FILE:-$ROOT/.env}
DB=${CLUSTERX_DB_NAME:-clusterx}
ADMIN=${CLUSTERX_DB_ADMIN:-clusterx_admin}
dc() { (cd "$DIR" && docker compose --env-file "$ENVF" "$@"); }
pgx() { dc exec -T postgres "$@"; } # runs inside the postgres container (local socket, no password on the wire)
now_ms() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
