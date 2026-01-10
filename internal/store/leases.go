package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) AcquireJobLease(
	ctx context.Context,
	schedulerID uuid.UUID,
	leaseDuration time.Duration,
) (jobID uuid.UUID, leaseExpiresAt time.Time, err error) {

	err = s.WithTransaction(ctx, func(transaction pgx.Tx) error {
		row := transaction.QueryRow(ctx, `
			WITH candidate_job AS (
				SELECT id
				FROM jobs
				WHERE state = 'PENDING'
				ORDER BY created_at
				FOR UPDATE SKIP LOCKED
				LIMIT 1
			)
			UPDATE jobs
			SET state = 'SCHEDULED',
				updated_at = now()
			FROM candidate_job
			WHERE jobs.id = candidate_job.id
			RETURNING jobs.id
		`)

		if err := row.Scan(&jobID); err != nil {
			return err
		}

		leaseExpiresAt = time.Now().Add(leaseDuration)

		_, err := transaction.Exec(ctx, `
			INSERT INTO job_leases (
				job_id,
				scheduler_id,
				lease_expires_at
			)
			VALUES ($1, $2, $3)
		`,
			jobID,
			schedulerID,
			leaseExpiresAt,
		)

		return err
	})

	return
}

func (s *Store) RecoverExpiredLeases(
	ctx context.Context,
	now time.Time,
) ([]uuid.UUID, error) {
	rows, err := s.connectionPool.Query(ctx, `
		SELECT
			j.id,
			j.state,
			j.current_attempt,
			j.max_attempts
		FROM job_leases l
		JOIN jobs j ON j.id = l.job_id
		WHERE l.lease_expires_at < $1
			AND j.state NOT IN ('COMPLETED', 'FAILED', 'CANCELLED')
		FOR UPDATE SKIP LOCKED
	`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recovered []uuid.UUID

	for rows.Next() {
		var (
			jobID          uuid.UUID
			state          string
			currentAttempt int
			maxAttempts    int
		)

		if err := rows.Scan(
			&jobID,
			&state,
			&currentAttempt,
			&maxAttempts,
		); err != nil {
			return nil, err
		}

		if err := s.recoverSingleJob(
			ctx,
			jobID,
			state,
			currentAttempt,
			maxAttempts,
		); err != nil {
			return nil, err
		}

		recovered = append(recovered, jobID)
	}

	return recovered, rows.Err()
}

func (s *Store) recoverSingleJob(
	ctx context.Context,
	jobID uuid.UUID,
	state string,
	currentAttempt int,
	maxAttempts int,
) error {
	return s.WithTransaction(ctx, func(transaction pgx.Tx) error {
		switch state {
		case "SCHEDULED":
			if err := transitionJobState(
				ctx,
				transaction,
				jobID,
				"SCHEDULED",
				"PENDING",
			); err != nil {
				return err
			}
		case "RUNNING":
			newAttempt := currentAttempt + 1

			if newAttempt < maxAttempts {
				_, err := transaction.Exec(
					ctx,
					`
					UPDATE jobs
					SET state = 'PENDING',
						current_attempt = $2,
						updated_at = now()
					WHERE id = $1
					`,
					jobID,
					newAttempt,
				)
				if err != nil {
					return err
				}
			} else {
				_, err := transaction.Exec(ctx,
					`
					UPDATE jobs
					SET state = 'FAILED',
						current_attempt = $2,
						updated_at = now()
					WHERE id = $1
				`,
					jobID,
					newAttempt,
				)
				if err != nil {
					return err
				}
			}
		case "COMPLETED", "FAILED", "CANCELLED":
			// no state transition
		default:
			// Terminal or unexpected state => no-op
		}

		// Always delete lease
		_, err := transaction.Exec(ctx,
			`
		DELETE FROM job_leases
		WHERE job_id = $1
		`,
			jobID,
		)
		return err
	})
}
