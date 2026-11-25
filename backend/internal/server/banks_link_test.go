package server

import "testing"

func TestLinkBank(t *testing.T) {
	e := newEnv(t)
	newbie := e.onboarded("08070006666", "4821")
	notDone := e.signUp("08070007777")
	tests := []struct {
		name  string
		token string
		body  any
		want  int
		code  string
	}{
		{"links apex", newbie, linkBody("apex"), 201, ""},
		{"linking twice is idempotent", newbie, linkBody("apex"), 200, ""},
		{"no consent", newbie, map[string]any{"bank_code": "delta"}, 400, "consent_required"},
		{"consent not accepted", newbie, map[string]any{"bank_code": "delta", "consent": map[string]any{"accepted": false, "scopes": []string{"accounts", "balances", "transactions"}}}, 400, "consent_required"},
		{"missing scope", newbie, map[string]any{"bank_code": "delta", "consent": map[string]any{"accepted": true, "scopes": []string{"accounts"}}}, 400, "consent_required"},
		{"unknown bank", newbie, linkBody("nope"), 400, "unknown_bank"},
		{"coming soon bank", newbie, linkBody("cedar"), 400, "bank_not_supported"},
		{"not onboarded", notDone, linkBody("apex"), 403, "onboarding_required"},
		{"no token", "", linkBody("apex"), 401, "unauthorized"},
		{"bad json", newbie, "{", 400, "invalid_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/banks/link", tc.token, tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
		})
	}
	// Idempotent: still one account, and it became the receiving account.
	me := e.call("GET", "/api/me", newbie, nil)
	if get(me.Body, "receiving_account", "bank_code") != "apex" {
		t.Fatalf("first linked bank should become the receiving account: %v", me.Body)
	}
}

func TestLinkReturnsExpiryAndMaskedAccount(t *testing.T) {
	e := newEnv(t)
	r := e.call("POST", "/api/banks/link", e.onboarded("08070008888", "4821"), linkBody("harbor"))
	if r.Status != 201 || r.Body["expires_at"] == nil || get(r.Body, "account", "balance_kobo") != float64(2000000) {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	if n := get(r.Body, "account", "masked_account_number").(string); len(n) != 10 || n[0] != '*' {
		t.Fatalf("account number not masked: %q", n)
	}
}
