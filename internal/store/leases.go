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
) (uuid.UUID, time.Time, error) {
	var jobID uuid.UUID
	leaseExpiresAt := time.Now().Add(leaseDuration)

	err := s.WithTransaction(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(
			ctx,
			`
			SELECT id
			FROM jobs
			WHERE state = 'PENDING'
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
			`,
		).Scan(&jobID); err != nil {
			return err
		}

		if err := transitionJobState(
			ctx,
			tx,
			jobID,
			"PENDING",
			"SCHEDULED",
		); err != nil {
			return err
		}

		_, err := tx.Exec(
			ctx,
			`
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

	if err != nil {
		return uuid.Nil, time.Time{}, err
	}

	return jobID, leaseExpiresAt, nil
}

func (s *Store) RecoverExpiredLeases(
	ctx context.Context,
	now time.Time,
) ([]uuid.UUID, error) {
	var recovered []uuid.UUID

	err := s.WithTransaction(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				j.id,
				j.state,
				j.current_attempt,
				j.max_attempts,
				j.retryable
			FROM job_leases l
			JOIN jobs j ON j.id = l.job_id
			WHERE l.lease_expires_at < $1
			  AND j.state NOT IN ('COMPLETED', 'FAILED', 'CANCELLED')
			FOR UPDATE SKIP LOCKED
		`, now)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				jobID          uuid.UUID
				state          string
				currentAttempt int
				maxAttempts    int
				retryable      bool
			)

			if err := rows.Scan(
				&jobID,
				&state,
				&currentAttempt,
				&maxAttempts,
				&retryable,
			); err != nil {
				return err
			}

			if err := s.recoverSingleJob(
				ctx,
				tx,
				jobID,
				state,
				currentAttempt,
				maxAttempts,
				retryable,
			); err != nil {
				return err
			}

			recovered = append(recovered, jobID)
		}

		return rows.Err()
	})

	return recovered, err
}

func (s *Store) recoverSingleJob(
	ctx context.Context,
	tx pgx.Tx,
	jobID uuid.UUID,
	state string,
	currentAttempt int,
	maxAttempts int,
	retryable bool,
) error {
	switch state {
	case "SCHEDULED":
		if err := transitionJobState(ctx, tx, jobID, "SCHEDULED", "PENDING"); err != nil {
			return err
		}
	case "RUNNING":
		if err := s.RetryJobIfAllowed(
			ctx,
			tx,
			jobID,
			currentAttempt,
			maxAttempts,
			retryable,
		); err != nil {
			return err
		}
	}

	_, err := tx.Exec(ctx, `
		DELETE FROM job_leases
		WHERE job_id = $1
	`, jobID)

	return err
}
