package server

import (
	"testing"

	"github.com/Mykelsown/apapo-backend/internal/app"
)

func TestScanSplitsConfirmedCreditsExactlyOnce(t *testing.T) {
	e := newEnv(t)
	bola := e.bolaRules()
	apexBefore, savBefore, deltaBefore := e.balanceOf(bola, "apex"), e.balanceOf(bola, "savanna"), e.balanceOf(bola, "delta")
	e.credit(bola, 20000000, true) // N200,000 salary

	r := e.call("POST", "/api/splitreceive/scan", bola, nil)
	if r.Status != 200 || r.Body["count"] != float64(1) {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	run := r.Body["new_runs"].([]any)[0].(map[string]any)
	if run["sent_kobo"] != float64(14000000) || run["kept_kobo"] != float64(6000000) || run["held_kobo"] != float64(0) || run["state"] != "completed" {
		t.Fatalf("N200,000 at 30 and 40 percent should send N140,000 and keep N60,000: %v", run)
	}
	legs := run["legs"].([]any)
	if len(legs) != 2 || legs[0].(map[string]any)["amount_kobo"] != float64(6000000) || legs[1].(map[string]any)["amount_kobo"] != float64(8000000) {
		t.Fatalf("legs: %v", legs)
	}
	if got := e.balanceOf(bola, "savanna") - savBefore; got != 6000000 {
		t.Fatalf("savings rose by %v", got)
	}
	if got := e.balanceOf(bola, "delta") - deltaBefore; got != 8000000 {
		t.Fatalf("spending rose by %v", got)
	}
	if got := e.balanceOf(bola, "apex") - apexBefore; got != 6000000 {
		t.Fatalf("receiving account should keep 60 percent: %v", got)
	}
	// Scanning again must not split the same credit twice.
	again := e.call("POST", "/api/splitreceive/scan", bola, nil)
	if again.Body["count"] != float64(0) {
		t.Fatalf("second scan split again: %v", again.Body)
	}
	e.s.World().ScanAll()
	if got := e.balanceOf(bola, "savanna") - savBefore; got != 6000000 {
		t.Fatalf("timer pass split again: %v", got)
	}
}

func TestScanIgnoresWhatItShould(t *testing.T) {
	e := newEnv(t)
	bola := e.bolaRules()
	t.Run("unconfirmed credit is not split", func(t *testing.T) {
		e.credit(bola, 5000000, false)
		if r := e.call("POST", "/api/splitreceive/scan", bola, nil); r.Body["count"] != float64(0) {
			t.Fatalf("%v", r.Body)
		}
	})
	t.Run("credit with no rules is untouched", func(t *testing.T) {
		sam := e.sam()
		e.credit(sam, 5000000, true)
		if r := e.call("POST", "/api/splitreceive/scan", sam, nil); r.Body["count"] != float64(0) {
			t.Fatalf("%v", r.Body)
		}
	})
	t.Run("credit that arrived before consent is not split", func(t *testing.T) {
		e2 := newEnv(t)
		tok := e2.bola()
		e2.call("POST", "/api/banks/link", tok, linkBody("delta"))
		e2.credit(tok, 5000000, true)
		e2.advance(time_minute)
		e2.call("PUT", "/api/splitreceive/rules", tok, rulesBody(e2.accountID(tok, "apex"), "5912", ownPct(e2.accountID(tok, "delta"), 50)))
		if r := e2.call("POST", "/api/splitreceive/scan", tok, nil); r.Body["count"] != float64(0) {
			t.Fatalf("%v", r.Body)
		}
	})
	t.Run("a credit in another account is not split", func(t *testing.T) {
		uid, _ := e.s.World().Authenticate(bola)
		f := false
		if _, err := e.s.World().SimulateCredit(uid, app.SimulateIn{AmountKobo: 1000000, AccountID: e.accountID(bola, "delta"), Scan: &f}); err != nil {
			t.Fatal(err)
		}
		if r := e.call("POST", "/api/splitreceive/scan", bola, nil); r.Body["count"] != float64(0) {
			t.Fatalf("%v", r.Body)
		}
	})
	t.Run("auth", func(t *testing.T) {
		if r := e.call("POST", "/api/splitreceive/scan", "", nil); r.Status != 401 {
			t.Fatalf("%d", r.Status)
		}
	})
}

func TestApapoOwnMovementsAreNotSplitButP2PIs(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	bola := e.bolaRules()
	savBefore := e.balanceOf(bola, "savanna")
	// A reversal and a collection credit into Bola's receiving account would carry pay_/rev_ references;
	// they never become credits at all. A P2P payment does, and is split once by Bola's rules.
	r := e.pay(sam, execBody(20000, apapoTo("08098765432"), "4821"), "p2p-split")
	if r.Body["state"] != "completed" {
		t.Fatalf("%v", r.Body)
	}
	if got := e.balanceOf(bola, "savanna") - savBefore; got != 600000 { // 30 percent of N20,000
		t.Fatalf("savings rose by %v, want 600000", got)
	}
	runs := e.call("POST", "/api/splitreceive/scan", bola, nil)
	if runs.Body["count"] != float64(0) {
		t.Fatalf("the P2P credit was already split once: %v", runs.Body)
	}
	if got := e.balanceOf(bola, "savanna") - savBefore; got != 600000 {
		t.Fatalf("split twice: %v", got)
	}
}
