package server

import "testing"

func TestSimulateCredit(t *testing.T) {
	e := newEnv(t)
	bola := e.bolaRules()
	before := e.balanceOf(bola, "apex")
	t.Run("a salary splits 30 and 40 percent", func(t *testing.T) {
		r := e.call("POST", "/api/demo/simulate-credit", bola, map[string]any{"amount_kobo": 20000000, "sender_name": "Acme Payroll"})
		if r.Status != 201 || r.Body["verified"] != true || len(r.Body["runs"].([]any)) != 1 {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		run := r.Body["runs"].([]any)[0].(map[string]any)
		if run["sent_kobo"] != float64(14000000) || run["kept_kobo"] != float64(6000000) {
			t.Fatalf("%v", run)
		}
		if e.balanceOf(bola, "apex")-before != 6000000 {
			t.Fatalf("receiving account should keep 60 percent")
		}
	})
	t.Run("unconfirmed credit shows in the feed but not the balance and is not split", func(t *testing.T) {
		bal := e.balanceOf(bola, "apex")
		r := e.call("POST", "/api/demo/simulate-credit", bola, map[string]any{"amount_kobo": 5000000, "verified": false})
		if r.Status != 201 || r.Body["verified"] != false || len(r.Body["runs"].([]any)) != 0 {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		if e.balanceOf(bola, "apex") != bal {
			t.Fatal("unconfirmed money must not enter the balance")
		}
		feed := e.call("GET", "/api/dashboard?limit=1", bola, nil).Body["transactions"].([]any)[0].(map[string]any)
		if feed["verification"] != "unverified" || feed["amount_kobo"] != float64(5000000) {
			t.Fatalf("feed: %v", feed)
		}
		// Shield now warns Bola near that amount.
		if shield(e, bola, "5000000").Body["alert"] == nil {
			t.Fatal("shield should warn")
		}
	})
	t.Run("scan false leaves it for the scan endpoint", func(t *testing.T) {
		r := e.call("POST", "/api/demo/simulate-credit", bola, map[string]any{"amount_kobo": 1000000, "scan": false})
		if len(r.Body["runs"].([]any)) != 0 {
			t.Fatalf("%v", r.Body)
		}
		if s := e.call("POST", "/api/splitreceive/scan", bola, nil); s.Body["count"] != float64(1) {
			t.Fatalf("%v", s.Body)
		}
	})
	t.Run("errors", func(t *testing.T) {
		cases := []struct {
			name  string
			token string
			body  any
			want  int
			code  string
		}{
			{"no amount", bola, map[string]any{}, 400, "invalid_amount"},
			{"tiny amount", bola, map[string]any{"amount_kobo": 5}, 400, "invalid_amount"},
			{"someone else's account", bola, map[string]any{"amount_kobo": 1000, "account_id": e.accountID(e.sam(), "apex")}, 404, "not_found"},
			{"no token", "", map[string]any{"amount_kobo": 1000}, 401, "unauthorized"},
			{"not onboarded", e.signUp("08076660000"), map[string]any{"amount_kobo": 1000}, 403, "onboarding_required"},
			{"bad json", bola, "{", 400, "invalid_json"},
		}
		for _, tc := range cases {
			r := e.call("POST", "/api/demo/simulate-credit", tc.token, tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("%s: got %d %q (%v)", tc.name, r.Status, errCode(r), r.Body)
			}
		}
	})
}
