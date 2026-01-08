package worker

import (
	"context"
	"time"

	"github.com/Vin-Jex/job-orchestrator/internal/store"
	"github.com/google/uuid"
)

type Worker struct {
	id       uuid.UUID
	capacity int
	store    *store.Store
}

func New(id uuid.UUID, capacity int, storeLayer *store.Store) *Worker {
	return &Worker{
		id:       id,
		capacity: capacity,
		store:    storeLayer,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.store.RegisterWorker(ctx, w.id, w.capacity); err != nil {
		return err
	}

	go w.runExecutor(ctx)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			if err := w.store.HeartbeatWorker(ctx, w.id); err != nil {
				return err
			}
		}
	}
}

func (w *Worker) runExecutor(ctx context.Context) {
	semaphore := make(chan struct{}, w.capacity)

	for {
		select {
		case <-ctx.Done():
			return
		case semaphore <- struct{}{}:
			go func() {
				defer func() { <-semaphore }()

				jobID, payload, err := w.store.AcquireScheduledJobForWorker(ctx, w.id)
				if err != nil {
					time.Sleep(300 * time.Millisecond)
					return
				}

				jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				_ = payload
				time.Sleep(1 * time.Second)

				_ = w.store.MarkJobCompleted(jobCtx, jobID)

			}()
		}
	}
}
