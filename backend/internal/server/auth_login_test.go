package server

import "testing"

func TestLogin(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name string
		body any
		want int
		code string
	}{
		{"seeded user one", map[string]string{"phone": "08012345678", "pin": "4821"}, 200, ""},
		{"seeded user two, +234 format", map[string]string{"phone": "+2348098765432", "pin": "5912"}, 200, ""},
		{"wrong pin", map[string]string{"phone": "08012345678", "pin": "0000"}, 401, "invalid_credentials"},
		{"unknown phone looks the same", map[string]string{"phone": "08011112222", "pin": "4821"}, 401, "invalid_credentials"},
		{"bad phone", map[string]string{"phone": "nope", "pin": "4821"}, 401, "invalid_credentials"},
		{"bad json", "{", 400, "invalid_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/auth/login", "", tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
			if tc.want == 200 && get(r.Body, "onboarding", "complete") != true {
				t.Fatalf("seeded user not complete: %v", r.Body)
			}
		})
	}
}

func TestLoginWithoutPINYet(t *testing.T) {
	e := newEnv(t)
	e.signUp("08070004444")
	r := e.call("POST", "/api/auth/login", "", map[string]string{"phone": "08070004444", "pin": "4821"})
	if r.Status != 401 || errCode(r) != "invalid_credentials" {
		t.Fatalf("got %d %v", r.Status, r.Body)
	}
}

func TestLoginLocksAfterFiveWrongPINs(t *testing.T) {
	e := newEnv(t)
	bad := map[string]string{"phone": "08098765432", "pin": "0001"}
	for i := 0; i < 4; i++ {
		if r := e.call("POST", "/api/auth/login", "", bad); r.Status != 401 {
			t.Fatalf("try %d: %d", i, r.Status)
		}
		// keep each call on a fresh IP bucket window
		e.advance(time_minute)
	}
	r := e.call("POST", "/api/auth/login", "", bad)
	if r.Status != 423 || errCode(r) != "pin_locked" || r.Header.Get("Retry-After") == "" {
		t.Fatalf("5th: %d %v", r.Status, r.Body)
	}
	if r.Body["error"].(map[string]any)["details"].(map[string]any)["retry_after_seconds"] == nil {
		t.Fatal("seconds left missing")
	}
	// The right PIN is also refused while locked, and works after 15 minutes.
	good := map[string]string{"phone": "08098765432", "pin": "5912"}
	if r := e.call("POST", "/api/auth/login", "", good); r.Status != 423 {
		t.Fatalf("locked login: %d", r.Status)
	}
	e.advance(16 * time_minute)
	if r := e.call("POST", "/api/auth/login", "", good); r.Status != 200 {
		t.Fatalf("after lock: %d %v", r.Status, r.Body)
	}
}
