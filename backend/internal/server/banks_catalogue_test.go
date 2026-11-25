package server

import "testing"

func TestBankCatalogue(t *testing.T) {
	e := newEnv(t)
	r := e.call("GET", "/api/banks", e.sam(), nil)
	if r.Status != 200 {
		t.Fatalf("%d", r.Status)
	}
	banks := r.Body["banks"].([]any)
	supported, soon := 0, 0
	for _, b := range banks {
		m := b.(map[string]any)
		if m["code"] == "" || m["name"] == "" {
			t.Fatalf("incomplete: %v", m)
		}
		if m["supported"] == true {
			supported++
		} else {
			soon++
		}
	}
	if supported != 4 || soon != 2 {
		t.Fatalf("supported=%d soon=%d", supported, soon)
	}
	if r := e.call("GET", "/api/banks", "", nil); r.Status != 401 {
		t.Fatalf("no token: %d", r.Status)
	}
}
