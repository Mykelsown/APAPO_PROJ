package server

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/Mykelsown/apapo-backend/internal/app"
	"github.com/Mykelsown/apapo-backend/internal/httpx"
)

// result is what a handler returns: a status and a JSON body, or an error.
type result struct {
	status int
	body   any
}

type publicFn func(r *http.Request) (result, error)
type authedFn func(r *http.Request, uid string) (result, error)

func ok(v any) (result, error)       { return result{http.StatusOK, v}, nil }
func created(v any) (result, error)  { return result{http.StatusCreated, v}, nil }
func accepted(v any) (result, error) { return result{http.StatusAccepted, v}, nil }

func (s *Server) respond(w http.ResponseWriter, res result, err error) {
	if err != nil {
		var ae *app.Error
		if errors.As(err, &ae) {
			if ae.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(ae.RetryAfter))
			}
			httpx.WriteError(w, ae.Status, ae.Code, ae.Message, ae.Details)
			return
		}
		panic(err) // Recover turns it into the standard 500
	}
	httpx.WriteJSON(w, res.status, res.body)
}

// public wraps a handler that needs no sign in.
func (s *Server) public(fn publicFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := fn(r)
		s.respond(w, res, err)
	}
}

// authed wraps a handler that needs a valid bearer token.
func (s *Server) authed(fn authedFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := ""
		if h := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
			tok = strings.TrimSpace(h[7:])
		}
		uid, aerr := s.world.Authenticate(tok)
		if aerr != nil {
			s.respond(w, result{}, aerr)
			return
		}
		res, err := fn(r, uid)
		s.respond(w, res, err)
	}
}

// authLimited adds the per IP bucket used by every /api/auth/* route.
func (s *Server) authLimited(fn publicFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if e := s.world.AuthIPLimit(s.clientIP(r)); e != nil {
			s.respond(w, result{}, e)
			return
		}
		res, err := fn(r)
		s.respond(w, res, err)
	}
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
			return strings.TrimSpace(strings.Split(xf, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// decode reads a JSON body. Unknown fields are ignored on purpose: the frontend sends display
// fields the backend does not need. Only the fields a handler reads are validated.
func decode(r *http.Request, v any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return app.Errf(http.StatusBadRequest, "invalid_json", "The request body could not be read.", nil)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return app.Errf(http.StatusBadRequest, "invalid_json", "The request body is not valid JSON.", nil)
	}
	return nil
}

func queryInt(r *http.Request, key string, def, min, max int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || n < min {
		return def
	}
	if n > max {
		return max
	}
	return n
}
