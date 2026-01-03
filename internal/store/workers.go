package store

import (
	"context"

	"github.com/google/uuid"
)

func (s *Store) UpsertWorkerHeartbeat(
	ctx context.Context,
	workerID uuid.UUID,
	workerCapacity int,
) error {
	_, err := s.connectionPool.Exec(ctx, `
		INSERT INTO workers (
			id,
			last_heartbeat,
			capacity
		)
		VALUES ($1, now(), $2)
		ON CONFLICT (id)
		DO UPDATE
		SET last_heartbeat = now(),
			capacity = EXCLUDED.capacity
	`,
		workerID,
		workerCapacity,
	)

	return err
}
