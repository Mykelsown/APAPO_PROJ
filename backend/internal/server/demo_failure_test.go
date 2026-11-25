package server

import "testing"

func TestDemoFailureEndpoint(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	apex := e.accountID(sam, "apex")
	tests := []struct {
		name string
		body any
		want int
		code string
	}{
		{"bank unavailable", map[string]any{"bank_code": "delta", "mode": "bank_unavailable"}, 200, ""},
		{"account insufficient", map[string]any{"account_id": apex, "mode": "insufficient_funds"}, 200, ""},
		{"timeout", map[string]any{"account_id": apex, "mode": "timeout"}, 200, ""},
		{"reversal down needs a bank", map[string]any{"bank_code": "delta", "mode": "reversal_down"}, 200, ""},
		{"reversal down without a bank", map[string]any{"mode": "reversal_down"}, 400, "bank_code_required"},
		{"reversal down with an account", map[string]any{"account_id": apex, "mode": "reversal_down"}, 400, "bank_code_required"},
		{"none for a bank clears it", map[string]any{"bank_code": "delta", "mode": "none"}, 200, ""},
		{"none for everything", map[string]any{"mode": "none"}, 200, ""},
		{"unknown mode", map[string]any{"bank_code": "delta", "mode": "explode"}, 400, "invalid_mode"},
		{"both targets", map[string]any{"bank_code": "delta", "account_id": apex, "mode": "timeout"}, 400, "invalid_target"},
		{"no target", map[string]any{"mode": "bank_unavailable"}, 400, "invalid_target"},
		{"unknown bank", map[string]any{"bank_code": "zzz", "mode": "bank_unavailable"}, 400, "unknown_bank"},
		{"unknown account", map[string]any{"account_id": "acc_nope", "mode": "timeout"}, 404, "not_found"},
		{"bad json", "{", 400, "invalid_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/demo/failure", "", tc.body) // open: no token needed
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
		})
	}
}

func TestDemoFailureNoneWithBankClearsReversalOutage(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	e.call("POST", "/api/demo/failure", "", map[string]any{"bank_code": "delta", "mode": "reversal_down"})
	e.call("POST", "/api/demo/failure", "", map[string]any{"account_id": e.accountID(sam, "savanna"), "mode": "insufficient_funds"})
	r := e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "df-1")
	if r.Body["state"] != "needs_attention" {
		t.Fatalf("outage not applied: %v", r.Body)
	}
	e.call("POST", "/api/demo/failure", "", map[string]any{"bank_code": "delta", "mode": "none"})
	e.advance(time_minute)
	e.s.World().RetryReversals()
	if got := status(e, sam, r.Body["id"].(string)).Body["state"]; got != "rolled_back" {
		t.Fatalf("mode none with a bank_code must clear the reversal outage: %v", got)
	}
}

func TestDemoEndpointsCanBeSwitchedOff(t *testing.T) {
	cfgEnv := map[string]string{"APP_SECRET": "x", "DEMO_ENDPOINTS": "false"}
	e := newEnvWith(t, cfgEnv)
	for _, p := range []string{"/api/demo/failure", "/api/demo/reset", "/api/demo/simulate-credit"} {
		if r := e.call("POST", p, "", map[string]any{"mode": "none"}); r.Status != 404 {
			t.Fatalf("%s: %d %v", p, r.Status, r.Body)
		}
	}
}
