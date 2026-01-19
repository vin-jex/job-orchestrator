package api

import (
	"net/http"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

func (s *Server) registerRoutes() {
	r := mux.NewRouter()

	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	r.Handle("/metrics", s.handleMetrics()).Methods(http.MethodGet)
	r.HandleFunc("/healthz", s.handleHealth).Methods(http.MethodGet)
	r.HandleFunc("/readyz", s.handleReady).Methods(http.MethodGet)

	r.HandleFunc("/v1/jobs", s.handleCreateJob).Methods(http.MethodPost)
	r.HandleFunc("/v1/jobs", s.handleListJobs).Methods(http.MethodGet)
	r.HandleFunc("/v1/jobs/{jobID}", s.handleGetJob).Methods(http.MethodGet)
	r.HandleFunc("/v1/jobs/{jobID}/cancel", s.handleCancelJob).Methods(http.MethodPost)

	r.HandleFunc("/internal/jobs/lease", s.handleAcquireLease).Methods(http.MethodPost)
	r.HandleFunc("/internal/jobs/recover", s.handleRecoverLeases).Methods(http.MethodPost)

	r.HandleFunc("/internal/jobs/{jobID}/start", s.handleStartJob).Methods(http.MethodPost)
	r.HandleFunc("/internal/jobs/{jobID}/complete", s.handleCompleteJob).Methods(http.MethodPost)
	r.HandleFunc("/internal/jobs/{jobID}/fail", s.handleFailJob).Methods(http.MethodPost)

	r.HandleFunc("/internal/workers/{workerID}/heartbeat", s.handleWorkerHeartbeat).Methods(http.MethodPost)

	s.mux = r
}
