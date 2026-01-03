package api

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("POST /v1/jobs", s.handleCreateJob)
	// s.mux.HandleFunc("POST /v1/jobs/{jobID}/cancel", s.handleCancelJob)
	// s.mux.HandleFunc("GET /v1/jobs", s.handleListJobs)
	// s.mux.HandleFunc("GET /v1/jobs/{jobID}", s.handleGetJob)
}
