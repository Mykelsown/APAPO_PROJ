// Package server wires routes and middleware.
package server

import (
	"net/http"
	"time"

	"github.com/Mykelsown/apapo-backend/internal/config"
	"github.com/Mykelsown/apapo-backend/internal/httpx"
)

// Server holds shared dependencies for handlers.
type Server struct {
	cfg config.Config
	now func() time.Time
}

// New builds a Server.
func New(cfg config.Config) *Server {
	return &Server{cfg: cfg, now: func() time.Time { return time.Now().UTC() }}
}

// Handler returns the full HTTP handler with middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	cors := httpx.NewCORS(s.cfg.AllowedOrigins, s.cfg.AllowAnyOrigin)
	return httpx.Recover(httpx.Logger(cors.Wrap(jsonErrors(mux))))
}

// jsonErrors converts the mux's plain text 404 and 405 into the standard JSON error shape.
func jsonErrors(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern == "" {
			rr := &probe{header: http.Header{}}
			h.ServeHTTP(rr, r)
			if rr.status == http.StatusMethodNotAllowed {
				if a := rr.header.Get("Allow"); a != "" {
					w.Header().Set("Allow", a)
				}
				httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "This method is not allowed on this path.", nil)
				return
			}
			httpx.WriteError(w, http.StatusNotFound, "not_found", "This endpoint does not exist.", nil)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// probe captures the status of the mux's fallback handler without writing a response.
type probe struct {
	header http.Header
	status int
}

func (p *probe) Header() http.Header         { return p.header }
func (p *probe) Write(b []byte) (int, error) { return len(b), nil }
func (p *probe) WriteHeader(code int)        { p.status = code }
