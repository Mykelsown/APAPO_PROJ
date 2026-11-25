package server

import "testing"

func runsOf(e *env, tok string) []any {
	e.t.Helper()
	r := e.call("GET", "/api/splitreceive/runs", tok, nil)
	if r.Status != 200 {
		e.t.Fatalf("%d %v", r.Status, r.Body)
	}
	return r.Body["runs"].([]any)
}

func TestRunsListAndOwnership(t *testing.T) {
	e := newEnv(t)
	bola, sam := e.bolaRules(), e.sam()
	if got := runsOf(e, bola); len(got) != 0 {
		t.Fatalf("no runs yet: %v", got)
	}
	e.credit(bola, 10000000, true)
	e.credit(bola, 20000000, true)
	e.call("POST", "/api/splitreceive/scan", bola, nil)
	runs := runsOf(e, bola)
	if len(runs) != 2 {
		t.Fatalf("runs: %d", len(runs))
	}
	if runs[0].(map[string]any)["credit_amount_kobo"] != float64(20000000) {
		t.Fatalf("newest first expected: %v", runs[0])
	}
	if got := runsOf(e, sam); len(got) != 0 {
		t.Fatalf("sam sees bola's runs: %v", got)
	}
	if r := e.call("GET", "/api/splitreceive/runs", "", nil); r.Status != 401 {
		t.Fatalf("%d", r.Status)
	}
}

func TestRetryAFailedShare(t *testing.T) {
	e := newEnv(t)
	bola, sam := e.bolaRules(), e.sam()
	savID := e.accountID(bola, "savanna")
	savBefore := e.balanceOf(bola, "savanna")
	e.fail(savID, "", "bank_unavailable") // the savings share cannot be delivered, the spending share can
	e.credit(bola, 20000000, true)
	e.call("POST", "/api/splitreceive/scan", bola, nil)
	run := runsOf(e, bola)[0].(map[string]any)
	id := run["id"].(string)
	if run["state"] != "partial" || run["sent_kobo"] != float64(8000000) || run["held_kobo"] != float64(6000000) || run["kept_kobo"] != float64(6000000) {
		t.Fatalf("shares are independent: the 40 percent share is sent, the 30 percent share is held: %v", run)
	}
	legs := run["legs"].([]any)
	if legs[0].(map[string]any)["state"] != "failed" || legs[0].(map[string]any)["error"] != "bank_unavailable" || legs[1].(map[string]any)["state"] != "sent" {
		t.Fatalf("legs: %v", legs)
	}

	t.Run("another user cannot retry it", func(t *testing.T) {
		if r := e.call("POST", "/api/splitreceive/runs/"+id+"/retry", sam, nil); r.Status != 404 {
			t.Fatalf("%d", r.Status)
		}
	})
	t.Run("retry while the fault lasts keeps it held", func(t *testing.T) {
		r := e.call("POST", "/api/splitreceive/runs/"+id+"/retry", bola, nil)
		if r.Status != 200 || get(r.Body, "run", "state") != "partial" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
	t.Run("retry after the fault clears sends it once", func(t *testing.T) {
		e.fail(savID, "", "none")
		r := e.call("POST", "/api/splitreceive/runs/"+id+"/retry", bola, nil)
		if r.Status != 200 || get(r.Body, "run", "state") != "completed" || get(r.Body, "run", "held_kobo") != float64(0) || get(r.Body, "run", "sent_kobo") != float64(14000000) {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		if got := e.balanceOf(bola, "savanna") - savBefore; got != 6000000 {
			t.Fatalf("savings rose by %v", got)
		}
		again := e.call("POST", "/api/splitreceive/runs/"+id+"/retry", bola, nil)
		if again.Status != 409 || errCode(again) != "nothing_to_retry" {
			t.Fatalf("%d %v", again.Status, again.Body)
		}
		if got := e.balanceOf(bola, "savanna") - savBefore; got != 6000000 {
			t.Fatalf("retry paid twice: %v", got)
		}
	})
	t.Run("unknown run", func(t *testing.T) {
		if r := e.call("POST", "/api/splitreceive/runs/run_nope/retry", bola, nil); r.Status != 404 {
			t.Fatalf("%d", r.Status)
		}
	})
}

func TestRetryAfterATimeoutDoesNotPayTwice(t *testing.T) {
	e := newEnv(t)
	bola := e.bolaRules()
	savBefore := e.balanceOf(bola, "savanna")
	e.fail(e.accountID(bola, "savanna"), "", "timeout") // moves the money, reports a timeout once
	e.credit(bola, 10000000, true)
	e.call("POST", "/api/splitreceive/scan", bola, nil)
	if got := e.balanceOf(bola, "savanna") - savBefore; got != 3000000 {
		t.Fatalf("a timeout then a safe retry must pay exactly once: %v", got)
	}
}
