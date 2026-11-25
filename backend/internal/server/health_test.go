package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Mykelsown/apapo-backend/internal/config"
)

func testServer(t *testing.T, env map[string]string) http.Handler {
	t.Helper()
	cfg, err := config.Load(func(k string) (string, bool) { v, ok := env[k]; return v, ok }, "test")
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg)
	s.now = func() time.Time { return time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC) }
	return s.Handler()
}

func TestHealthEndpoints(t *testing.T) {
	h := testServer(t, map[string]string{"APP_SECRET": "top-secret-123", "VERSION": "0.1.0"})
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantType   string
		check      func(t *testing.T, body string)
	}{
		{name: "api health", method: "GET", path: "/api/health", wantStatus: 200, wantType: "application/json", check: func(t *testing.T, body string) {
			var got map[string]string
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"status": "ok", "service": "apapo-api", "version": "0.1.0", "provider": "mock", "time": "2026-11-25T12:00:00Z", "message": "Backend is reachable"}
			for k, v := range want {
				if got[k] != v {
					t.Fatalf("%s = %q want %q", k, got[k], v)
				}
			}
			if len(got) != len(want) {
				t.Fatalf("unexpected fields: %v", got)
			}
			if strings.Contains(body, "top-secret-123") || strings.Contains(body, "123456") {
				t.Fatal("health leaks a secret")
			}
		}},
		{name: "healthz", method: "GET", path: "/healthz", wantStatus: 200, wantType: "text/plain", check: func(t *testing.T, body string) {
			if body != "ok" {
				t.Fatalf("body %q", body)
			}
		}},
		{name: "head health works", method: "HEAD", path: "/api/health", wantStatus: 200, wantType: "application/json"},
		{name: "wrong method", method: "POST", path: "/api/health", wantStatus: 405, wantType: "application/json", check: func(t *testing.T, body string) {
			if !strings.Contains(body, `"code":"method_not_allowed"`) {
				t.Fatal(body)
			}
		}},
		{name: "unknown api path", method: "GET", path: "/api/nope", wantStatus: 404, wantType: "application/json", check: func(t *testing.T, body string) {
			if !strings.Contains(body, `"code":"not_found"`) || !strings.Contains(body, `"details":{}`) {
				t.Fatal(body)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest(tt.method, tt.path, nil))
			if rr.Code != tt.wantStatus {
				t.Fatalf("status %d want %d: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, tt.wantType) {
				t.Fatalf("content type %q", ct)
			}
			if tt.check != nil {
				tt.check(t, rr.Body.String())
			}
		})
	}
}

func TestHealthCORS(t *testing.T) {
	h := testServer(t, map[string]string{"ALLOWED_ORIGIN": "https://app.apapo.test"})
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "https://app.apapo.test")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://app.apapo.test" {
		t.Fatal("missing CORS header on health")
	}

	pre := httptest.NewRequest("OPTIONS", "/api/splitpay/execute", nil)
	pre.Header.Set("Origin", "https://app.apapo.test")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, pre)
	if rr.Code != 204 {
		t.Fatalf("preflight status %d", rr.Code)
	}
}
