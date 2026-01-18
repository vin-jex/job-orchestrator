package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCancelPendingJob(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	jobID := uuid.New()
	if err := store.CreateJob(ctx, jobID, []byte(`{}`), 3, 30); err != nil {
		t.Fatal(err)
	}

	if err := store.CancelJob(ctx, jobID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := store.CancelJob(ctx, jobID); !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestCancelCompletedJobFails(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	jobID := uuid.New()
	if err := store.CreateJob(ctx, jobID, []byte(`{}`), 3, 30); err != nil {
		t.Fatal(err)
	}

	err := store.WithTransaction(ctx, func(tx pgx.Tx) error {
		if err := transitionJobState(ctx, tx, jobID, JobPending, JobScheduled); err != nil {
			return err
		}
		if err := transitionJobState(ctx, tx, jobID, JobScheduled, JobRunning); err != nil {
			return err
		}
		return transitionJobState(ctx, tx, jobID, JobRunning, JobCompleted)
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.CancelJob(ctx, jobID); !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestFailedJobCannotRun(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	jobID := uuid.New()
	if err := store.CreateJob(ctx, jobID, []byte(`{}`), 3, 30); err != nil {
		t.Fatal(err)
	}

	err := store.WithTransaction(ctx, func(tx pgx.Tx) error {
		if err := transitionJobState(ctx, tx, jobID, JobPending, JobScheduled); err != nil {
			return err
		}
		if err := transitionJobState(ctx, tx, jobID, JobScheduled, JobRunning); err != nil {
			return err
		}
		return transitionJobState(ctx, tx, jobID, JobRunning, JobFailed)
	})
	if err != nil {
		t.Fatal(err)
	}

	err = store.WithTransaction(ctx, func(tx pgx.Tx) error {
		return transitionJobState(ctx, tx, jobID, JobFailed, JobRunning)
	})

	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}
