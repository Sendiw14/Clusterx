package api

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/migrate"
)

// DBReadiness reports ready when pool answers a ping and schema_migrations matches migs.
func DBReadiness(pool *pgxpool.Pool, migs []migrate.Migration) ReadinessCheck {
	return func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return err
		}
		return migrate.Verify(ctx, pool, migs)
	}
}
