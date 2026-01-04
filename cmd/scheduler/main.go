package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Vin-Jex/job-orchestrator/internal/scheduler"
	"github.com/Vin-Jex/job-orchestrator/internal/store"
	"github.com/google/uuid"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	storeLayer, err := store.NewStore(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer storeLayer.Close()

	schedulerID := uuid.New()

	s := scheduler.New(
		schedulerID,
		storeLayer,
	)

	go s.Run(ctx)
	<-ctx.Done()

}
