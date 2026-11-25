package server

import "testing"

func TestDashboard(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	r := e.call("GET", "/api/dashboard", sam, nil)
	if r.Status != 200 {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	if r.Body["total_balance_kobo"] != float64(19500000) {
		t.Fatalf("total = %v, want 19500000 (N195,000)", r.Body["total_balance_kobo"])
	}
	banks := r.Body["banks"].([]any)
	if len(banks) != 4 || banks[0].(map[string]any)["bank_code"] != "apex" {
		t.Fatalf("banks: %v", banks)
	}
	feed := r.Body["transactions"].([]any)
	if len(feed) < 5 {
		t.Fatalf("feed too short: %d", len(feed))
	}
	// Newest first, and the unconfirmed N80,000 credit is marked unverified.
	var prev string
	sawUnverified := false
	for i, it := range feed {
		m := it.(map[string]any)
		at := m["posted_at"].(string)
		if i > 0 && at > prev {
			t.Fatalf("feed not newest first at %d: %s after %s", i, at, prev)
		}
		prev = at
		for _, f := range []string{"direction", "amount_kobo", "bank_name", "counterparty", "category", "verification", "posted_at"} {
			if m[f] == nil {
				t.Fatalf("feed item missing %s: %v", f, m)
			}
		}
		if m["verification"] == "unverified" && m["amount_kobo"] == float64(8000000) && m["direction"] == "credit" {
			sawUnverified = true
		}
	}
	if !sawUnverified {
		t.Fatalf("the unconfirmed credit is not marked unverified: %v", feed)
	}

	t.Run("limit", func(t *testing.T) {
		r := e.call("GET", "/api/dashboard?limit=2", sam, nil)
		if len(r.Body["transactions"].([]any)) != 2 {
			t.Fatalf("limit ignored")
		}
	})
	t.Run("a bank that is down is flagged and left out of the total", func(t *testing.T) {
		if err := e.s.World().SetFailure("", "delta", "bank_unavailable"); err != nil {
			t.Fatal(err)
		}
		r := e.call("GET", "/api/dashboard", sam, nil)
		if r.Body["total_balance_kobo"] != float64(19500000-6000000) {
			t.Fatalf("total = %v", r.Body["total_balance_kobo"])
		}
		down := 0
		for _, b := range r.Body["banks"].([]any) {
			if b.(map[string]any)["unavailable"] == true {
				down++
				if b.(map[string]any)["bank_code"] != "delta" {
					t.Fatalf("wrong bank flagged")
				}
			}
		}
		if down != 1 {
			t.Fatalf("unavailable banks = %d", down)
		}
	})
	t.Run("a user sees only their own data", func(t *testing.T) {
		r := e.call("GET", "/api/dashboard", e.bola(), nil)
		if r.Body["total_balance_kobo"] != float64(3000000) {
			t.Fatalf("bola total = %v", r.Body["total_balance_kobo"])
		}
	})
	t.Run("auth", func(t *testing.T) {
		if r := e.call("GET", "/api/dashboard", "", nil); r.Status != 401 {
			t.Fatalf("%d", r.Status)
		}
	})
}
