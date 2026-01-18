package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := NewStore(context.Background(), testDatabaseURL())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestCancelPendingJob(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	jobID := uuid.New()
	err := store.CreateJob(ctx, jobID, []byte(`{}`), 3, 30)
	if err != nil {
		t.Fatal(err)
	}

	_ = store.CancelJob(ctx, jobID)
	err = store.CancelJob(ctx, jobID)
	if err != ErrInvalidStateTransition {
		t.Fatalf("expected ErrInvalidStateTransition, got  %v", err)
	}

	job, err := store.GetJobByID(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if job.State != "CANCELLED" {
		t.Fatalf("expected CANCELLED, got %s", job.State)
	}
}

func TestCancelCompletedJobFails(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	jobID := uuid.New()

	err := store.CreateJob(ctx, jobID, []byte(`{}`), 3, 30)
	if err != nil {
		t.Fatal(err)
	}

	err = store.WithTransaction(ctx, func(tx pgx.Tx) error {
		if err := transitionJobState(ctx, tx, jobID, "PENDING", "SCHEDULED"); err != nil {
			return err
		}
		if err := transitionJobState(ctx, tx, jobID, "SCHEDULED", "RUNNING"); err != nil {
			return err
		}
		return transitionJobState(ctx, tx, jobID, "RUNNING", "COMPLETED")
	})
	if err != nil {
		t.Fatal(err)
	}

	err = store.CancelJob(ctx, jobID)
	if err != ErrInvalidStateTransition {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestFailedJobCannotRun(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	jobID := uuid.New()

	err := store.CreateJob(ctx, jobID, []byte(`{}`), 3, 30)
	if err != nil {
		t.Fatal(err)
	}

	err = store.WithTransaction(ctx, func(transaction pgx.Tx) error {
		if err := transitionJobState(ctx, transaction, jobID, "PENDING", "SCHEDULED"); err != nil {
			return err
		}
		if err := transitionJobState(ctx, transaction, jobID, "SCHEDULED", "RUNNING"); err != nil {
			return err
		}
		return transitionJobState(ctx, transaction, jobID, "RUNNING", "FAILED")
	})
	if err != nil {
		t.Fatal(err)
	}

	err = store.WithTransaction(ctx, func(transaction pgx.Tx) error {
		return transitionJobState(ctx, transaction, jobID, "FAILED", "RUNNING")
	})

	if err != ErrInvalidStateTransition {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}
