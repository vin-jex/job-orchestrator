package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Job struct {
	ID             uuid.UUID
	State          string
	Payload        []byte
	MaxAttempts    int
	CurrentAttempt int
	TimeoutSeconds int
	LastError      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CancelledAt    *time.Time
}

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

func (s *Store) CancelJob(
	ctx context.Context,
	jobID uuid.UUID,
) error {
	return s.WithTransaction(ctx, func(transaction pgx.Tx) error {
		commandTag, err := transaction.Exec(
			ctx,
			`
			UPDATE jobs
			SET state = 'CANCELLED',
				cancelled_at = now(),
				updated_at = now()
			WHERE id = $1
				AND state NOT IN ('COMPLETED', 'CANCELLED')
			`,
			jobID,
		)

		if err != nil {
			return err
		}

		if commandTag.RowsAffected() == 0 {
			return ErrInvalidStateTransition
		}

		return nil
	})
}

func (s *Store) GetJobByID(
	ctx context.Context,
	jobID uuid.UUID,
) (*Job, error) {
	row := s.connectionPool.QueryRow(
		ctx,
		`
			SELECT
				id,
				state,
				payload,
				max_attempts,
				current_attempt,
				timeout_seconds,
				last_error,
				created_at,
				updated_at,
				cancelled_at
			FROM jobs
			WHERE id = $1
		`,
		jobID,
	)
	var job Job

	err := row.Scan(
		&job.ID,
		&job.State,
		&job.Payload,
		&job.MaxAttempts,
		&job.CurrentAttempt,
		&job.TimeoutSeconds,
		&job.LastError,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.CancelledAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	return &job, nil
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
