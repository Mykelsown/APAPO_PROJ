package server

import (
	"strings"
	"testing"
)

func reportBody(bank, number, evidence string) map[string]any {
	return map[string]any{"recipient": map[string]any{"bank_code": bank, "account_number": number}, "evidence_ref": evidence}
}

func TestReports(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	tests := []struct {
		name  string
		token string
		body  any
		want  int
		code  string
	}{
		{"valid report", sam, reportBody("harbor", "5566778899", "screenshot-123"), 201, ""},
		{"same reporter and account again", sam, reportBody("harbor", "5566778899", "another"), 200, ""},
		{"evidence missing", sam, reportBody("harbor", "5566778800", ""), 400, "evidence_required"},
		{"evidence blank", sam, reportBody("harbor", "5566778800", "   "), 400, "evidence_required"},
		{"bad account number", sam, reportBody("harbor", "55", "x"), 400, "invalid_account_number"},
		{"unknown bank", sam, reportBody("nope", "5566778800", "x"), 400, "unknown_bank"},
		{"no token", "", reportBody("harbor", "5566778811", "x"), 401, "unauthorized"},
		{"bad json", sam, "{", 400, "invalid_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/reports", tc.token, tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
		})
	}
	t.Run("repeat says already_reported", func(t *testing.T) {
		r := e.call("POST", "/api/reports", sam, reportBody("harbor", "5566778899", "again"))
		if r.Status != 200 || r.Body["status"] != "already_reported" {
			t.Fatalf("%d %v", r.Status, r.Body)
		}
	})
}

func TestReportsLimitFivePerDay(t *testing.T) {
	e := newEnv(t)
	sam := e.sam()
	for i := 0; i < 5; i++ {
		n := "70000000" + string(rune('0'+i)) + "0"
		if r := e.call("POST", "/api/reports", sam, reportBody("delta", n, "ev")); r.Status != 201 {
			t.Fatalf("report %d: %d %v", i, r.Status, r.Body)
		}
	}
	r := e.call("POST", "/api/reports", sam, reportBody("delta", "7000000099", "ev"))
	if r.Status != 429 || errCode(r) != "rate_limited" || r.Header.Get("Retry-After") == "" {
		t.Fatalf("6th report: %d %v", r.Status, r.Body)
	}
	// A repeat of an account already reported is still answered, not rate limited.
	if r := e.call("POST", "/api/reports", sam, reportBody("delta", "7000000000", "ev")); r.Status != 200 {
		t.Fatalf("repeat after the limit: %d", r.Status)
	}
}

func TestThreeIndependentReportsMakeAnAccountHighRisk(t *testing.T) {
	e := newEnv(t)
	sam, bola := e.sam(), e.bola()
	third := e.onboarded("08074440000", "4821")
	target := outside("harbor", "6600000001")
	tier := func() string {
		return get(e.call("POST", "/api/risk/check", sam, previewBody(5000, target)).Body, "risk", "tier").(string)
	}
	if tier() == "high" {
		t.Fatal("clean account should not start high")
	}
	// Sam reports twice (counts once), then Bola, then a third person: only the third makes it high.
	steps := []struct {
		tok  string
		want string
	}{{sam, "unknown"}, {sam, "unknown"}, {bola, "unknown"}, {third, "high"}}
	for i, st := range steps {
		e.call("POST", "/api/reports", st.tok, reportBody("harbor", "6600000001", "ev"))
		if got := tier(); got != st.want {
			t.Fatalf("after report %d tier = %s, want %s", i, got, st.want)
		}
	}
}

func TestReportsStoreOnlyHashes(t *testing.T) {
	e := newEnv(t)
	e.call("POST", "/api/reports", e.sam(), reportBody("harbor", "6611223344", "ev"))
	w := e.s.World()
	dump := w.DebugReports()
	if strings.Contains(dump, "6611223344") || strings.Contains(dump, "usr_") || strings.Contains(dump, "Samuel") {
		t.Fatalf("report data must be salted hashes only: %s", dump)
	}
}
