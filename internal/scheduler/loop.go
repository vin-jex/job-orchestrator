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
			_ = s.store.RecoverExpiredLeases(ctx, time.Now())
		}
	}
}

func (s *Scheduler) tryScheduleOnce(ctx context.Context) {
	_, _, err := s.store.AcquireJobLease(
		ctx,
		s.id,
		30*time.Second,
	)
	if err != nil {
		// No job acquired is NOT an error condition
		return
	}

	// Lease acquired successfully.
	// At this stage we do nothing else.
	// Workers will pick this up later.
}
