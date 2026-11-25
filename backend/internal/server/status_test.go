package server

import (
	"reflect"
	"testing"
	"time"
)

func status(e *env, tok, id string) reply {
	e.t.Helper()
	return e.call("GET", "/api/splitpay/"+id, tok, nil)
}

func TestPaymentStatus(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	p := e.pay(sam, execBody(30000, outside("delta", "1000000001"), "4821"), "st-1")
	id := p.Body["id"].(string)
	t.Run("owner reads it", func(t *testing.T) {
		r := status(e, sam, id)
		if r.Status != 200 || r.Body["state"] != "completed" || r.Body["total_amount_kobo"] != float64(3000000) {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		if get(r.Body, "recipient", "account_name") != "Ada Okafor" || get(r.Body, "recipient", "masked_account_number") != "******0001" {
			t.Fatalf("recipient: %v", r.Body["recipient"])
		}
		for _, k := range []string{"reversal_attempt_count", "next_reversal_retry_at", "last_reversal_attempt_at", "human_review", "legs"} {
			if _, ok := r.Body[k]; !ok {
				t.Fatalf("missing %s", k)
			}
		}
	})
	t.Run("another user gets 404, never the payment", func(t *testing.T) {
		if r := status(e, e.bola(), id); r.Status != 404 || errCode(r) != "not_found" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
	t.Run("unknown id and auth", func(t *testing.T) {
		if r := status(e, sam, "pay_nope"); r.Status != 404 {
			t.Fatalf("%d", r.Status)
		}
		if r := status(e, "", id); r.Status != 401 {
			t.Fatalf("%d", r.Status)
		}
	})
}

func TestReversalOutageHoldsThenRetriesToRolledBack(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	w := e.s.World()
	start := map[string]float64{}
	for _, b := range []string{"apex", "delta", "savanna", "harbor"} {
		start[b] = e.balanceOf(sam, b)
	}
	e.fail(e.accountID(sam, "savanna"), "", "insufficient_funds") // the third leg fails, so a rollback starts
	e.fail("", "delta", "reversal_down")                          // and the delta refund cannot go through
	r := e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "ro-1")
	id := r.Body["id"].(string)
	if r.Body["state"] != "needs_attention" || r.Body["reversal_attempt_count"] != float64(1) || r.Body["human_review"] != false {
		t.Fatalf("%v", r.Body)
	}
	if got := legStates(r); !reflect.DeepEqual(got, []string{"collected", "collected", "failed"}) {
		t.Fatalf("leg states: %v", got)
	}
	if r.Body["next_reversal_retry_at"] == nil || r.Body["last_reversal_attempt_at"] == nil {
		t.Fatalf("retry timing missing: %v", r.Body)
	}
	// The money is held safely in the collector: delta's share is in apex, nothing is lost.
	var held float64
	for b := range start {
		held += e.balanceOf(sam, b)
	}
	want := 0.0
	for _, v := range start {
		want += v
	}
	// Delta's share moved into the collector, still inside the user's own accounts. Only the
	// N10 fee is outside, and it comes back with the reversal.
	if want-held != 1000 {
		t.Fatalf("during the hold nothing may be lost except the pending N10 fee: short by %v", want-held)
	}
	if e.balanceOf(sam, "apex") != start["apex"]+5999000 {
		t.Fatalf("delta's share should be held in the collector account: %v", e.balanceOf(sam, "apex"))
	}
	// The outage lasts: the worker keeps retrying with the same reference and the count grows.
	for i := 1; i <= 3; i++ {
		e.advance(time.Duration(i) * time.Hour)
		w.RetryReversals()
	}
	if got := status(e, sam, id).Body; got["state"] != "needs_attention" || got["reversal_attempt_count"] != float64(4) {
		t.Fatalf("after 3 retries: %v", got)
	}
	// Retrying earlier than next_reversal_retry_at does nothing.
	w.RetryReversals()
	if got := status(e, sam, id).Body["reversal_attempt_count"]; got != float64(4) {
		t.Fatalf("worker ignored the delay: %v", got)
	}
	// The fault clears and the next pass finishes the rollback.
	e.fail("", "delta", "none")
	e.advance(8 * time.Hour)
	w.RetryReversals()
	final := status(e, sam, id)
	if final.Body["state"] != "rolled_back" {
		t.Fatalf("%v", final.Body)
	}
	if got := legStates(final); !reflect.DeepEqual(got, []string{"reversed", "reversed", "failed"}) {
		t.Fatalf("leg states: %v", got)
	}
	for b, v := range start {
		if got := e.balanceOf(sam, b); got != v {
			t.Fatalf("%s is %v, want %v after the retry", b, got, v)
		}
	}
	// Running the worker again must not reverse a second time.
	e.advance(12 * time.Hour)
	w.RetryReversals()
	for b, v := range start {
		if got := e.balanceOf(sam, b); got != v {
			t.Fatalf("%s moved again: %v", b, got)
		}
	}
}

func TestHumanReviewAfterTenFailedAttempts(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	w := e.s.World()
	e.fail(e.accountID(sam, "savanna"), "", "insufficient_funds")
	e.fail("", "delta", "reversal_down")
	id := e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "hr-1").Body["id"].(string)
	for i := 1; i <= 12; i++ {
		e.advance(time.Duration(i) * 30 * time.Minute)
		w.RetryReversals()
		s := status(e, sam, id).Body
		if s["human_review"] == true != (s["reversal_attempt_count"].(float64) >= 10) {
			t.Fatalf("human_review %v at attempt %v", s["human_review"], s["reversal_attempt_count"])
		}
	}
	s := status(e, sam, id).Body
	if s["human_review"] != true || s["state"] != "needs_attention" {
		t.Fatalf("after 10 attempts it keeps retrying but is flagged: %v", s)
	}
	e.fail("", "delta", "none")
	e.advance(8 * time.Hour)
	w.RetryReversals()
	if got := status(e, sam, id).Body["state"]; got != "rolled_back" {
		t.Fatalf("it should still recover: %v", got)
	}
}

func TestRescanPicksUpUnfinishedPayments(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	w := e.s.World()
	e.fail(e.accountID(sam, "savanna"), "", "insufficient_funds")
	e.fail("", "delta", "reversal_down")
	id := e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "rs-1").Body["id"].(string)
	e.fail("", "delta", "none")
	if n := w.Rescan(); n != 1 {
		t.Fatalf("rescan found %d", n)
	}
	w.RetryReversals() // due immediately after a rescan
	if got := status(e, sam, id).Body["state"]; got != "rolled_back" {
		t.Fatalf("%v", got)
	}
}
