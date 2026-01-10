package scheduler

import (
	"context"
	"errors"
	"time"
)

var ErrNoJobsAvailable = errors.New("No jobs available")

func (s *Scheduler) Run(ctx context.Context) {
	scheduleTicker := time.NewTicker(500 * time.Millisecond)
	recoveryTicker := time.NewTicker(2 * time.Second)
	defer scheduleTicker.Stop()
	defer recoveryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-scheduleTicker.C:
			s.tryScheduleOnce(ctx)
		case <-recoveryTicker.C:
			recovered, err := s.store.RecoverExpiredLeases(ctx, time.Now())
			if err != nil {
				s.logger.Error("lease recovery failed", "error", err)
			}

			for _, jobID := range recovered {
				s.logger.Info("expired lease recovered", "job_id", jobID.String())
			}
		}
	}
}

func (s *Scheduler) tryScheduleOnce(ctx context.Context) {
	jobID, _, err := s.store.AcquireJobLease(
		ctx,
		s.id,
		30*time.Second,
	)
	if err != nil {
		// No job acquired is NOT an error condition
		return
	}

	s.logger.Info("lease acquired", "job_id", jobID.String())

	// Lease acquired successfully.
	// At this stage we do nothing else.
	// Workers will pick this up later.
}
