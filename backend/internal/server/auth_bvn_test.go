package server

import (
	"strings"
	"testing"
)

// signUp returns a token for a brand new user straight after verify-otp.
func (e *env) signUp(phone string) string {
	e.t.Helper()
	r := e.call("POST", "/api/auth/verify-otp", "", map[string]string{"phone": phone, "code": "123456"})
	if r.Status != 200 {
		e.t.Fatalf("verify-otp: %d %v", r.Status, r.Body)
	}
	return r.Body["access_token"].(string)
}

func TestBVN(t *testing.T) {
	e := newEnv(t)
	tok := e.signUp("08070001111")
	tests := []struct {
		name  string
		token string
		body  any
		want  int
		code  string
	}{
		{"no token", "", map[string]string{"bvn": "22212345678"}, 401, "unauthorized"},
		{"bad token", "abc.def", map[string]string{"bvn": "22212345678"}, 401, "invalid_token"},
		{"too short", tok, map[string]string{"bvn": "1234"}, 400, "invalid_bvn"},
		{"letters", tok, map[string]string{"bvn": "2221234567a"}, 400, "invalid_bvn"},
		{"bad json", tok, "[", 400, "invalid_json"},
		{"valid", tok, map[string]string{"bvn": "22212345678"}, 200, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/auth/bvn", tc.token, tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
			if tc.want == 200 {
				if r.Body["bvn_verified"] != true || r.Body["display_name"] == "" {
					t.Fatalf("body: %v", r.Body)
				}
				if strings.Contains(string(mustJSON(r.Body)), "22212345678") {
					t.Fatal("the BVN must never be returned")
				}
			}
		})
	}
}
