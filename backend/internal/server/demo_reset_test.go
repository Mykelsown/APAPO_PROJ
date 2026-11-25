package server

import "testing"

func TestDemoReset(t *testing.T) {
	e := newEnv(t)
	sam, bola := e.sam(), e.bolaRules()
	// Make a mess: payments, runs, reports, failures, an extra user and a revoked bank.
	e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "rs-1")
	e.pay(sam, execBody(20000, apapoTo("08098765432"), "4821"), "rs-2")
	e.credit(bola, 10000000, true)
	e.call("POST", "/api/splitreceive/scan", bola, nil)
	third := e.onboarded("08077000001", "4821")
	for _, tok := range []string{sam, bola, third} {
		e.call("POST", "/api/reports", tok, reportBody("harbor", "6600000009", "ev"))
	}
	e.call("POST", "/api/demo/failure", "", map[string]any{"bank_code": "harbor", "mode": "bank_unavailable"})
	stranger := e.onboarded("08077000002", "4821")
	shield(e, sam, "8000000")
	e.call("POST", "/api/shield/dismiss", sam, map[string]string{"credit_id": "crd_seed_unconfirmed"})
	linkID := e.call("GET", "/api/accounts", sam, nil).Body["accounts"].([]any)[1].(map[string]any)["link_id"].(string)
	e.call("DELETE", "/api/banks/"+linkID, sam, nil)

	r := e.call("POST", "/api/demo/reset", "", nil)
	if r.Status != 200 || r.Body["reset"] != true {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	t.Run("seeded users stay signed in with their ids", func(t *testing.T) {
		if r := e.call("GET", "/api/me", sam, nil); r.Status != 200 {
			t.Fatalf("sam's token died: %d %v", r.Status, r.Body)
		}
		if r := e.call("GET", "/api/me", bola, nil); r.Status != 200 {
			t.Fatalf("bola's token died: %d", r.Status)
		}
	})
	t.Run("balances and accounts are back", func(t *testing.T) {
		if e.total(sam) != 19500000 || len(e.call("GET", "/api/accounts", sam, nil).Body["accounts"].([]any)) != 4 {
			t.Fatalf("sam: %v", e.total(sam))
		}
		if e.total(bola) != 3000000 || len(e.call("GET", "/api/accounts", bola, nil).Body["accounts"].([]any)) != 1 {
			t.Fatalf("bola: %v", e.total(bola))
		}
	})
	t.Run("payments, runs, rules and credits are cleared", func(t *testing.T) {
		if got := len(e.call("GET", "/api/payments", sam, nil).Body["payments"].([]any)); got != 0 {
			t.Fatalf("payments: %d", got)
		}
		if got := len(runsOf(e, bola)); got != 0 {
			t.Fatalf("runs: %d", got)
		}
		if e.call("GET", "/api/splitreceive/rules", bola, nil).Body["rules"] != nil {
			t.Fatal("rules should be cleared")
		}
		recon := e.call("GET", "/api/reconciliation", bola, nil)
		if len(recon.Body["credits"].([]any)) != 0 {
			t.Fatalf("credits: %v", recon.Body["credits"])
		}
	})
	t.Run("failure switches are cleared", func(t *testing.T) {
		d := e.call("GET", "/api/dashboard", sam, nil)
		for _, b := range d.Body["banks"].([]any) {
			if b.(map[string]any)["unavailable"] == true {
				t.Fatalf("still unavailable: %v", b)
			}
		}
		if r := e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "after-reset"); r.Body["state"] != "completed" {
			t.Fatalf("a clean payment should complete again: %v", r.Body)
		}
	})
	t.Run("seed data is restored, user reports are gone", func(t *testing.T) {
		if shield(e, sam, "8000000").Body["alert"] == nil {
			t.Fatal("the seeded unconfirmed credit and its alert should be back")
		}
		if get(e.call("POST", "/api/risk/check", sam, previewBody(5000, outside("apex", "9000000001"))).Body, "risk", "tier") != "high" {
			t.Fatal("the flagged account must still be high risk")
		}
		if get(e.call("POST", "/api/risk/check", sam, previewBody(5000, outside("harbor", "6600000009"))).Body, "risk", "tier") == "high" {
			t.Fatal("reports made during the demo must be cleared")
		}
	})
	t.Run("other users are removed", func(t *testing.T) {
		if r := e.call("GET", "/api/me", stranger, nil); r.Status != 401 {
			t.Fatalf("a user created during the demo should be gone: %d", r.Status)
		}
	})
	t.Run("it can be run again and again", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			if r := e.call("POST", "/api/demo/reset", "", nil); r.Status != 200 {
				t.Fatalf("%d", r.Status)
			}
		}
		if e.total(sam) != 19500000 {
			t.Fatalf("%v", e.total(sam))
		}
	})
}
