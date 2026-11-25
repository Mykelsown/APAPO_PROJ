package server

import (
	"reflect"
	"testing"
)

func TestExecuteCompletesAThreeLegPayment(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	before := e.total(sam)
	r := e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "k-150")
	if r.Status != 202 {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	if r.Body["id"] == "" || r.Body["state"] != "completed" {
		t.Fatalf("body: %v", r.Body)
	}
	if got := legStates(r); !reflect.DeepEqual(got, []string{"collected", "collected", "collected"}) {
		t.Fatalf("leg states: %v", got)
	}
	if after := e.total(sam); before-after != 15000000+3000 {
		t.Fatalf("balance fell by %v, want amount plus N30 fees", before-after)
	}
	// Status endpoint is added in the next commit; the payment is visible in the history feed.
	feed := e.call("GET", "/api/dashboard?limit=10", sam, nil).Body["transactions"].([]any)
	if feed[0].(map[string]any)["category"] != "transfer" {
		t.Fatalf("debits should appear in the feed: %v", feed[0])
	}
}

func TestExecuteValidation(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	good := execBody(5000, outside("delta", "1000000001"), "4821")
	t.Run("idempotency key is required", func(t *testing.T) {
		r := e.call("POST", "/api/splitpay/execute", sam, good)
		if r.Status != 400 || errCode(r) != "idempotency_key_required" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
	t.Run("auth", func(t *testing.T) {
		if r := e.pay("", good, "k"); r.Status != 401 {
			t.Fatalf("%d", r.Status)
		}
	})
	t.Run("not onboarded", func(t *testing.T) {
		if r := e.pay(e.signUp("08072220000"), good, "k"); r.Status != 403 {
			t.Fatalf("%d", r.Status)
		}
	})
	t.Run("wrong pin moves nothing", func(t *testing.T) {
		before := e.total(sam)
		r := e.pay(sam, execBody(5000, outside("delta", "1000000001"), "0001"), "k-bad-pin")
		if r.Status != 401 || errCode(r) != "invalid_pin" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		if e.total(sam) != before {
			t.Fatal("money moved on a wrong PIN")
		}
	})
	t.Run("missing pin", func(t *testing.T) {
		b := previewBody(5000, outside("delta", "1000000001"))
		if r := e.pay(sam, b, "k-nopin"); r.Status != 401 || errCode(r) != "invalid_pin" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
	t.Run("validation errors come from the planner", func(t *testing.T) {
		if r := e.pay(sam, execBody(0, outside("delta", "1000000001"), "4821"), "k-0"); r.Status != 400 || errCode(r) != "invalid_amount" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		if r := e.pay(sam, execBody(500000, outside("delta", "1000000001"), "4821"), "k-big"); r.Status != 422 || errCode(r) != "insufficient_funds" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
	t.Run("extra recipient display fields are fine", func(t *testing.T) {
		rec := outside("delta", "1000000001")
		rec["recipient_type"] = "outside"
		rec["masked_display_name"] = "x"
		if r := e.pay(sam, execBody(2000, rec, "4821"), "k-display"); r.Status != 202 {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
}

func TestExecuteIsIdempotent(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	body := execBody(20000, outside("delta", "1000000001"), "4821")
	before := e.total(sam)
	first := e.pay(sam, body, "same-key")
	second := e.pay(sam, body, "same-key")
	if first.Status != 202 || second.Status != 202 || first.Body["id"] != second.Body["id"] {
		t.Fatalf("same key and body must return the same payment: %v / %v", first.Body["id"], second.Body["id"])
	}
	if second.Header.Get("Idempotent-Replay") != "true" {
		t.Fatal("replay header missing")
	}
	if got := before - e.total(sam); got != 2000000+1000 {
		t.Fatalf("paid twice: balance fell by %v", got)
	}
	other := execBody(21000, outside("delta", "1000000001"), "4821")
	r := e.pay(sam, other, "same-key")
	if r.Status != 409 || errCode(r) != "idempotency_conflict" {
		t.Fatalf("different body, same key: %d %v", r.Status, r.Body)
	}
	// Another user may reuse the same key string.
	if r := e.pay(e.bola(), execBody(1000, outside("delta", "1000000001"), "5912"), "same-key"); r.Status != 202 {
		t.Fatalf("keys are per user: %d %v", r.Status, r.Body)
	}
}

func TestExecutePINLockout(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	for i := 0; i < 4; i++ {
		if r := e.pay(sam, execBody(1000, outside("delta", "1000000001"), "0002"), "lk"+string(rune('a'+i))); r.Status != 401 {
			t.Fatalf("try %d: %d", i, r.Status)
		}
	}
	r := e.pay(sam, execBody(1000, outside("delta", "1000000001"), "0002"), "lk-last")
	if r.Status != 423 || errCode(r) != "pin_locked" {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	if r := e.pay(sam, execBody(1000, outside("delta", "1000000001"), "4821"), "lk-right"); r.Status != 423 {
		t.Fatalf("right PIN must also be refused while locked: %d", r.Status)
	}
}

func TestExecuteHighRiskNeedsAcknowledgement(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	body := execBody(5000, outside("apex", "9000000001"), "4821")
	before := e.total(sam)
	r := e.pay(sam, body, "flagged-1")
	if r.Status != 409 || errCode(r) != "risk_acknowledgement_required" {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	if e.total(sam) != before {
		t.Fatal("money moved without acknowledgement")
	}
	body["acknowledge_risk"] = true
	if r := e.pay(sam, body, "flagged-2"); r.Status != 202 || r.Body["state"] != "completed" {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
}

func TestExecuteRollsBackOnDefiniteFailure(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	start := map[string]float64{}
	for _, b := range []string{"apex", "delta", "savanna", "harbor"} {
		start[b] = e.balanceOf(sam, b)
	}
	e.fail(e.accountID(sam, "savanna"), "", "insufficient_funds") // the third leg will fail
	r := e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "rb-1")
	if r.Status != 202 || r.Body["state"] != "rolled_back" {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	if got := legStates(r); !reflect.DeepEqual(got, []string{"reversed", "reversed", "failed"}) {
		t.Fatalf("leg states: %v", got)
	}
	if r.Body["failure_reason"] != "insufficient_funds" {
		t.Fatalf("failure reason: %v", r.Body["failure_reason"])
	}
	for b, want := range start {
		if got := e.balanceOf(sam, b); got != want {
			t.Fatalf("%s balance %v, want %v back", b, got, want)
		}
	}
}

func TestExecuteRollsBackWhenThePayoutFails(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	before := e.total(sam)
	e.fail(e.accountID(sam, "apex"), "", "insufficient_funds") // the collector cannot send
	r := e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "po-1")
	if r.Body["state"] != "rolled_back" {
		t.Fatalf("%v", r.Body)
	}
	if got := legStates(r); !reflect.DeepEqual(got, []string{"reversed", "reversed", "reversed"}) {
		t.Fatalf("leg states: %v", got)
	}
	if e.total(sam) != before {
		t.Fatalf("balance changed after rollback: %v vs %v", e.total(sam), before)
	}
}

func TestExecuteTimeoutIsRetriedWithoutDoubleDebit(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	before := e.total(sam)
	e.fail(e.accountID(sam, "delta"), "", "timeout")
	r := e.pay(sam, execBody(150000, outside("delta", "1000000001"), "4821"), "to-1")
	if r.Body["state"] != "completed" {
		t.Fatalf("%v", r.Body)
	}
	if got := before - e.total(sam); got != 15000000+3000 {
		t.Fatalf("timeout retry changed the total debit: %v", got)
	}
}
