package server

import "testing"

func shield(e *env, tok string, kobo string) reply {
	e.t.Helper()
	return e.call("GET", "/api/shield/check?amount_kobo="+kobo, tok, nil)
}

func TestShieldCheck(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	tests := []struct {
		name  string
		kobo  string
		alert bool
		code  string
		want  int
	}{
		{"exactly the unconfirmed credit", "8000000", true, "", 200},
		{"10 percent below", "7200000", true, "", 200},
		{"10 percent above", "8800000", true, "", 200},
		{"just outside the window", "8800100", false, "", 200},
		{"far away", "1000000", false, "", 200},
		{"missing amount", "", false, "invalid_amount", 400},
		{"zero", "0", false, "invalid_amount", 400},
		{"not a number", "abc", false, "invalid_amount", 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := shield(e, sam, tc.kobo)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
			if tc.want != 200 {
				return
			}
			if (r.Body["alert"] != nil) != tc.alert {
				t.Fatalf("alert = %v, want present=%v", r.Body["alert"], tc.alert)
			}
			if tc.alert {
				a := r.Body["alert"].(map[string]any)
				if a["message"] != "This money has not been confirmed by the bank yet. Wait for confirmation before sending it on." {
					t.Fatalf("message: %v", a["message"])
				}
				if a["credit_id"] == "" || a["sender_name"] != "Greenfield Traders" || a["amount_kobo"] != float64(8000000) {
					t.Fatalf("alert: %v", a)
				}
			}
		})
	}
	t.Run("a key present with null alert", func(t *testing.T) {
		r := shield(e, sam, "1000000")
		if v, ok := r.Body["alert"]; !ok || v != nil {
			t.Fatalf("want alert:null, got %v present=%v", v, ok)
		}
	})
	t.Run("only the owner of the credit is warned", func(t *testing.T) {
		if r := shield(e, e.bola(), "8000000"); r.Body["alert"] != nil {
			t.Fatalf("bola has no unconfirmed credit: %v", r.Body)
		}
	})
	t.Run("auth", func(t *testing.T) {
		if r := shield(e, "", "8000000"); r.Status != 401 {
			t.Fatalf("%d", r.Status)
		}
	})
	t.Run("credits older than 24 hours are ignored", func(t *testing.T) {
		e2 := newEnv(t)
		tok := e2.sam()
		e2.advance(25 * time_hour)
		tok = e2.login("08012345678", "4821")
		if r := shield(e2, tok, "8000000"); r.Body["alert"] != nil {
			t.Fatalf("old credit still alerts: %v", r.Body)
		}
	})
}
