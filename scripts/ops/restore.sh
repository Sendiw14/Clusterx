#!/usr/bin/env bash
# Restore a backup.sh archive into a NEW database of the same PostgreSQL cluster (never over the live one):
# checksum verified, database must not exist, single transaction, stop on the first error. The cluster must
# already have the CLUSTERX roles (infra/compose/initdb/01-roles.sh); grants are restored from the archive.
# Point the control plane at the restored database afterwards (docs/operations/backup-restore.md).
#   scripts/ops/restore.sh <archive.dump> <new_database_name>
set -euo pipefail
. "$(dirname "$0")/lib.sh"
[ $# -eq 2 ] || { echo "usage: $0 <archive.dump> <new_database_name>" >&2; exit 2; }
f=$1; new=$2
[[ "$new" =~ ^[a-z][a-z0-9_]{0,62}$ ]] || { echo "database name must match ^[a-z][a-z0-9_]{0,62}$" >&2; exit 2; }
[ -f "$f.sha256" ] || { echo "missing $f.sha256" >&2; exit 1; }
(cd "$(dirname "$f")" && shasum -a 256 -c "$(basename "$f").sha256" >/dev/null) || { echo "checksum mismatch: $f" >&2; exit 1; }
exists=$(pgx psql -U "$ADMIN" -d postgres -XAtc "SELECT 1 FROM pg_database WHERE datname = '$new'")
[ -z "$exists" ] || { echo "database $new already exists; restore only into a new database" >&2; exit 1; }
t0=$(now_ms)
pgx createdb -U "$ADMIN" -T template0 "$new"
pgx pg_restore -U "$ADMIN" -d "$new" --exit-on-error --single-transaction < "$f"
pgx psql -U "$ADMIN" -d "$new" -XAtq -c "ANALYZE"
t1=$(now_ms)
echo "restored=$new from=$(basename "$f") started_epoch=$t0 finished_epoch=$t1 seconds=$(echo "$t1 - $t0" | bc)"
