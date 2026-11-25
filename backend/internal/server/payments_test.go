package server

import (
	"strings"
	"testing"
)

func ids(r reply) []string {
	var out []string
	for _, p := range r.Body["payments"].([]any) {
		out = append(out, p.(map[string]any)["id"].(string))
	}
	return out
}

func TestPaymentsListPagesNewestFirst(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	var made []string
	for i := 0; i < 5; i++ {
		r := e.pay(sam, execBody(int64(1000+i), outside("delta", "1000000001"), "4821"), "pg-"+string(rune('a'+i)))
		made = append(made, r.Body["id"].(string))
	}
	all := e.call("GET", "/api/payments", sam, nil)
	if all.Status != 200 || len(ids(all)) != 5 || ids(all)[0] != made[4] || ids(all)[4] != made[0] {
		t.Fatalf("not newest first: %v want reverse of %v", ids(all), made)
	}
	if all.Body["next_before"] != nil {
		t.Fatalf("no more pages expected: %v", all.Body["next_before"])
	}
	page1 := e.call("GET", "/api/payments?limit=2", sam, nil)
	if len(ids(page1)) != 2 || page1.Body["next_before"] != ids(page1)[1] {
		t.Fatalf("page1: %v next=%v", ids(page1), page1.Body["next_before"])
	}
	page2 := e.call("GET", "/api/payments?limit=2&before="+page1.Body["next_before"].(string), sam, nil)
	if ids(page2)[0] != made[2] || ids(page2)[1] != made[1] {
		t.Fatalf("page2: %v", ids(page2))
	}
	page3 := e.call("GET", "/api/payments?limit=2&before="+page2.Body["next_before"].(string), sam, nil)
	if len(ids(page3)) != 1 || ids(page3)[0] != made[0] || page3.Body["next_before"] != nil {
		t.Fatalf("page3: %v", ids(page3))
	}
}

func TestPaymentsAreOwnerOnly(t *testing.T) {
	e := newEnv(t)
	sam, bola := e.sam(), e.bola()
	mine := e.pay(sam, execBody(1000, outside("delta", "1000000001"), "4821"), "own-1").Body["id"].(string)
	if got := ids(e.call("GET", "/api/payments", bola, nil)); len(got) != 0 {
		t.Fatalf("bola sees %v", got)
	}
	if r := e.call("GET", "/api/payments?before="+mine, bola, nil); r.Status != 400 || errCode(r) != "invalid_cursor" {
		t.Fatalf("another user's id as cursor: %d %v", r.Status, r.Body)
	}
	if r := e.call("GET", "/api/payments", "", nil); r.Status != 401 {
		t.Fatalf("%d", r.Status)
	}
	if r := e.call("GET", "/api/recipients/recent", "", nil); r.Status != 401 {
		t.Fatalf("%d", r.Status)
	}
	if got := e.call("GET", "/api/recipients/recent", bola, nil).Body["recipients"].([]any); len(got) != 0 {
		t.Fatalf("bola sees recipients: %v", got)
	}
}

func TestSendToAnotherApapoUser(t *testing.T) {
	e := newEnv(t)
	sam, bola := e.sam(), e.bola()
	bolaBefore := e.balanceOf(bola, "apex")
	r := e.pay(sam, execBody(20000, apapoTo("08098765432"), "4821"), "p2p-1")
	if r.Status != 202 || r.Body["state"] != "completed" {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	if get(r.Body, "recipient", "recipient_type") != "apapo" || get(r.Body, "recipient", "masked_display_name") != "Bola A." {
		t.Fatalf("recipient: %v", r.Body["recipient"])
	}
	if got := e.balanceOf(bola, "apex") - bolaBefore; got != 2000000 {
		t.Fatalf("Bola's receiving account rose by %v, want 2000000", got)
	}
	feed := e.call("GET", "/api/dashboard?limit=3", bola, nil).Body["transactions"].([]any)
	top := feed[0].(map[string]any)
	if top["direction"] != "credit" || top["amount_kobo"] != float64(2000000) || top["counterparty"] != "Samuel P." || top["verification"] != "verified" {
		t.Fatalf("Bola's feed: %v", top)
	}
	raw := string(mustJSON(r.Body))
	for _, leak := range []string{"2009009009", "Adeyemi", "08098765432", "+2348098765432"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("payment response leaked %q: %s", leak, raw)
		}
	}
	if r := e.pay(sam, execBody(1000, apapoTo("08012345678"), "4821"), "p2p-self"); r.Status != 400 || errCode(r) != "self_send" {
		t.Fatalf("self send: %d %v", r.Status, r.Body)
	}
	if r := e.pay(sam, execBody(1000, apapoTo("08011110000"), "4821"), "p2p-none"); r.Status != 404 {
		t.Fatalf("unknown: %d", r.Status)
	}
}

func TestP2PRollsBackIfTheRecipientBankIsDown(t *testing.T) {
	e := newEnv(t)
	sam, bola := e.sam(), e.bola()
	before, bolaBefore := e.total(sam), e.total(bola)
	e.fail("", "apex", "bank_unavailable") // the planner skips Sam's Apex account; Bola's Apex account cannot receive
	r := e.pay(sam, execBody(100000, apapoTo("08098765432"), "4821"), "p2p-down")
	if r.Body["state"] != "rolled_back" || len(r.Body["legs"].([]any)) != 2 {
		t.Fatalf("expected a two leg payment that rolled back: %v", r.Body)
	}
	e.fail("", "apex", "none")
	if r.Status != 202 {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	if e.total(sam) != before || e.total(bola) != bolaBefore {
		t.Fatalf("balances changed: sam %v/%v bola %v/%v (state %v)", e.total(sam), before, e.total(bola), bolaBefore, r.Body["state"])
	}
}

func TestRecentRecipientsAreMaskedAndReusable(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	e.pay(sam, execBody(1000, outside("delta", "1000000001"), "4821"), "rc-1")
	e.pay(sam, execBody(1000, apapoTo("08098765432"), "4821"), "rc-2")
	e.pay(sam, execBody(2000, outside("delta", "1000000001"), "4821"), "rc-3") // same payee again: listed once
	r := e.call("GET", "/api/recipients/recent", sam, nil)
	list := r.Body["recipients"].([]any)
	if r.Status != 200 || len(list) != 2 {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	raw := string(mustJSON(list))
	for _, leak := range []string{"1000000001", "2009009009", "Adeyemi", "08098765432"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("recent recipients leaked %q: %s", leak, raw)
		}
	}
	first, second := list[0].(map[string]any), list[1].(map[string]any)
	if get(first, "recipient", "recipient_type") != "outside" || get(second, "recipient", "masked_display_name") != "Bola A." {
		t.Fatalf("order or content: %v", list)
	}
	// Pay Bola again straight from the recent list, without ever sending her phone.
	again := e.pay(sam, execBody(1500, map[string]any{"recent_id": second["id"]}, "4821"), "rc-4")
	if again.Status != 202 || again.Body["state"] != "completed" || get(again.Body, "recipient", "masked_display_name") != "Bola A." {
		t.Fatalf("%d %v", again.Status, again.Body)
	}
	// Someone else's recent id is not usable.
	if r := e.pay(e.bola(), execBody(1000, map[string]any{"recent_id": second["id"]}, "5912"), "rc-5"); r.Status != 404 {
		t.Fatalf("foreign recent_id: %d %v", r.Status, r.Body)
	}
}
