#!/usr/bin/env bash
# LOCAL PROOF ONLY (not for a real deployment): creates ./secrets with a self-signed internal CA, a site certificate
# for CLUSTERX_WEB_HOST/CLUSTERX_API_HOST, a control-plane certificate (SAN control-plane), generated DB passwords,
# metrics token, web session key, a BFF Ed25519 key pair and placeholder GitHub OAuth values (login is not
# exercised), builds the images, starts the stack and mints one operator token + three worker credentials.
# A real deployment provisions these from its own secret store and a public CA (docs/operations/deployment.md).
set -euo pipefail
cd "$(dirname "$0")"
ENVF=${CLUSTERX_ENV_FILE:-.env.production}
[ -f "$ENVF" ] || { echo "missing $ENVF (copy .env.production.example)" >&2; exit 1; }
set -a; . "./$ENVF"; set +a
dc() { docker compose --env-file "$ENVF" "$@"; }
S=secrets; umask 077; mkdir -p "$S"; chmod 700 "$S"
rand() { openssl rand -hex 32; }
keep() { [ -s "$S/$1" ] || printf '%s' "$2" > "$S/$1"; }

keep postgres_password "$(rand)"; keep app_db_password "$(rand)"; keep retention_db_password "$(rand)"
keep metrics_token "$(rand)"; keep web_session_key "$(openssl rand -base64 32)"
keep github_client_id "local-proof-placeholder"; keep github_client_secret "local-proof-placeholder"
keep database_url "postgres://clusterx_app:$(cat $S/app_db_password)@postgres:5432/clusterx?sslmode=disable"
keep migrate_database_url "postgres://clusterx_admin:$(cat $S/postgres_password)@postgres:5432/clusterx?sslmode=disable"
for i in 1 2 3; do [ -e "$S/worker_credential_$i" ] || : > "$S/worker_credential_$i"; done

if [ ! -s "$S/internal-ca.crt" ]; then
  openssl ecparam -name prime256v1 -genkey -noout -out "$S/internal-ca.key"
  openssl req -x509 -new -key "$S/internal-ca.key" -sha256 -days 30 -subj "/CN=CLUSTERX local proof CA" \
    -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" -out "$S/internal-ca.crt"
  leaf() { # name, SAN list
    openssl ecparam -name prime256v1 -genkey -noout -out "$S/$1.key"
    openssl req -new -key "$S/$1.key" -subj "/CN=$1" -out "$S/$1.csr"
    printf 'subjectAltName=%s\nextendedKeyUsage=serverAuth\nkeyUsage=critical,digitalSignature\n' "$2" > "$S/$1.ext"
    openssl x509 -req -in "$S/$1.csr" -CA "$S/internal-ca.crt" -CAkey "$S/internal-ca.key" -CAcreateserial \
      -days 30 -sha256 -extfile "$S/$1.ext" -out "$S/$1.crt" 2>/dev/null
    rm -f "$S/$1.csr" "$S/$1.ext"
  }
  leaf site "DNS:$CLUSTERX_WEB_HOST,DNS:$CLUSTERX_API_HOST"
  leaf control-plane "DNS:control-plane"
fi
# Containers run as non-root UIDs (65532, postgres 999, caddy root): files readable inside the 0700 directory.
chmod 644 "$S"/*; chmod 600 "$S/internal-ca.key"; chmod 700 "$S"

[ -n "${CLUSTERX_SKIP_BUILD:-}" ] || dc build migrate web worker # images already built: CLUSTERX_SKIP_BUILD=1
if [ ! -s "$S/bff.jwks.json" ]; then
  kg=$(mktemp -d "$PWD/.keygen.XXXXXX"); trap 'rm -rf "$kg"' EXIT
  # dev-keygen is a development-only command; it only writes a fresh Ed25519 key pair here.
  dc run --rm --no-deps --user "$(id -u):$(id -g)" -e CLUSTERX_ENV=development -v "$kg:/out" migrate dev-keygen --out-dir /out >/dev/null
  mv "$kg/bff-dev.jwks.json" "$S/bff.jwks.json"; mv "$kg/bff-dev-private.jwk.json" "$S/bff-private.jwk.json"
  chmod 644 "$S/bff.jwks.json" "$S/bff-private.jwk.json"
fi
dc up -d postgres migrate control-plane web caddy
if [ ! -s "$S/operator_token" ]; then
  cli() { dc run --rm --no-deps control-plane "$@" | sed -n 's/^secret=//p' | tr -d '\n'; }
  cli token create --role operator --name prod-proof-operator --owner-github-id 1 --owner-login local > "$S/operator_token"
  chmod 600 "$S/operator_token"
  for i in 1 2 3; do cli worker-credential create --name worker-$i > "$S/worker_credential_$i"; chmod 644 "$S/worker_credential_$i"; done
fi
dc up -d worker worker-2 worker-3
echo "up: https://$CLUSTERX_WEB_HOST${CLUSTERX_PUBLIC_PORT_SUFFIX:-} and https://$CLUSTERX_API_HOST${CLUSTERX_PUBLIC_PORT_SUFFIX:-} (CA: infra/production/secrets/internal-ca.crt)"
