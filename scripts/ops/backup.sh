#!/usr/bin/env bash
# Logical backup of the CLUSTERX database: pg_dump custom format (-Fc) as the migration/admin role, run inside
# the postgres container, streamed to <out-dir>/clusterx-<db>-<UTC>.dump (mode 0600) with a .sha256 file.
# The archive is verified readable (pg_restore --list) before it is renamed into place.
#   scripts/ops/backup.sh <out-dir>
# Optional CLUSTERX_BACKUP_KEEP_DAYS=N prunes this script's own archives older than N days in <out-dir>
# (retention baseline 14 days, MASTER_PROMPT §6). Backups contain job payloads/results: store them encrypted.
set -euo pipefail
. "$(dirname "$0")/lib.sh"
[ $# -eq 1 ] || { echo "usage: $0 <out-dir>" >&2; exit 2; }
umask 077
out=$(mkdir -p "$1" && cd "$1" && pwd)
ts=$(date -u +%Y%m%dT%H%M%SZ)
f="$out/clusterx-$DB-$ts.dump"
trap 'rm -f "$f.partial" "$f.toc"' EXIT # no half-written archive is left behind; the TOC is only used for the count
t0=$(now_ms)
pgx pg_dump -U "$ADMIN" -d "$DB" -Fc -Z 6 > "$f.partial"
pgx pg_restore --list < "$f.partial" > "$f.toc"
mv "$f.partial" "$f"
(cd "$out" && shasum -a 256 "$(basename "$f")" > "$(basename "$f").sha256")
t1=$(now_ms)
echo "backup=$f bytes=$(wc -c < "$f" | tr -d ' ') toc_entries=$(grep -vc '^;' "$f.toc") started_epoch=$t0 finished_epoch=$t1 seconds=$(echo "$t1 - $t0" | bc)"
if [ -n "${CLUSTERX_BACKUP_KEEP_DAYS:-}" ]; then
  find "$out" -maxdepth 1 -type f -name "clusterx-$DB-*.dump*" -mtime +"$CLUSTERX_BACKUP_KEEP_DAYS" -print -delete
fi
