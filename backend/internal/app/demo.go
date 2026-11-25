package app

import (
	"net/http"
	"strings"
)

// SimulateIn is the body of POST /api/demo/simulate-credit.
type SimulateIn struct {
	AmountKobo int64  `json:"amount_kobo"`
	SenderName string `json:"sender_name"`
	Verified   *bool  `json:"verified"`
	AccountID  string `json:"account_id"`
	Scan       *bool  `json:"scan"`
}

// SimulateOut describes the credit that arrived and any runs it triggered.
type SimulateOut struct {
	CreditID string    `json:"credit_id"`
	Verified bool      `json:"verified"`
	Runs     []RunView `json:"runs"`
}

// SimulateCredit makes money arrive for a user, as if a bank posted a credit. Demo only.
// A confirmed credit changes the balance at once; an unconfirmed one shows in the feed but
// not in the balance. Unless scan is false, Split-Receive runs straight away.
func (w *World) SimulateCredit(uid string, in SimulateIn) (*SimulateOut, *Error) {
	if in.AmountKobo < 100 || in.AmountKobo > maxAmount {
		return nil, Errf(http.StatusBadRequest, "invalid_amount", "amount_kobo must be at least 100.", nil)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	u, e := w.onboarded(uid)
	if e != nil {
		return nil, e
	}
	var acc *Account
	if in.AccountID != "" {
		acc = w.accounts[in.AccountID]
		if acc == nil || acc.UserID != uid || !w.usable(acc) {
			return nil, errNotFound()
		}
	} else if acc = w.receiving(u); acc == nil {
		return nil, Errf(http.StatusConflict, "no_receiving_account", "Link a bank first.", nil)
	}
	verified := in.Verified == nil || *in.Verified
	sender := strings.TrimSpace(in.SenderName)
	if sender == "" {
		sender = "Acme Payroll Ltd"
	}
	now := w.now()
	c := &Credit{ID: w.nextID("crd"), UserID: uid, AccountID: acc.ID, Amount: in.AmountKobo, Sender: sender,
		Reference: "bank_" + randHex(4), Verified: verified, At: now}
	w.credits = append(w.credits, c)
	if verified {
		acc.Balance += c.Amount
	}
	w.addTx(acc, "credit", c.Amount, sender, "income", verified, now)
	out := &SimulateOut{CreditID: c.ID, Verified: verified, Runs: []RunView{}}
	if in.Scan == nil || *in.Scan {
		for _, r := range w.scanLocked(uid) {
			out.Runs = append(out.Runs, w.runView(r))
		}
	}
	return out, nil
}
