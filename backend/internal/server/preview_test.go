package server

import (
	"strings"
	"testing"
)

func legAmounts(r reply) []float64 {
	var out []float64
	for _, l := range r.Body["legs"].([]any) {
		out = append(out, l.(map[string]any)["amount_kobo"].(float64))
	}
	return out
}

func TestPreviewPlansFewestLegs(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	tests := []struct {
		name  string
		naira int64
		legs  []float64
		fee   float64
	}{
		{"one account covers it", 50000, []float64{5000000}, 1000},
		{"N150,000 needs three legs", 150000, []float64{6999000, 5999000, 2002000}, 3000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/splitpay/preview", sam, previewBody(tc.naira, outside("delta", "1000000001")))
			if r.Status != 200 {
				t.Fatalf("%d %v", r.Status, r.Body)
			}
			got := legAmounts(r)
			if len(got) != len(tc.legs) {
				t.Fatalf("legs = %v, want %v", got, tc.legs)
			}
			var sum float64
			for i := range got {
				if got[i] != tc.legs[i] {
					t.Fatalf("legs = %v, want %v", got, tc.legs)
				}
				sum += got[i]
			}
			if sum != float64(tc.naira*naira) || r.Body["total_amount_kobo"] != sum || r.Body["total_fee_kobo"] != tc.fee {
				t.Fatalf("totals wrong: %v", r.Body)
			}
			first := r.Body["legs"].([]any)[0].(map[string]any)
			if first["is_collector"] != true || r.Body["collector_account_id"] != first["account_id"] {
				t.Fatalf("first leg should be the collector: %v", first)
			}
			for _, k := range []string{"risk", "flags", "duplicate_warning", "shield_warning", "total_debit_kobo"} {
				if _, ok := r.Body[k]; !ok {
					t.Fatalf("missing %s", k)
				}
			}
		})
	}
}

func TestPreviewMinBalanceAndMaxLegs(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	body := previewBody(45000, outside("delta", "1000000001"))
	body["min_balance_kobo"] = 4000000 // keep N40,000 in each account
	r := e.call("POST", "/api/splitpay/preview", sam, body)
	if r.Status != 200 || len(legAmounts(r)) != 2 {
		t.Fatalf("expected 2 legs for N45,000 with a N40,000 floor: %d %v", r.Status, r.Body)
	}
	body = previewBody(150000, outside("delta", "1000000001"))
	body["max_legs"] = 2
	r = e.call("POST", "/api/splitpay/preview", sam, body)
	if r.Status != 422 || errCode(r) != "too_many_legs" {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
}

func TestPreviewErrors(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	tests := []struct {
		name  string
		token string
		body  any
		want  int
		code  string
	}{
		{"no token", "", previewBody(100, outside("delta", "1000000001")), 401, "unauthorized"},
		{"not onboarded", e.signUp("08071110000"), previewBody(100, outside("delta", "1000000001")), 403, "onboarding_required"},
		{"zero amount", sam, previewBody(0, outside("delta", "1000000001")), 400, "invalid_amount"},
		{"negative amount", sam, map[string]any{"amount_kobo": -5, "recipient": outside("delta", "1000000001")}, 400, "invalid_amount"},
		{"bad account number", sam, previewBody(100, outside("delta", "123")), 400, "invalid_account_number"},
		{"letters in account number", sam, previewBody(100, outside("delta", "10000000ab")), 400, "invalid_account_number"},
		{"missing bank", sam, previewBody(100, map[string]any{"account_number": "1000000001"}), 400, "invalid_recipient"},
		{"unknown bank", sam, previewBody(100, outside("zzz", "1000000001")), 400, "unknown_bank"},
		{"empty recipient", sam, previewBody(100, map[string]any{}), 400, "invalid_recipient"},
		{"bad strategy", sam, map[string]any{"amount_kobo": 10000, "recipient": outside("delta", "1000000001"), "strategy": "wild"}, 400, "invalid_strategy"},
		{"more than all balances", sam, previewBody(200000, outside("delta", "1000000001")), 422, "insufficient_funds"},
		{"apapo self send", sam, previewBody(100, apapoTo("08012345678")), 400, "self_send"},
		{"apapo bad phone", sam, previewBody(100, apapoTo("12")), 400, "invalid_phone"},
		{"apapo unknown", sam, previewBody(100, apapoTo("08011110000")), 404, "not_found"},
		{"bad json", sam, "{", 400, "invalid_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/splitpay/preview", tc.token, tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
		})
	}
	t.Run("insufficient funds says how much is spendable and short", func(t *testing.T) {
		r := e.call("POST", "/api/splitpay/preview", sam, previewBody(200000, outside("delta", "1000000001")))
		d := r.Body["error"].(map[string]any)["details"].(map[string]any)
		if d["spendable_kobo"] != float64(19500000-4000) || d["shortfall_kobo"] != float64(20000000-(19500000-4000)) {
			t.Fatalf("details: %v", d)
		}
	})
}

