package server

import (
	"math/rand"
	"testing"
)

// checkLedger asserts the invariant sent + held + kept = credit on every credit and in the totals.
func checkLedger(t *testing.T, r reply) (credit, sent, held, kept float64) {
	t.Helper()
	if r.Status != 200 {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	for _, c := range r.Body["credits"].([]any) {
		m := c.(map[string]any)
		a, s, h, k := m["amount_kobo"].(float64), m["sent_kobo"].(float64), m["held_kobo"].(float64), m["kept_kobo"].(float64)
		if s+h+k != a || m["balanced"] != true {
			t.Fatalf("credit does not balance: %v", m)
		}
		if s < 0 || h < 0 || k < 0 {
			t.Fatalf("negative part: %v", m)
		}
	}
	tot := r.Body["totals"].(map[string]any)
	credit, sent, held, kept = tot["credit_kobo"].(float64), tot["sent_kobo"].(float64), tot["held_kobo"].(float64), tot["kept_kobo"].(float64)
	if sent+held+kept != credit {
		t.Fatalf("totals do not balance: %v", tot)
	}
	return
}

func TestReconciliationBalancesForTheStory(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	bola := e.bolaRules()
	// Bola's own seed deposit is not a credit; N20,000 from Sam by phone, then a N200,000 salary.
	e.pay(sam, execBody(20000, apapoTo("08098765432"), "4821"), "rc-p2p")
	e.credit(bola, 20000000, true)
	e.call("POST", "/api/splitreceive/scan", bola, nil)
	r := e.call("GET", "/api/reconciliation", bola, nil)
	credit, sent, held, kept := checkLedger(t, r)
	if credit != 2000000+20000000 || sent != 1400000+14000000 || held != 0 || kept != 600000+6000000 {
		t.Fatalf("totals: credit=%v sent=%v held=%v kept=%v", credit, sent, held, kept)
	}
	list := r.Body["credits"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["amount_kobo"] != float64(20000000) {
		t.Fatalf("newest first: %v", list)
	}
	if list[1].(map[string]any)["sender_name"] != "Samuel P." || list[1].(map[string]any)["status"] != "split" {
		t.Fatalf("p2p credit entry: %v", list[1])
	}
}

func TestReconciliationCoversEveryKindOfCredit(t *testing.T) {
	e := newEnv(t)
	bola := e.bolaRules()
	e.credit(bola, 3000000, false)                               // waiting for the bank
	e.fail(e.accountID(bola, "savanna"), "", "bank_unavailable") // one share will be held
	e.credit(bola, 10000000, true)
	e.call("POST", "/api/splitreceive/scan", bola, nil)
	r := e.call("GET", "/api/reconciliation", bola, nil)
	credit, sent, held, kept := checkLedger(t, r)
	if credit != 13000000 || sent != 4000000 || held != 3000000 || kept != 3000000+3000000 {
		t.Fatalf("credit=%v sent=%v held=%v kept=%v", credit, sent, held, kept)
	}
	statuses := map[string]bool{}
	for _, c := range r.Body["credits"].([]any) {
		statuses[c.(map[string]any)["status"].(string)] = true
	}
	if !statuses["waiting_for_bank"] || !statuses["partial"] {
		t.Fatalf("statuses: %v", statuses)
	}
	// After the fault clears and the share is retried, held goes to zero and it still balances.
	e.fail(e.accountID(bola, "savanna"), "", "none")
	run := runsOf(e, bola)[0].(map[string]any)["id"].(string)
	e.call("POST", "/api/splitreceive/runs/"+run+"/retry", bola, nil)
	_, _, held, _ = checkLedger(t, e.call("GET", "/api/reconciliation", bola, nil))
	if held != 0 {
		t.Fatalf("held after retry: %v", held)
	}
}

// A hard test of the invariant: many random credits and random rules, with random failures.
func TestLedgerInvariantUnderRandomLoad(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for round := 0; round < 25; round++ {
		e := newEnv(t)
		bola := e.bola()
		e.call("POST", "/api/banks/link", bola, linkBody("delta"))
		e.call("POST", "/api/banks/link", bola, linkBody("savanna"))
		apex, delta, sav := e.accountID(bola, "apex"), e.accountID(bola, "delta"), e.accountID(bola, "savanna")
		var dests []map[string]any
		left := 100.0
		for i := 0; i < rng.Intn(4)+1; i++ {
			p := float64(rng.Intn(int(left/2)+1)) + float64(rng.Intn(100))/100
			if p <= 0 || p > left {
				continue
			}
			left -= p
			dests = append(dests, ownPct([]string{delta, sav}[rng.Intn(2)], p))
		}
		if rng.Intn(2) == 0 {
			dests = append(dests, map[string]any{"bank_code": "harbor", "account_number": "1000000001", "amount_kobo": int64(rng.Intn(500000) + 1)})
		}
		if r := e.call("PUT", "/api/splitreceive/rules", bola, rulesBody(apex, "5912", dests...)); r.Status != 200 {
			continue // an unlucky random total over 100 percent; the validator is tested elsewhere
		}
		for i := 0; i < rng.Intn(6)+1; i++ {
			if rng.Intn(4) == 0 {
				e.fail([]string{delta, sav}[rng.Intn(2)], "", "bank_unavailable")
			} else if rng.Intn(3) == 0 {
				e.fail("", "", "none")
			}
			e.credit(bola, int64(rng.Intn(50000000)+100), rng.Intn(5) != 0)
			e.call("POST", "/api/splitreceive/scan", bola, nil)
		}
		credit, sent, held, kept := checkLedger(t, e.call("GET", "/api/reconciliation", bola, nil))
		if sent+held+kept != credit {
			t.Fatalf("round %d: %v %v %v %v", round, credit, sent, held, kept)
		}
	}
}

func TestReconciliationIsPrivateAndAuthed(t *testing.T) {
	e := newEnv(t)
	bola := e.bolaRules()
	e.credit(bola, 1000000, true)
	e.call("POST", "/api/splitreceive/scan", bola, nil)
	sam := e.call("GET", "/api/reconciliation", e.sam(), nil)
	if len(sam.Body["credits"].([]any)) != 1 { // Samuel only has his own unconfirmed N80,000 credit
		t.Fatalf("sam sees: %v", sam.Body["credits"])
	}
	if sam.Body["credits"].([]any)[0].(map[string]any)["sender_name"] != "Greenfield Traders" {
		t.Fatalf("sam sees someone else's credit: %v", sam.Body["credits"])
	}
	if r := e.call("GET", "/api/reconciliation", "", nil); r.Status != 401 {
		t.Fatalf("%d", r.Status)
	}
}
