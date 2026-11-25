package server

import "testing"

func accountIDs(r reply) []string {
	var out []string
	for _, a := range r.Body["accounts"].([]any) {
		out = append(out, a.(map[string]any)["id"].(string))
	}
	return out
}

func TestAccountsAndRevoke(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	r := e.call("GET", "/api/accounts", sam, nil)
	if r.Status != 200 || len(accountIDs(r)) != 4 {
		t.Fatalf("seeded user should have 4 accounts: %d %v", r.Status, r.Body)
	}
	first := r.Body["accounts"].([]any)[0].(map[string]any)
	if first["bank_code"] != "apex" || first["balance_kobo"] != float64(7000000) || first["is_receiving"] != true {
		t.Fatalf("first account: %v", first)
	}
	linkID := first["link_id"].(string)

	t.Run("another user cannot revoke my link", func(t *testing.T) {
		if r := e.call("DELETE", "/api/banks/"+linkID, e.bola(), nil); r.Status != 404 || errCode(r) != "not_found" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
	t.Run("unknown link", func(t *testing.T) {
		if r := e.call("DELETE", "/api/banks/lnk_missing", sam, nil); r.Status != 404 {
			t.Fatalf("%d", r.Status)
		}
	})
	t.Run("auth", func(t *testing.T) {
		if r := e.call("GET", "/api/accounts", "", nil); r.Status != 401 {
			t.Fatalf("%d", r.Status)
		}
		if r := e.call("DELETE", "/api/banks/"+linkID, "", nil); r.Status != 401 {
			t.Fatalf("%d", r.Status)
		}
	})
	t.Run("revoke removes the account and moves the receiving account", func(t *testing.T) {
		if r := e.call("DELETE", "/api/banks/"+linkID, sam, nil); r.Status != 200 || r.Body["revoked"] != true {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		if got := len(accountIDs(e.call("GET", "/api/accounts", sam, nil))); got != 3 {
			t.Fatalf("accounts after revoke = %d", got)
		}
		me := e.call("GET", "/api/me", sam, nil)
		if get(me.Body, "receiving_account", "bank_code") == "apex" {
			t.Fatalf("receiving account still points at the revoked bank")
		}
	})
	t.Run("relinking brings the account back with its balance", func(t *testing.T) {
		if r := e.call("POST", "/api/banks/link", sam, linkBody("apex")); r.Status != 201 || get(r.Body, "account", "balance_kobo") != float64(7000000) {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
		if got := len(accountIDs(e.call("GET", "/api/accounts", sam, nil))); got != 4 {
			t.Fatalf("accounts = %d", got)
		}
	})
}
