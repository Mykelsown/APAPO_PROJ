package server

import (
	"strings"
	"testing"
)

func TestMe(t *testing.T) {
	e := newEnv(t)
	t.Run("seeded user", func(t *testing.T) {
		r := e.call("GET", "/api/me", e.sam(), nil)
		if r.Status != 200 {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		if get(r.Body, "user", "display_name") != "Samuel Pelumi" || get(r.Body, "user", "phone") != "+2348012345678" {
			t.Fatalf("user: %v", r.Body["user"])
		}
		if get(r.Body, "onboarding", "complete") != true {
			t.Fatalf("onboarding: %v", r.Body["onboarding"])
		}
		if get(r.Body, "receiving_account", "bank_code") != "apex" || !strings.HasPrefix(get(r.Body, "receiving_account", "masked_account_number").(string), "******") {
			t.Fatalf("receiving: %v", r.Body["receiving_account"])
		}
	})
	t.Run("new user has no receiving account", func(t *testing.T) {
		r := e.call("GET", "/api/me", e.signUp("08070005555"), nil)
		if r.Status != 200 || r.Body["receiving_account"] != nil || get(r.Body, "onboarding", "complete") != false {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
	t.Run("auth", func(t *testing.T) {
		if r := e.call("GET", "/api/me", "", nil); r.Status != 401 || errCode(r) != "unauthorized" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		if r := e.call("GET", "/api/me", "garbage", nil); r.Status != 401 || errCode(r) != "invalid_token" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
}

func TestProfile(t *testing.T) {
	e := newEnv(t)
	tok := e.bola()
	tests := []struct {
		name  string
		body  any
		want  int
		code  string
		name2 string
	}{
		{"valid", map[string]string{"display_name": "  Bola   A. Adeyemi "}, 200, "", "Bola A. Adeyemi"},
		{"empty", map[string]string{"display_name": "   "}, 400, "invalid_display_name", ""},
		{"too long", map[string]string{"display_name": strings.Repeat("a", 81)}, 400, "invalid_display_name", ""},
		{"bad json", "nope", 400, "invalid_json", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("PUT", "/api/me/profile", tok, tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
			if tc.want == 200 && get(r.Body, "user", "display_name") != tc.name2 {
				t.Fatalf("name: %v", r.Body)
			}
		})
	}
	if r := e.call("PUT", "/api/me/profile", "", map[string]string{"display_name": "x"}); r.Status != 401 {
		t.Fatalf("no token: %d", r.Status)
	}
}
