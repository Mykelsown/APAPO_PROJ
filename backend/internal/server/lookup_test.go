package server

import (
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	halfway := e.signUp("08070009999") // signed up but no BVN or PIN
	tests := []struct {
		name  string
		token string
		query string
		want  int
		code  string
	}{
		{"finds Bola, masked", sam, "08098765432", 200, ""},
		{"+234 format", sam, "%2B2348098765432", 200, ""},
		{"self", sam, "08012345678", 400, "self_send"},
		{"bad phone", sam, "12", 400, "invalid_phone"},
		{"missing phone", sam, "", 400, "invalid_phone"},
		{"unknown", sam, "08011110000", 404, "not_found"},
		{"user who has not finished onboarding looks unknown", sam, "08070009999", 404, "not_found"},
		{"no token", "", "08098765432", 401, "unauthorized"},
	}
	_ = halfway
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("GET", "/api/users/lookup?phone="+tc.query, tc.token, nil)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
			if tc.want == 200 {
				if get(r.Body, "user", "display_name") != "Bola A." || get(r.Body, "user", "recipient_type") != "apapo" {
					t.Fatalf("body: %v", r.Body)
				}
				raw := string(mustJSON(r.Body))
				for _, leak := range []string{"Adeyemi", "2009009009", "acc_", "08098765432", "+2348098765432"} {
					if strings.Contains(raw, leak) {
						t.Fatalf("lookup leaked %q: %s", leak, raw)
					}
				}
			}
		})
	}
	t.Run("rate limited", func(t *testing.T) {
		var last reply
		for i := 0; i < 32; i++ {
			last = e.call("GET", "/api/users/lookup?phone=08011110000", sam, nil)
		}
		if last.Status != 429 || last.Header.Get("Retry-After") == "" {
			t.Fatalf("got %d", last.Status)
		}
	})
}
