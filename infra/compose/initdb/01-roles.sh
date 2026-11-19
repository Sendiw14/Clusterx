#!/bin/sh
# Creates group roles ahead of migration 0002, the least-privilege runtime login (NF-04) and the
# retention login (ADR-0004). Passwords come from compose secrets files (RV5-09), never env vars.
set -eu
app_pw=$(cat /run/secrets/app_db_password)
ret_pw=$(cat /run/secrets/retention_db_password)
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v app_pw="$app_pw" -v ret_pw="$ret_pw" <<'SQL'
CREATE ROLE clusterx_runtime NOLOGIN;
CREATE ROLE clusterx_retention NOLOGIN;
CREATE ROLE clusterx_app LOGIN PASSWORD :'app_pw' IN ROLE clusterx_runtime;
CREATE ROLE clusterx_retention_job LOGIN PASSWORD :'ret_pw' IN ROLE clusterx_retention;
SQL
