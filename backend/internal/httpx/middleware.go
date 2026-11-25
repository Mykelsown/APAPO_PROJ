package httpx

import (
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// CORS allows browser calls from the configured origins. With no origins set it adds no
// CORS headers, which suits same origin use or a dev server that proxies /api.
type CORS struct {
	Origins  map[string]bool
	AllowAny bool
}

// NewCORS builds a CORS policy from a list of origins.
func NewCORS(origins []string, allowAny bool) CORS {
	m := make(map[string]bool, len(origins))
	for _, o := range origins {
		m[strings.ToLower(o)] = true
	}
	return CORS{Origins: m, AllowAny: allowAny}
}

func (c CORS) allowed(origin string) bool {
	return c.AllowAny || c.Origins[strings.ToLower(strings.TrimRight(origin, "/"))]
}

// Wrap applies the policy. Preflight from an allowed origin gets 204; from any other
// origin it gets 403. Normal requests from a blocked origin are served without CORS
// headers, so the browser refuses to hand the response to the page.
func (c CORS) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
		if origin == "" {
			if preflight {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if !c.allowed(origin) {
			if preflight {
				WriteError(w, http.StatusForbidden, "origin_not_allowed", "This origin is not allowed to call the API.", nil)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Expose-Headers", "Retry-After")
		if preflight {
			h.Add("Vary", "Access-Control-Request-Method")
			h.Add("Vary", "Access-Control-Request-Headers")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logger logs method, path, status and duration. It never logs query strings, headers or
// bodies, so phone numbers, tokens and PINs stay out of the logs.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" {
			return // health probes every few seconds would flood the log
		}
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

// Recover turns a panic into a 500 with the standard error body.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				log.Printf("panic on %s %s: %v\n%s", r.Method, r.URL.Path, v, debug.Stack())
				WriteError(w, http.StatusInternalServerError, "internal_error", "Something went wrong on our side. Please try again.", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
