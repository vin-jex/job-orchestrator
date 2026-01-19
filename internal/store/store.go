package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// IMPORTANT:
// All job state transitions MUST go through transitionJobState.
// Any direct UPDATE of jobs.state outside this gate is a correctness bug.

type Store struct {
	connectionPool *pgxpool.Pool
}

func NewStore(context context.Context, databaseUrl string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseUrl)

	if err != nil {
		return nil, err
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(context, config)
	if err != nil {
		return nil, err
	}

	return &Store{connectionPool: pool}, nil
}

func (s *Store) Close() {
	s.connectionPool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.connectionPool.Ping(ctx)
}