package server

import (
	"strings"
	"testing"
)

func TestRiskCheck(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	tests := []struct {
		name  string
		token string
		body  any
		want  int
		code  string
		tier  string
		flag  string
	}{
		{"reported account", sam, previewBody(5000, outside("apex", "9000000001")), 200, "", "high", "reported_account"},
		{"good payee", sam, previewBody(5000, outside("delta", "1000000001")), 200, "", "low", ""},
		{"stranger", sam, previewBody(5000, outside("harbor", "4455667788")), 200, "", "unknown", "first_payment"},
		{"clean apapo user", sam, previewBody(5000, apapoTo("08098765432")), 200, "", "low", ""},
		{"zero amount is allowed for a recipient only check", sam, previewBody(0, outside("delta", "1000000001")), 200, "", "low", ""},
		{"negative amount", sam, map[string]any{"amount_kobo": -1, "recipient": outside("delta", "1000000001")}, 400, "invalid_amount", "", ""},
		{"bad account", sam, previewBody(5000, outside("delta", "12")), 400, "invalid_account_number", "", ""},
		{"display fields are ignored", sam, previewBody(5000, map[string]any{"apapo_phone": "08098765432", "recipient_type": "apapo", "masked_display_name": "x"}), 200, "", "low", ""},
		{"no token", "", previewBody(5000, outside("delta", "1000000001")), 401, "unauthorized", "", ""},
		{"not onboarded", e.signUp("08073330000"), previewBody(5000, outside("delta", "1000000001")), 403, "onboarding_required", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/risk/check", tc.token, tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
			if tc.want != 200 {
				return
			}
			if get(r.Body, "risk", "tier") != tc.tier {
				t.Fatalf("tier = %v, want %s (%v)", get(r.Body, "risk", "tier"), tc.tier, r.Body)
			}
			if len(r.Body["risk"].(map[string]any)["reasons"].([]any)) == 0 {
				t.Fatal("reasons must explain the tier in plain words")
			}
			if tc.flag != "" && !strings.Contains(string(mustJSON(r.Body["flags"])), tc.flag) {
				t.Fatalf("flags %v missing %s", r.Body["flags"], tc.flag)
			}
		})
	}
}

func TestRiskFlagsNameMismatchAndDuplicate(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	rec := outside("delta", "1000000001")
	rec["account_name"] = "Totally Different"
	r := e.call("POST", "/api/risk/check", sam, previewBody(5000, rec))
	if !strings.Contains(string(mustJSON(r.Body["flags"])), "name_mismatch") || get(r.Body, "risk", "tier") != "unknown" {
		t.Fatalf("%v", r.Body)
	}
	e.pay(sam, execBody(3000, outside("delta", "1000000001"), "4821"), "dup-1")
	r = e.call("POST", "/api/risk/check", sam, previewBody(3000, outside("delta", "1000000001")))
	if r.Body["duplicate_warning"] != true || !strings.Contains(string(mustJSON(r.Body["flags"])), "possible_duplicate") {
		t.Fatalf("same amount again within minutes should warn: %v", r.Body)
	}
	if strings.Contains(string(mustJSON(r.Body["flags"])), "first_payment") {
		t.Fatalf("no longer a first payment: %v", r.Body["flags"])
	}
}
