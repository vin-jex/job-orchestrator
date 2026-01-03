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
