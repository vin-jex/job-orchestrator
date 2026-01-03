package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type TransactionFunc func(transaction pgx.Tx) error

func (s *Store) WithTransaction(
	ctx context.Context,
	fn TransactionFunc,
) error {
	transaction, err := s.connectionPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	defer transaction.Rollback(ctx)

	if err := fn(transaction); err != nil {
		return err
	}

	return transaction.Commit(ctx)
}
