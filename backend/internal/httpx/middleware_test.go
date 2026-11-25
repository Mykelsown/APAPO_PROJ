package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
}

func TestCORS(t *testing.T) {
	allowed := "https://app.apapo.test"
	tests := []struct {
		name       string
		cors       CORS
		method     string
		origin     string
		preflight  bool
		wantStatus int
		wantACAO   string
		wantBody   string
	}{
		{name: "allowed origin", cors: NewCORS([]string{allowed}, false), method: "GET", origin: allowed, wantStatus: 200, wantACAO: allowed, wantBody: "ok"},
		{name: "allowed origin case and slash", cors: NewCORS([]string{allowed}, false), method: "GET", origin: "https://APP.apapo.test/", wantStatus: 200, wantACAO: "https://APP.apapo.test/"},
		{name: "blocked origin gets no header", cors: NewCORS([]string{allowed}, false), method: "GET", origin: "https://evil.test", wantStatus: 200, wantACAO: ""},
		{name: "no origin passes through", cors: NewCORS(nil, false), method: "GET", wantStatus: 200, wantACAO: "", wantBody: "ok"},
		{name: "preflight allowed", cors: NewCORS([]string{allowed}, false), method: "OPTIONS", origin: allowed, preflight: true, wantStatus: 204, wantACAO: allowed},
		{name: "preflight blocked", cors: NewCORS([]string{allowed}, false), method: "OPTIONS", origin: "https://evil.test", preflight: true, wantStatus: 403, wantBody: "origin_not_allowed"},
		{name: "preflight no origin", cors: NewCORS(nil, false), method: "OPTIONS", preflight: true, wantStatus: 204},
		{name: "star echoes origin", cors: NewCORS(nil, true), method: "GET", origin: "https://any.test", wantStatus: 200, wantACAO: "https://any.test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/health", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.preflight {
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "authorization, content-type, idempotency-key")
			}
			rr := httptest.NewRecorder()
			tt.cors.Wrap(okHandler()).ServeHTTP(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("status %d want %d", rr.Code, tt.wantStatus)
			}
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != tt.wantACAO {
				t.Fatalf("ACAO %q want %q", got, tt.wantACAO)
			}
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got == "*" {
				t.Fatal("must never send *")
			}
			if !strings.Contains(strings.Join(rr.Header().Values("Vary"), ","), "Origin") {
				t.Fatal("missing Vary: Origin")
			}
			if tt.wantBody != "" && !strings.Contains(rr.Body.String(), tt.wantBody) {
				t.Fatalf("body %q missing %q", rr.Body.String(), tt.wantBody)
			}
			if tt.preflight && tt.wantStatus == 204 && tt.origin != "" {
				if m := rr.Header().Get("Access-Control-Allow-Methods"); m != "GET, POST, PUT, DELETE, OPTIONS" {
					t.Fatalf("methods %q", m)
				}
				if h := rr.Header().Get("Access-Control-Allow-Headers"); h != "Authorization, Content-Type, Idempotency-Key" {
					t.Fatalf("headers %q", h)
				}
			}
		})
	}
}

func TestRecover(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/x", nil))
	if rr.Code != 500 || !strings.Contains(rr.Body.String(), `"internal_error"`) {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
}
