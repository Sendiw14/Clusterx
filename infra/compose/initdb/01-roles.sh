#!/bin/sh
# Creates group roles ahead of migration 0002 and the least-privilege runtime login (NF-04).
set -eu
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v app_pw="$CLUSTERX_APP_DB_PASSWORD" <<'SQL'
CREATE ROLE clusterx_runtime NOLOGIN;
CREATE ROLE clusterx_retention NOLOGIN;
CREATE ROLE clusterx_app LOGIN PASSWORD :'app_pw' IN ROLE clusterx_runtime;
SQL
