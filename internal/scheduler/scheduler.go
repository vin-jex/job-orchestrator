package scheduler

import (
	"github.com/Vin-Jex/job-orchestrator/internal/store"
	"github.com/google/uuid"
)

type Scheduler struct {
	id    uuid.UUID
	store *store.Store
}

func New(
	id uuid.UUID,
	storeLayer *store.Store,
) *Scheduler {
	return &Scheduler{
		id:    id,
		store: storeLayer,
	}
}
