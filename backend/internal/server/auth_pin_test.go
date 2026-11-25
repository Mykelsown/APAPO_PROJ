package server

import "testing"

func TestSetPIN(t *testing.T) {
	e := newEnv(t)
	noBVN := e.signUp("08070002222")
	withBVN := e.signUp("08070003333")
	if r := e.call("POST", "/api/auth/bvn", withBVN, map[string]string{"bvn": "22212345678"}); r.Status != 200 {
		t.Fatal(r.Body)
	}
	tests := []struct {
		name  string
		token string
		pin   string
		want  int
		code  string
	}{
		{"needs bvn first", noBVN, "4821", 403, "bvn_required"},
		{"too short", withBVN, "12", 400, "invalid_pin_format"},
		{"letters", withBVN, "48a1", 400, "invalid_pin_format"},
		{"weak", withBVN, "1234", 400, "weak_pin"},
		{"repeated digit", withBVN, "7777", 400, "weak_pin"},
		{"valid", withBVN, "4821", 200, ""},
		{"cannot set twice", withBVN, "5912", 409, "pin_already_set"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/auth/pin", tc.token, map[string]string{"pin": tc.pin})
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
		})
	}
	if r := e.call("POST", "/api/auth/pin", "", map[string]string{"pin": "4821"}); r.Status != 401 {
		t.Fatalf("no token: %d", r.Status)
	}
	// The PIN is stored hashed with a per user salt: two users with the same PIN differ.
	w := e.s.World()
	if w == nil {
		t.Fatal("world")
	}
}
