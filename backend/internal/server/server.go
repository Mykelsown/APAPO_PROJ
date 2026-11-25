// Package server wires routes and middleware.
package server

import (
	"net/http"
	"time"

	"github.com/Mykelsown/apapo-backend/internal/app"
	"github.com/Mykelsown/apapo-backend/internal/config"
	"github.com/Mykelsown/apapo-backend/internal/httpx"
)

// Server holds shared dependencies for handlers.
type Server struct {
	cfg   config.Config
	now   func() time.Time
	world *app.World
}

// New builds a Server.
func New(cfg config.Config) *Server {
	w := app.New(app.Config{Secret: cfg.AppSecret, DevOTP: cfg.DevOTP, DemoEnabled: cfg.DemoEnabled})
	return &Server{cfg: cfg, now: func() time.Time { return time.Now().UTC() }, world: w}
}

// World exposes the domain, for the background workers and tests.
func (s *Server) World() *app.World { return s.world }

// Handler returns the full HTTP handler with middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.routes(mux)

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
