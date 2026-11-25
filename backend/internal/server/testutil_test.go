package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mykelsown/apapo-backend/internal/app"
	"github.com/Mykelsown/apapo-backend/internal/config"
)

type env struct {
	t *testing.T
	s *Server
	h http.Handler
}

type reply struct {
	Status int
	Body   map[string]any
	Header http.Header
}

func newEnv(t *testing.T) *env {
	t.Helper()
	cfg, err := config.Load(func(k string) (string, bool) {
		return map[string]string{"APP_SECRET": "test-secret", "ALLOWED_ORIGIN": "*"}[k], k == "APP_SECRET" || k == "ALLOWED_ORIGIN"
	}, "test")
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg)
	s.world.Async = false
	return &env{t: t, s: s, h: s.Handler()}
}

// call sends a request. body may be nil, a string (raw) or any JSON-able value.
func (e *env) call(method, path, token string, body any, hdr ...string) reply {
	e.t.Helper()
	var buf bytes.Buffer
	switch b := body.(type) {
	case nil:
	case string:
		buf.WriteString(b)
	default:
		if err := json.NewEncoder(&buf).Encode(b); err != nil {
			e.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.RemoteAddr = "203.0.113.9:4000"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rr := httptest.NewRecorder()
	e.h.ServeHTTP(rr, req)
	out := map[string]any{}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return reply{Status: rr.Code, Body: out, Header: rr.Header()}
}

// login signs in with phone and PIN and returns the token.
func (e *env) login(phone, pin string) string {
	e.t.Helper()
	r := e.call("POST", "/api/auth/login", "", map[string]string{"phone": phone, "pin": pin})
	if r.Status != 200 {
		e.t.Fatalf("login failed: %d %v", r.Status, r.Body)
	}
	return r.Body["access_token"].(string)
}

func (e *env) sam() string  { return e.login("08012345678", "4821") }
func (e *env) bola() string { return e.login("08098765432", "5912") }

func errCode(r reply) string {
	if m, ok := r.Body["error"].(map[string]any); ok {
		s, _ := m["code"].(string)
		return s
	}
	return ""
}

func (e *env) advance(d time.Duration) {
	now := time.Now().UTC().Add(d)
	e.s.world.SetClock(func() time.Time { return now })
}

func get(m map[string]any, path ...string) any {
	var cur any = m
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[p]
	}
	return cur
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

const time_minute = time.Minute

// onboarded creates a fully onboarded new user and returns the token.
func (e *env) onboarded(phone, pin string) string {
	e.t.Helper()
	tok := e.signUp(phone)
	if r := e.call("POST", "/api/auth/bvn", tok, map[string]string{"bvn": "22200011122"}); r.Status != 200 {
		e.t.Fatal(r.Body)
	}
	if r := e.call("POST", "/api/auth/pin", tok, map[string]string{"pin": pin}); r.Status != 200 {
		e.t.Fatal(r.Body)
	}
	return tok
}

var fullConsent = map[string]any{"accepted": true, "scopes": []string{"accounts", "balances", "transactions"}}

func linkBody(bank string) map[string]any {
	return map[string]any{"bank_code": bank, "consent": fullConsent}
}

const naira = 100

func outside(bank, number string) map[string]any {
	return map[string]any{"bank_code": bank, "account_number": number}
}

func apapoTo(phone string) map[string]any { return map[string]any{"apapo_phone": phone} }

func previewBody(amountNaira int64, rec map[string]any) map[string]any {
	return map[string]any{"amount_kobo": amountNaira * naira, "recipient": rec}
}

func init() {
	log.SetOutput(io.Discard)
	app.PINIterations = 1000
}

func execBody(amountNaira int64, rec map[string]any, pin string) map[string]any {
	b := previewBody(amountNaira, rec)
	b["pin"] = pin
	return b
}

// pay runs execute with a fresh idempotency key.
func (e *env) pay(token string, body map[string]any, key string) reply {
	e.t.Helper()
	return e.call("POST", "/api/splitpay/execute", token, body, "Idempotency-Key", key)
}

func (e *env) total(token string) float64 {
	e.t.Helper()
	return e.call("GET", "/api/dashboard", token, nil).Body["total_balance_kobo"].(float64)
}

func (e *env) accountID(token, bank string) string {
	e.t.Helper()
	for _, a := range e.call("GET", "/api/accounts", token, nil).Body["accounts"].([]any) {
		m := a.(map[string]any)
		if m["bank_code"] == bank {
			return m["id"].(string)
		}
	}
	e.t.Fatalf("no %s account", bank)
	return ""
}

func (e *env) balanceOf(token, bank string) float64 {
	e.t.Helper()
	for _, a := range e.call("GET", "/api/accounts", token, nil).Body["accounts"].([]any) {
		m := a.(map[string]any)
		if m["bank_code"] == bank {
			return m["balance_kobo"].(float64)
		}
	}
	return -1
}

func (e *env) fail(account, bank, mode string) {
	e.t.Helper()
	if err := e.s.World().SetFailure(account, bank, mode); err != nil {
		e.t.Fatal(err)
	}
}

func legStates(r reply) []string {
	var out []string
	for _, l := range r.Body["legs"].([]any) {
		out = append(out, l.(map[string]any)["state"].(string))
	}
	return out
}

const time_hour = time.Hour

// pct and fixed build Split-Receive destinations.
func ownPct(accountID string, p float64) map[string]any {
	return map[string]any{"account_id": accountID, "percent": p}
}

func rulesBody(recvID, pin string, dests ...map[string]any) map[string]any {
	return map[string]any{"receiving_account_id": recvID, "standing_consent": true, "pin": pin, "destinations": dests}
}

// bolaRules gives Bola the story rules: 30 percent to Savanna savings, 40 percent to Delta spending.
func (e *env) bolaRules() (tok string) {
	e.t.Helper()
	tok = e.bola()
	for _, b := range []string{"delta", "savanna"} {
		if r := e.call("POST", "/api/banks/link", tok, linkBody(b)); r.Status != 201 {
			e.t.Fatalf("link %s: %d %v", b, r.Status, r.Body)
		}
	}
	r := e.call("PUT", "/api/splitreceive/rules", tok, rulesBody(e.accountID(tok, "apex"), "5912",
		ownPct(e.accountID(tok, "savanna"), 30), ownPct(e.accountID(tok, "delta"), 40)))
	if r.Status != 200 {
		e.t.Fatalf("rules: %d %v", r.Status, r.Body)
	}
	return tok
}

// credit makes money arrive for the user through the demo helper in the domain, without scanning.
func (e *env) credit(token string, kobo int64, verified bool) string {
	e.t.Helper()
	uid, _ := e.s.World().Authenticate(token)
	f := false
	out, err := e.s.World().SimulateCredit(uid, app.SimulateIn{AmountKobo: kobo, Verified: &verified, Scan: &f})
	if err != nil {
		e.t.Fatal(err)
	}
	return out.CreditID
}
