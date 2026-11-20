#!/usr/bin/env bash
# Checks the local production proof (setup-local-proof.sh) end to end and prints the evidence: TLS handshakes against
# the local CA, API + web over HTTPS through Caddy, a job executed by workers over gRPC+TLS, published ports, container
# hardening, and that no secret value is in any container environment.
#   ./verify-local-proof.sh > ../../docs/testing/evidence/m8/production/e2e.txt
set -uo pipefail
cd "$(dirname "$0")"
ENVF=${CLUSTERX_ENV_FILE:-.env.production}; set -a; . "./$ENVF"; set +a
dc() { docker compose --env-file "$ENVF" "$@"; }
P=${CLUSTERX_HTTPS_PORT}; W=$CLUSTERX_WEB_HOST; A=$CLUSTERX_API_HOST; CA=secrets/internal-ca.crt
c() { curl -sS --resolve "$W:$P:127.0.0.1" --resolve "$A:$P:127.0.0.1" "$@"; }
HDR=$(mktemp); trap 'rm -f "$HDR"' EXIT; chmod 600 "$HDR"
printf 'Authorization: Bearer %s\nContent-Type: application/json\n' "$(cat secrets/operator_token)" > "$HDR"

echo "== TLS handshake (openssl s_client, verified against the local CA)"
for h in "$W" "$A"; do echo "-- $h"; echo | openssl s_client -connect "127.0.0.1:$P" -servername "$h" -CAfile "$CA" 2>/dev/null | grep -E '^(subject|issuer)=|^Verify return code|Protocol *:' | sort -u; done
echo; echo "== client without the CA is refused"; c -o /dev/null "https://$A:$P/health/live" 2>&1 | head -1; echo "curl exit=${PIPESTATUS[0]}"
echo; echo "== API over HTTPS (Caddy -> control plane over TLS, internal CA)"
c --cacert "$CA" -o /dev/null -w 'GET /health/ready %{http_code} http/%{http_version}\n' "https://$A:$P/health/ready"
c --cacert "$CA" -o /dev/null -w 'GET /metrics on the API host %{http_code}\n' "https://$A:$P/metrics"
c --cacert "$CA" -o /dev/null -w 'GET /api/v1/jobs without a token %{http_code}\n' "https://$A:$P/api/v1/jobs"
echo; echo "== job executed by workers over gRPC+TLS"
id=$(c --cacert "$CA" -H @"$HDR" -H "Idempotency-Key: prod-proof-$(date +%s)" -d '{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"prod-proof"}}' "https://$A:$P/api/v1/jobs" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
echo "submitted job=$id"
for _ in $(seq 30); do st=$(c --cacert "$CA" -H @"$HDR" "https://$A:$P/api/v1/jobs/$id" | python3 -c 'import json,sys; j=json.load(sys.stdin); print(j["status"], "attempts_used=%s" % j.get("attempts_used"))'); case $st in succeeded*|failed*) break;; esac; sleep 1; done
echo "status=$st"
c --cacert "$CA" -H @"$HDR" "https://$A:$P/api/v1/workers" | python3 -c 'import json,sys; [print("worker", w.get("name"), w.get("status"), w.get("liveness")) for w in json.load(sys.stdin)["items"]]'
dc logs worker worker-2 worker-3 2>&1 | grep -oE '"worker_name":"worker-[0-9]","env":"production","control_plane":"[^"]*","tls":[a-z]+' | sort -u
echo; echo "== web over HTTPS"
c --cacert "$CA" -o /dev/null -w 'GET / %{http_code}\n' "https://$W:$P/"
c --cacert "$CA" -o /dev/null -w 'GET /app %{http_code} -> %{redirect_url}\n' "https://$W:$P/app"
c --cacert "$CA" -o /dev/null -w 'GET /demo %{http_code}\n' "https://$W:$P/demo"
c --cacert "$CA" -D - -o /dev/null "https://$W:$P/" | grep -iE '^(strict-transport-security|content-security-policy|x-frame-options|server):'
curl -sS -o /dev/null -w "plain HTTP :${CLUSTERX_HTTP_REDIRECT_PORT} -> %{http_code} %{redirect_url}\n" --resolve "$W:${CLUSTERX_HTTP_REDIRECT_PORT}:127.0.0.1" "http://$W:${CLUSTERX_HTTP_REDIRECT_PORT}/"
echo; echo "== published ports"; docker ps --filter "label=com.docker.compose.project=$COMPOSE_PROJECT_NAME" --format '{{.Names}} [{{.Ports}}]' | sort
echo; echo "== container hardening"
for id in $(dc ps -q); do docker inspect -f '{{.Name}} user={{.Config.User}} read_only={{.HostConfig.ReadonlyRootfs}} cap_drop={{.HostConfig.CapDrop}} cap_add={{.HostConfig.CapAdd}} mem={{.HostConfig.Memory}} pids={{.HostConfig.PidsLimit}} secopt={{.HostConfig.SecurityOpt}}' "$id"; done | sort
echo; echo "== secret-looking environment values (expect none; only *_FILE paths)"
for id in $(dc ps -q); do n=$(docker inspect -f '{{.Name}}' "$id"); docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$id" | grep -iE 'password|token|secret|session_key|database_url|credential' | grep -vE '_FILE=/run/secrets/' | sed "s#^#$n: #"; done; echo "(end)"
echo; echo "== networks"; for n in data workers edge; do echo "$n internal=$(docker network inspect -f '{{.Internal}}' "${COMPOSE_PROJECT_NAME}_$n")"; done
