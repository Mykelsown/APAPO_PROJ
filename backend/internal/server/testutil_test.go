package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mykelsown/apapo-backend/internal/config"
)

type env struct {
	t *testing.T
	s *Server
	h http.Handler
}

type reply struct {
	Status int
	Body   map[string]any
	Header http.Header
}

func newEnv(t *testing.T) *env {
	t.Helper()
	cfg, err := config.Load(func(k string) (string, bool) {
		return map[string]string{"APP_SECRET": "test-secret", "ALLOWED_ORIGIN": "*"}[k], k == "APP_SECRET" || k == "ALLOWED_ORIGIN"
	}, "test")
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg)
	s.world.Async = false
	return &env{t: t, s: s, h: s.Handler()}
}

// call sends a request. body may be nil, a string (raw) or any JSON-able value.
func (e *env) call(method, path, token string, body any, hdr ...string) reply {
	e.t.Helper()
	var buf bytes.Buffer
	switch b := body.(type) {
	case nil:
	case string:
		buf.WriteString(b)
	default:
		if err := json.NewEncoder(&buf).Encode(b); err != nil {
			e.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.RemoteAddr = "203.0.113.9:4000"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rr := httptest.NewRecorder()
	e.h.ServeHTTP(rr, req)
	out := map[string]any{}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return reply{Status: rr.Code, Body: out, Header: rr.Header()}
}

// login signs in with phone and PIN and returns the token.
func (e *env) login(phone, pin string) string {
	e.t.Helper()
	r := e.call("POST", "/api/auth/login", "", map[string]string{"phone": phone, "pin": pin})
	if r.Status != 200 {
		e.t.Fatalf("login failed: %d %v", r.Status, r.Body)
	}
	return r.Body["access_token"].(string)
}

func (e *env) sam() string  { return e.login("08012345678", "4821") }
func (e *env) bola() string { return e.login("08098765432", "5912") }

func errCode(r reply) string {
	if m, ok := r.Body["error"].(map[string]any); ok {
		s, _ := m["code"].(string)
		return s
	}
	return ""
}

func (e *env) advance(d time.Duration) {
	now := time.Now().UTC().Add(d)
	e.s.world.SetClock(func() time.Time { return now })
}

func get(m map[string]any, path ...string) any {
	var cur any = m
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[p]
	}
	return cur
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
