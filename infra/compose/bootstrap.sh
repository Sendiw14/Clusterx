#!/usr/bin/env bash
# Local bootstrap: write the compose secrets files (./.secrets/, gitignored, mode 0700 directory), generate a
# development BFF key pair, start DB + control plane, mint one operator token and three worker credentials
# (printed once by the server CLI; recorded in gitignored infra/compose/.secrets.env and as secrets files),
# start 3 workers. Honours COMPOSE_PROJECT_NAME and CLUSTERX_IMAGE_PREFIX (separate stacks / image tags).
set -euo pipefail
cd "$(dirname "$0")"
ENVF=../../.env; SECRETS=.secrets.env; SD=.secrets
[ -f "$ENVF" ] || { echo "missing $ENVF — copy .env.example and fill it" >&2; exit 1; }
set -a; . "$ENVF"; set +a
: "${POSTGRES_PASSWORD:?set in .env}" "${CLUSTERX_APP_DB_PASSWORD:?set in .env}" "${CLUSTERX_METRICS_TOKEN:?set in .env}"
dc() { docker compose --env-file "$ENVF" "$@"; }

umask 077; mkdir -p "$SD"; chmod 700 "$SD"
put() { printf '%s' "$2" > "$SD/$1"; }                 # always refreshed from .env
keep() { [ -s "$SD/$1" ] || printf '%s' "$2" > "$SD/$1"; } # generated once
rand() { head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n'; }
put postgres_password "$POSTGRES_PASSWORD"
put app_db_password "$CLUSTERX_APP_DB_PASSWORD"
put metrics_token "$CLUSTERX_METRICS_TOKEN"
put database_url "postgres://clusterx_app:${CLUSTERX_APP_DB_PASSWORD}@postgres:5432/clusterx?sslmode=disable"
put migrate_database_url "postgres://clusterx_admin:${POSTGRES_PASSWORD}@postgres:5432/clusterx?sslmode=disable"
keep retention_db_password "$(rand)"
put retention_database_url "postgres://clusterx_retention_job:$(cat "$SD/retention_db_password")@postgres:5432/clusterx?sslmode=disable"
for i in 1 2 3; do [ -e "$SD/worker_credential_$i" ] || : > "$SD/worker_credential_$i"; done
[ -e "$SD/bff-dev.jwks.json" ] || : > "$SD/bff-dev.jwks.json"
# Containers run as 65532; on Linux hosts they need read access to the bind-mounted files.
chmod 644 "$SD"/*; chmod 700 "$SD"; [ ! -e "$SD/bff-dev-private.jwk.json" ] || chmod 600 "$SD/bff-dev-private.jwk.json"

dc build migrate worker
if [ ! -s "$SD/bff-dev.jwks.json" ]; then
  # Development BFF key pair (dev-keygen is refused in production): public JWKS for the control plane,
  # private JWK (0600) for a local BFF. Written through a temporary container mount.
  dc run --rm --no-deps -v "$PWD/$SD:/out" migrate dev-keygen --out-dir /out >/dev/null
  chmod 600 "$SD/bff-dev-private.jwk.json"; chmod 644 "$SD/bff-dev.jwks.json"
fi
dc up -d --wait postgres control-plane
if [ ! -s "$SECRETS" ]; then
  : > "$SECRETS"
  cli() { local v; v=$(dc run --rm --no-deps control-plane "$@" | sed -n 's/^secret=//p'); [ -n "$v" ] || { echo "secret creation failed: $*" >&2; rm -f "$SECRETS"; exit 1; }; printf '%s' "$v"; }
  t=$(cli token create --role operator --name local-operator --owner-github-id 1 --owner-login local); echo "CLUSTERX_OPERATOR_TOKEN=$t" >> "$SECRETS"
  for i in 1 2 3; do c=$(cli worker-credential create --name worker-$i); echo "CLUSTERX_WORKER_CREDENTIAL_$i=$c" >> "$SECRETS"; done
fi
for i in 1 2 3; do
  v=$(sed -n "s/^CLUSTERX_WORKER_CREDENTIAL_$i=//p" "$SECRETS")
  if [ -n "$v" ]; then printf '%s' "$v" > "$SD/worker_credential_$i"; fi
done
chmod 644 "$SD"/worker_credential_*
dc --profile workers up -d worker worker-2 worker-3
echo "Ready: http://127.0.0.1:${CLUSTERX_HTTP_PORT:-8080}/health/ready — operator token is in infra/compose/.secrets.env"
