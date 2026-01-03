package api

import (
	"net/http"

	"github.com/Vin-Jex/job-orchestrator/internal/store"
)

type Server struct {
	store *store.Store
	mux   *http.ServeMux
}

func NewServer(storeLayer *store.Store) *Server {
	server := &Server{
		store: storeLayer,
		mux:   http.NewServeMux(),
	}

	server.registerRoutes()

	return server
}

func (s *Server) Handler() http.Handler {
	return s.mux
}
