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
				AND state NOT IN ('COMPLETED', 'FAILED', 'CANCELLED')
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

func (s *Store) AcquireScheduledJobForWorker(
	ctx context.Context,
	workerID uuid.UUID,
) (uuid.UUID, []byte, error) {
	var (
		jobID   uuid.UUID
		payload []byte
	)

	err := s.WithTransaction(ctx, func(transaction pgx.Tx) error {
		row := transaction.QueryRow(ctx, `
			SELECT j.id, j.payload
			FROM jobs j
			JOIN job_leases l ON l.job_id = j.id
			WHERE j.state = 'SCHEDULED'
			ORDER BY j.created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		`)

		if err := row.Scan(&jobID, &payload); err != nil {
			return err
		}

		if err := transitionJobState(
			ctx,
			transaction,
			jobID,
			"SCHEDULED",
			"RUNNING",
		); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return uuid.Nil, nil, err
	}

	return jobID, payload, nil
}

func (s *Store) MarkJobCompleted(
	ctx context.Context,
	jobID uuid.UUID,
) error {
	_, err := s.connectionPool.Exec(
		ctx,
		`
			UPDATE jobs
			SET state = 'COMPLETED',
				updated_at = now()
			WHERE id = $1
				AND state = 'RUNNING'
		`,
		jobID,
	)
	return err
}

func (s *Store) MarkJobFailed(
	ctx context.Context,
	jobID uuid.UUID,
	errMessage string,
) error {
	_, err := s.connectionPool.Exec(
		ctx,
		`
			UPDATE jobs
			SET state = 'FAILED',
				last_error = $2,
				updated_at = now()
			WHERE id = $1
				AND state = 'RUNNING'
		`,
		jobID,
		errMessage,
	)
	return err
}

func (s *Store) IsJobCancelled(
	ctx context.Context,
	jobID uuid.UUID,
) (bool, error) {
	var state string

	err := s.connectionPool.QueryRow(
		ctx,
		`
			SELECT state
			FROM jobs
			WHERE id = $1
			`,
		jobID,
	).Scan(&state)

	if err != nil {
		return false, err
	}

	return state == "CANCELLED", nil
}

func (s *Store) CountRunningJobs(ctx context.Context) (int, error) {
	row := s.connectionPool.QueryRow(
		ctx,
		`
			SELECT COUNT(*)
			FROM jobs
			WHERE state = 'RUNNING'
		`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}