// The frontend sends display fields inside the recipient. They must be accepted and ignored.
func TestPreviewAcceptsFrontendDisplayFields(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	rec := apapoTo("08098765432")
	rec["recipient_type"] = "apapo"
	rec["masked_display_name"] = "Bola A."
	rec["something_new"] = 42
	r := e.call("POST", "/api/splitpay/preview", sam, previewBody(5000, rec))
	if r.Status != 200 {
		t.Fatalf("display fields rejected: %d %v", r.Status, r.Body)
	}
	if get(r.Body, "recipient", "recipient_type") != "apapo" || get(r.Body, "recipient", "masked_display_name") != "Bola A." {
		t.Fatalf("recipient: %v", r.Body["recipient"])
	}
	raw := string(mustJSON(r.Body["recipient"]))
	if strings.Contains(raw, "2009009009") || strings.Contains(raw, "account") {
		t.Fatalf("apapo recipient must not expose an account: %s", raw)
	}
	out := outside("apex", "1000000001")
	out["recipient_type"] = "outside"
	out["masked_display_name"] = "whatever"
	if r := e.call("POST", "/api/splitpay/preview", sam, previewBody(5000, out)); r.Status != 200 {
		t.Fatalf("outside with display fields: %d %v", r.Status, r.Body)
	}
}

func TestPreviewRiskAndWarnings(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	t.Run("reported account is high", func(t *testing.T) {
		r := e.call("POST", "/api/splitpay/preview", sam, previewBody(5000, outside("apex", "9000000001")))
		if get(r.Body, "risk", "tier") != "high" || len(r.Body["flags"].([]any)) == 0 {
			t.Fatalf("%v", r.Body)
		}
		if get(r.Body, "recipient", "account_name") != "Quick Returns Investments" {
			t.Fatalf("recipient: %v", r.Body["recipient"])
		}
	})
	t.Run("known good payee is low", func(t *testing.T) {
		r := e.call("POST", "/api/splitpay/preview", sam, previewBody(5000, outside("delta", "1000000001")))
		if get(r.Body, "risk", "tier") != "low" {
			t.Fatalf("%v", r.Body["risk"])
		}
	})
	t.Run("clean apapo recipient is low", func(t *testing.T) {
		r := e.call("POST", "/api/splitpay/preview", sam, previewBody(5000, apapoTo("08098765432")))
		if get(r.Body, "risk", "tier") != "low" {
			t.Fatalf("%v", r.Body["risk"])
		}
	})
	t.Run("a stranger is unknown", func(t *testing.T) {
		r := e.call("POST", "/api/splitpay/preview", sam, previewBody(5000, outside("savanna", "4455667788")))
		if get(r.Body, "risk", "tier") != "unknown" {
			t.Fatalf("%v", r.Body["risk"])
		}
	})
	t.Run("name mismatch is flagged", func(t *testing.T) {
		rec := outside("delta", "1000000001")
		rec["account_name"] = "Someone Else"
		r := e.call("POST", "/api/splitpay/preview", sam, previewBody(5000, rec))
		if get(r.Body, "risk", "tier") != "unknown" || !strings.Contains(string(mustJSON(r.Body["flags"])), "name_mismatch") {
			t.Fatalf("%v", r.Body)
		}
	})
	t.Run("unusual amount is flagged", func(t *testing.T) {
		r := e.call("POST", "/api/splitpay/preview", sam, previewBody(150000, outside("delta", "1000000001")))
		if !strings.Contains(string(mustJSON(r.Body["flags"])), "unusual_amount") {
			t.Fatalf("%v", r.Body["flags"])
		}
	})
	t.Run("shield warns near the unconfirmed N80,000 credit", func(t *testing.T) {
		r := e.call("POST", "/api/splitpay/preview", sam, previewBody(78000, outside("delta", "1000000001")))
		if get(r.Body, "shield_warning", "credit_id") == nil {
			t.Fatalf("expected a shield warning: %v", r.Body["shield_warning"])
		}
		r = e.call("POST", "/api/splitpay/preview", sam, previewBody(50000, outside("delta", "1000000001")))
		if r.Body["shield_warning"] != nil {
			t.Fatalf("no warning expected at N50,000: %v", r.Body["shield_warning"])
		}
	})
}
