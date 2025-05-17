package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type ConnectionPoolManager struct {
	pools     map[string]*pgxpool.Pool
	poolLocks sync.Mutex
}

func NewConnectionPoolManager() *ConnectionPoolManager {
	return &ConnectionPoolManager{
		pools: make(map[string]*pgxpool.Pool),
	}
}

func (cpm *ConnectionPoolManager) GetConnection(ctx context.Context, tenantID, dsn string) (*pgxpool.Pool, error) {
	cpm.poolLocks.Lock()
	defer cpm.poolLocks.Unlock()

	key := tenantID + ":" + dsn
	if pool, exists := cpm.pools[key]; exists {
		return pool, nil
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	cpm.pools[key] = pool
	log.Info().Str("tenant_id", tenantID).Msg("Created new connection pool")
	return pool, nil
}

func (cpm *ConnectionPoolManager) ReleaseConnection(ctx context.Context, tenantID, dsn string) error {
	cpm.poolLocks.Lock()
	defer cpm.poolLocks.Unlock()

	key := tenantID + ":" + dsn
	if pool, exists := cpm.pools[key]; exists {
		pool.Close()
		delete(cpm.pools, key)
		log.Info().Str("tenant_id", tenantID).Msg("Released connection pool")
		return nil
	}
	return fmt.Errorf("pool not found for tenant %s and dsn %s", tenantID, dsn)
}

func (cpm *ConnectionPoolManager) GetStats(ctx context.Context, tenantID, dsn string) (Stats, error) {
	cpm.poolLocks.Lock()
	defer cpm.poolLocks.Unlock()

	key := tenantID + ":" + dsn
	if pool, exists := cpm.pools[key]; exists {
		stats := pool.Stat()
		return Stats{
			ActiveConnections: stats.AcquiredConns(),
			IdleConnections:   stats.IdleConns(),
			TotalConnections:  stats.TotalConns(),
		}, nil
	}
	return Stats{}, fmt.Errorf("pool not found for tenant %s and dsn %s", tenantID, dsn)
}

type Stats struct {
	ActiveConnections int32
	IdleConnections   int32
	TotalConnections  int32
}
