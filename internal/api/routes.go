package api

func (s *Server) registerRoutes() {
	// Public
	s.mux.HandleFunc("POST /v1/jobs", s.handleCreateJob)
	s.mux.HandleFunc("GET /v1/jobs", s.handleListJobs)
	s.mux.HandleFunc("GET /v1/jobs/{jobID}", s.handleGetJob)
	s.mux.HandleFunc("POST /v1/jobs/{jobID}/cancel", s.handleCancelJob)

	// Internal - Scheduler
	s.mux.HandleFunc("POST /internal/jobs/lease", s.handleAcquireLease)
	s.mux.HandleFunc("POST /internal/jobs/recover", s.handleRecoverLeases)

	// // Internal - Worker
	// s.mux.HandleFunc("POST /internal/jobs/{jobID}/start", s.handleStartJob)
	// s.mux.HandleFunc("POST /internal/jobs/{jobID}/complete", s.handleCompleteJob)
	// s.mux.HandleFunc("POST /internal/jobs/{jobID}/fail", s.handleFailJob)
	// s.mux.HandleFunc("POST /internal/workers/{workerID}/heartbeat", s.handleWorkerHeartbeat)
}
