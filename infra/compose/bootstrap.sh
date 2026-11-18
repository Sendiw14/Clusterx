#!/usr/bin/env bash
# Local bootstrap: start DB + control plane, mint one operator token and three worker credentials
# (printed once by the server CLI, stored only in gitignored infra/compose/.secrets.env), start 3 workers.
set -euo pipefail
cd "$(dirname "$0")"
ENVF=../../.env; SECRETS=.secrets.env
[ -f "$ENVF" ] || { echo "missing $ENVF — copy .env.example and fill it" >&2; exit 1; }
dc() { docker compose --env-file "$ENVF" ${SECRETS_ARG:-} "$@"; }
dc up -d --build --wait postgres control-plane
if [ ! -s "$SECRETS" ]; then
  umask 077; : > "$SECRETS"
  cli() { local v; v=$(dc run --rm --no-deps control-plane "$@" | sed -n 's/^secret=//p'); [ -n "$v" ] || { echo "secret creation failed: $*" >&2; rm -f "$SECRETS"; exit 1; }; printf '%s' "$v"; }
  t=$(cli token create --role operator --name local-operator --owner-github-id 1 --owner-login local); echo "CLUSTERX_OPERATOR_TOKEN=$t" >> "$SECRETS"
  for i in 1 2 3; do c=$(cli worker-credential create --name worker-$i); echo "CLUSTERX_WORKER_CREDENTIAL_$i=$c" >> "$SECRETS"; done
fi
SECRETS_ARG="--env-file $SECRETS"
dc --profile workers up -d --build worker worker-2 worker-3
echo "Ready: http://127.0.0.1:8080/health/ready — operator token is in infra/compose/.secrets.env"
