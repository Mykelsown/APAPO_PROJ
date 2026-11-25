package server

import (
	"net/http"
	"time"

	"github.com/Mykelsown/apapo-backend/internal/httpx"
)

type healthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Version  string `json:"version"`
	Provider string `json:"provider"`
	Time     string `json:"time"`
	Message  string `json:"message"`
}

// handleHealth answers GET /api/health for the frontend "Connected" badge.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, healthResponse{
		Status:   "ok",
		Service:  "apapo-api",
		Version:  s.cfg.Version,
		Provider: s.cfg.Provider,
		Time:     s.now().Format(time.RFC3339),
		Message:  "Backend is reachable",
	})
}

// handleHealthz answers GET /healthz with plain text for container health checks.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte("ok"))
}
