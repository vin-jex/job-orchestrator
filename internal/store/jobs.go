package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateJob(
	ctx context.Context,
	jobID uuid.UUID,
	jobPayload []byte,
	maxAttempts int,
	executionTimeoutSeconds int,
) error {
	_, err := s.connectionPool.Exec(
		ctx,
		`
		INSERT INTO jobs (
			id,
			state,
			payload,
			max_attempts,
			current_attempt,
			timeout_seconds
		)
		VALUES ($1, 'PENDING', $2, $3, 0, $4)
		`,
		jobID,
		jobPayload,
		maxAttempts,
		executionTimeoutSeconds,
	)

	return err
}

func transitionJobState(
	ctx context.Context,
	transaction pgx.Tx,
	jobID uuid.UUID,
	previousState string,
	nextState string,
) error {
	commandTag, err := transaction.Exec(
		ctx,
		`
			UPDATE jobs
			SET state = $2,
					updated_at = now()
			WHERE id = $1
					AND state = $3
		`,
		jobID,
		nextState,
		previousState,
	)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() != 1 {
		return ErrInvalidStateTransition
	}

	return nil
}
