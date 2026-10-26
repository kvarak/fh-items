// Package api implements the fh-items HTTP API: GET /api/state (current
// character, supplies, building levels, and catalog items) and POST
// /api/check (affordability check for a selected set of items).
package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"fh-items/internal/catalog"
	"fh-items/internal/state"
)

// Server holds the catalog and current runtime state served by the API.
// State is guarded by a mutex since the HTTP server handles requests
// concurrently. Nothing here is persisted to disk: building levels,
// discovered items, and resources all live in the caller's request/cookie
// rather than server-side storage.
type Server struct {
	Catalog catalog.Catalog

	mu    sync.RWMutex
	state state.State
}

// NewServer builds a Server for the given catalog and initial state.
func NewServer(cat catalog.Catalog, st state.State) *Server {
	return &Server{Catalog: cat, state: st}
}

// State returns a copy of the current runtime state, for callers (such as
// the server-rendered index page) that need it outside the API handlers.
func (s *Server) State() state.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// RegisterRoutes attaches the API routes to mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", s.handleGetState)
	mux.HandleFunc("POST /api/check", s.handlePostCheck)
}

// errorEnvelope is the shared JSON error shape for every API response.
type errorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error errorEnvelope `json:"error"`
	}{errorEnvelope{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
