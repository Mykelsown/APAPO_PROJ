package server

import (
	"strings"
	"testing"
)

func TestPutAndGetRules(t *testing.T) {
	e := newEnv(t)
	bola := e.bola()
	e.call("POST", "/api/banks/link", bola, linkBody("delta"))
	e.call("POST", "/api/banks/link", bola, linkBody("savanna"))
	apex, delta, sav := e.accountID(bola, "apex"), e.accountID(bola, "delta"), e.accountID(bola, "savanna")
	sam := e.sam()
	samApex := e.accountID(sam, "apex")
	fixed := map[string]any{"account_id": delta, "amount_kobo": 500000}
	outsideDest := map[string]any{"bank_code": "harbor", "account_number": "1000000001", "percent": 10.5}

	if r := e.call("GET", "/api/splitreceive/rules", bola, nil); r.Status != 200 || r.Body["rules"] != nil {
		t.Fatalf("no rules yet should give null: %d %v", r.Status, r.Body)
	}
	tests := []struct {
		name  string
		token string
		body  any
		want  int
		code  string
	}{
		{"valid story rules", bola, rulesBody(apex, "5912", ownPct(sav, 30), ownPct(delta, 40)), 200, ""},
		{"fixed and outside destinations", bola, rulesBody(apex, "5912", fixed, outsideDest), 200, ""},
		{"empty destinations clears the split", bola, rulesBody(apex, "5912"), 200, ""},
		{"no standing consent", bola, map[string]any{"receiving_account_id": apex, "pin": "5912", "destinations": []any{}}, 400, "consent_required"},
		{"consent false", bola, map[string]any{"receiving_account_id": apex, "standing_consent": false, "pin": "5912"}, 400, "consent_required"},
		{"wrong pin", bola, rulesBody(apex, "0001", ownPct(sav, 30)), 401, "invalid_pin"},
		{"missing pin", bola, rulesBody(apex, "", ownPct(sav, 30)), 401, "invalid_pin"},
		{"more than 100 percent", bola, rulesBody(apex, "5912", ownPct(sav, 60), ownPct(delta, 50)), 400, "percent_total_exceeds_100"},
		{"three decimals", bola, rulesBody(apex, "5912", ownPct(sav, 33.333)), 400, "invalid_destination"},
		{"zero percent", bola, rulesBody(apex, "5912", ownPct(sav, 0)), 400, "invalid_destination"},
		{"percent and amount together", bola, rulesBody(apex, "5912", map[string]any{"account_id": sav, "percent": 10, "amount_kobo": 5}), 400, "invalid_destination"},
		{"neither percent nor amount", bola, rulesBody(apex, "5912", map[string]any{"account_id": sav}), 400, "invalid_destination"},
		{"negative fixed amount", bola, rulesBody(apex, "5912", map[string]any{"account_id": sav, "amount_kobo": -5}), 400, "invalid_destination"},
		{"six destinations", bola, rulesBody(apex, "5912", ownPct(sav, 1), ownPct(sav, 1), ownPct(sav, 1), ownPct(sav, 1), ownPct(sav, 1), ownPct(sav, 1)), 400, "too_many_destinations"},
		{"destination is someone else's account", bola, rulesBody(apex, "5912", ownPct(samApex, 10)), 400, "invalid_destination"},
		{"destination is the receiving account itself", bola, rulesBody(apex, "5912", ownPct(apex, 10)), 400, "invalid_destination"},
		{"receiving account is not mine", bola, rulesBody(samApex, "5912", ownPct(sav, 10)), 400, "invalid_receiving_account"},
		{"bad outside account", bola, rulesBody(apex, "5912", map[string]any{"bank_code": "harbor", "account_number": "12", "percent": 5}), 400, "invalid_account_number"},
		{"not onboarded", e.signUp("08075550000"), rulesBody(apex, "5912"), 403, "onboarding_required"},
		{"no token", "", rulesBody(apex, "5912"), 401, "unauthorized"},
		{"bad json", bola, "{", 400, "invalid_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("PUT", "/api/splitreceive/rules", tc.token, tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
		})
	}
	t.Run("the response wraps rules, and GET returns the same", func(t *testing.T) {
		put := e.call("PUT", "/api/splitreceive/rules", bola, rulesBody(apex, "5912", ownPct(sav, 30), ownPct(delta, 40)))
		rules, ok := put.Body["rules"].(map[string]any)
		if !ok || rules["receiving_account_id"] != apex || rules["standing_consent"] != true || rules["consent_at"] == nil {
			t.Fatalf("put: %v", put.Body)
		}
		dests := rules["destinations"].([]any)
		if len(dests) != 2 || dests[0].(map[string]any)["percent"] != float64(30) || dests[1].(map[string]any)["bank_code"] != "delta" {
			t.Fatalf("destinations: %v", dests)
		}
		get1 := e.call("GET", "/api/splitreceive/rules", bola, nil)
		if string(mustJSON(get1.Body["rules"])) != string(mustJSON(rules)) {
			t.Fatalf("get differs from put:\n%s\n%s", mustJSON(get1.Body["rules"]), mustJSON(rules))
		}
		if strings.Contains(string(mustJSON(get1.Body)), "pin") {
			t.Fatal("rules must never echo the PIN")
		}
	})
	t.Run("rules are private to their owner", func(t *testing.T) {
		if r := e.call("GET", "/api/splitreceive/rules", sam, nil); r.Body["rules"] != nil {
			t.Fatalf("sam sees %v", r.Body)
		}
	})
	t.Run("the receiving account becomes the one P2P money lands in", func(t *testing.T) {
		me := e.call("GET", "/api/me", bola, nil)
		if get(me.Body, "receiving_account", "account_id") != apex {
			t.Fatalf("%v", me.Body["receiving_account"])
		}
	})
}
