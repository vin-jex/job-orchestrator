package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// IMPORTANT:
// All job state transitions MUST go through transitionJobState.
// Any direct UPDATE of jobs.state outside this gate is a correctness bug.

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
	return s.WithTransaction(ctx, func(tx pgx.Tx) error {
		var state string

		if err := tx.QueryRow(
			ctx,
			`SELECT state FROM jobs WHERE id = $1 FOR UPDATE`,
			jobID,
		).Scan(&state); err != nil {
			return err
		}

		if err := transitionJobState(ctx, tx, jobID, JobCancelled, state); err != nil {
			return err
		}

		_, err := tx.Exec(
			ctx,
			`UPDATE jobs SET cancelled_at = now() WHERE id = $1`,
			jobID,
		)
		return err
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
	to string,
	from string,
) error {
	if err := ValidateJobTransition(from, to); err != nil {
		return err
	}
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
		to,
		from,
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

func (s *Store) MarkJobRunning(
	ctx context.Context,
	jobID uuid.UUID,
) error {
	return s.WithTransaction(ctx, func(transaction pgx.Tx) error {
		var expiresAt time.Time

		err := transaction.QueryRow(
			ctx,
			`
			SELECT lease_expires_at
			FROM job_leases
			WHERE job_id = $1
			FOR UPDATE
			`,
			jobID,
		).Scan(&expiresAt)
		if err != nil {
			return err
		}

		if time.Now().After(expiresAt) {
			return ErrInvalidStateTransition
		}

		return transitionJobState(ctx, transaction, jobID, JobScheduled, JobRunning)
	})
}

func (s *Store) MarkJobCompleted(ctx context.Context, jobID uuid.UUID) error {
	return s.WithTransaction(ctx, func(tx pgx.Tx) error {
		return transitionJobState(ctx, tx, jobID, JobRunning, JobCompleted)
	})
}

func (s *Store) MarkJobFailed(
	ctx context.Context,
	jobID uuid.UUID,
	errMessage string,
	retryable bool,
) error {
	return s.WithTransaction(ctx, func(tx pgx.Tx) error {
		if err := transitionJobState(ctx, tx, jobID, JobRunning, JobFailed); err != nil {
			return err
		}

		_, err := tx.Exec(
			ctx,
			`
			UPDATE jobs
			SET last_error = $2,
			    retryable = $3
			WHERE id = $1
			`,
			jobID,
			errMessage,
			retryable,
		)
		return err
	})
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

func (s *Store) RetryJobIfAllowed(
	ctx context.Context,
	tx pgx.Tx,
	jobID uuid.UUID,
	currentAttempt int,
	maxAttempts int,
	retryable bool,
) error {
	if !retryable {
		return nil
	}

	nextAttempt := currentAttempt + 1

	if nextAttempt >= maxAttempts {
		return nil
	}

	if err := transitionJobState(ctx, tx, jobID, JobFailed, JobPending); err != nil {
		return err
	}

	_, err := tx.Exec(
		ctx,
		`
		UPDATE jobs
		SET current_attempt = $2
		WHERE id = $1
		`,
		jobID,
		nextAttempt,
	)
	return err
}

func (s *Store) ListJobs(
	ctx context.Context,
	state *string,
	limit int,
) ([]Job, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows pgx.Rows
	var err error

	if state != nil {
		rows, err = s.connectionPool.Query(
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
			WHERE state = $1
			ORDER BY created_at DESC
			LIMIT $2
			`,
			*state,
			limit,
		)
	} else {
		rows, err = s.connectionPool.Query(
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
			ORDER BY created_at DESC
			LIMIT $1
			`,
			limit,
		)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job

	for rows.Next() {
		var job Job
		if err := rows.Scan(
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
		); err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}
