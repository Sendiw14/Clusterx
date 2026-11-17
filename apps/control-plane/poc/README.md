# Race PoC (throwaway, M1)

Proves the ADR-0002/0003 SQL patterns against real PostgreSQL 18 before M2 implementation.
Not production code; M2 re-implements in `internal/store` with the same predicates.

Covers: AC-02 (concurrent claim, failpoints), AC-03 (stale success after expiry+reclaim),
AC-06 (cancel vs success, both orders and concurrent).

```
CLUSTERX_PG_ADMIN_DSN='postgres://clusterx@localhost:55432/postgres?host=/path/to/socketdir' \
  go test -race -count=1 -v ./poc/
```
The tests create (and drop) database `clusterx_poc`, apply `migrations/0001_init.sql`, and are
skipped — not passed — when the env var is unset.
