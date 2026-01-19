package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/vin-jex/job-orchestrator/internal/api"
	"github.com/vin-jex/job-orchestrator/internal/observability"
	"github.com/vin-jex/job-orchestrator/internal/store"

	_ "github.com/vin-jex/job-orchestrator/docs"
)

// @title Distributed Job Orchestrator API
// @version 1.0
// @description Correctness-first distributed job orchestration control plane.
// @termsOfService https://example.com/terms

// @contact.name Okereke Vincent
// @contact.url https://github.com/vin-jex
// @contact.email vincentcode0@gmail.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @BasePath /
func main() {
	_ = godotenv.Load()

	logger := observability.NewLogger("control-plane")

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

	server := api.NewServer(storeLayer, logger)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      server.Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Info(
		"control-plane started",
		"addr", httpServer.Addr,
		"docs", "/swagger/index.html",
	)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(shutdownCtx)
}
