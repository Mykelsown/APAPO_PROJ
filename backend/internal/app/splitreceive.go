package app

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxDestinations = 5

// DestinationIn is one Split-Receive target in a PUT body. Give account_id for one of the
// user's own accounts, or bank_code and account_number for an outside beneficiary. Give
// exactly one of percent and amount_kobo.
type DestinationIn struct {
	AccountID     string   `json:"account_id"`
	BankCode      string   `json:"bank_code"`
	AccountNumber string   `json:"account_number"`
	Label         string   `json:"label"`
	Percent       *float64 `json:"percent"`
	AmountKobo    *int64   `json:"amount_kobo"`
}

// RulesIn is the body of PUT /api/splitreceive/rules.
type RulesIn struct {
	ReceivingAccountID string          `json:"receiving_account_id"`
	StandingConsent    bool            `json:"standing_consent"`
	PIN                string          `json:"pin"`
	Destinations       []DestinationIn `json:"destinations"`
}

// DestinationView is a destination as the frontend sees it.
type DestinationView struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Label         string   `json:"label"`
	AccountID     string   `json:"account_id,omitempty"`
	BankCode      string   `json:"bank_code"`
	BankName      string   `json:"bank_name"`
	MaskedAccount string   `json:"masked_account_number"`
	AccountName   string   `json:"account_name,omitempty"`
	Percent       *float64 `json:"percent,omitempty"`
	AmountKobo    *int64   `json:"amount_kobo,omitempty"`
}

// RulesView is the rules object.
type RulesView struct {
	ReceivingAccountID string            `json:"receiving_account_id"`
	StandingConsent    bool              `json:"standing_consent"`
	ConsentAt          time.Time         `json:"consent_at"`
	Destinations       []DestinationView `json:"destinations"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

func (w *World) destView(d Destination) DestinationView {
	v := DestinationView{ID: d.ID, Kind: d.Kind, Label: d.Label, AccountID: d.AccountID, BankCode: d.BankCode,
		BankName: BankName(d.BankCode), MaskedAccount: MaskNumber(d.Number), AccountName: d.AccountName}
	if d.PercentBP > 0 {
		p := float64(d.PercentBP) / 100
		v.Percent = &p
	} else {
		a := d.AmountKobo
		v.AmountKobo = &a
	}
	return v
}

func (w *World) rulesView(r *Rules) *RulesView {
	if r == nil {
		return nil
	}
	v := &RulesView{ReceivingAccountID: r.ReceivingAccountID, StandingConsent: true, ConsentAt: r.ConsentAt,
		Destinations: []DestinationView{}, UpdatedAt: r.UpdatedAt}
	for _, d := range r.Destinations {
		v.Destinations = append(v.Destinations, w.destView(d))
	}
	return v
}

// GetRules returns the user's rules, or nil.
func (w *World) GetRules(uid string) *RulesView {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rulesView(w.rules[uid])
}

// PutRules saves the rules. Standing consent and the PIN are both required.
func (w *World) PutRules(uid string, in RulesIn) (*RulesView, *Error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	u, e := w.onboarded(uid)
	if e != nil {
		return nil, e
	}
	if !in.StandingConsent {
		return nil, Errf(http.StatusBadRequest, "consent_required", "You must agree to let Apapo split money you receive.", nil)
	}
	recv := w.accounts[in.ReceivingAccountID]
	if recv == nil || recv.UserID != uid || !w.usable(recv) {
		return nil, Errf(http.StatusBadRequest, "invalid_receiving_account", "Choose one of your linked accounts as the receiving account.", nil)
	}
	if len(in.Destinations) > maxDestinations {
		return nil, Errf(http.StatusBadRequest, "too_many_destinations", "You can split into at most 5 destinations.", nil)
	}
	var dests []Destination
	var totalBP int64
	for i, d := range in.Destinations {
		bad := func(msg string) *Error {
			return Errf(http.StatusBadRequest, "invalid_destination", msg, map[string]any{"index": i})
		}
		out := Destination{ID: "dst_" + strconv.Itoa(i+1), Label: strings.TrimSpace(d.Label)}
		switch {
		case d.Percent != nil && d.AmountKobo != nil, d.Percent == nil && d.AmountKobo == nil:
			return nil, bad("Give exactly one of percent or amount_kobo.")
		case d.Percent != nil:
			p := *d.Percent
			bp := math.Round(p * 100)
			if p <= 0 || p > 100 || math.Abs(p*100-bp) > 1e-6 {
				return nil, bad("percent must be above 0, at most 100, with at most 2 decimals.")
			}
			out.PercentBP = int64(bp)
			totalBP += out.PercentBP
		default:
			if *d.AmountKobo <= 0 || *d.AmountKobo > maxAmount {
				return nil, bad("amount_kobo must be a positive number of kobo.")
			}
			out.AmountKobo = *d.AmountKobo
		}
		if d.AccountID != "" {
			a := w.accounts[d.AccountID]
			if a == nil || a.UserID != uid || !w.usable(a) {
				return nil, bad("account_id is not one of your linked accounts.")
			}
			if a.ID == recv.ID {
				return nil, bad("A destination cannot be the receiving account itself.")
			}
			out.Kind, out.AccountID, out.BankCode, out.Number = "own", a.ID, a.BankCode, a.Number
			if out.Label == "" {
				out.Label = BankName(a.BankCode)
			}
		} else {
			name, e := w.resolveOutside(d.BankCode, d.AccountNumber)
			if e != nil {
				return nil, e
			}
			out.Kind, out.BankCode, out.Number, out.AccountName = "outside", d.BankCode, d.AccountNumber, name
			if out.Label == "" {
				out.Label = name
			}
		}
		dests = append(dests, out)
	}
	if totalBP > 10000 {
		return nil, Errf(http.StatusBadRequest, "percent_total_exceeds_100", "The percentages add up to more than 100.", nil)
	}
	if e := w.checkPIN(u, in.PIN); e != nil {
		return nil, e
	}
	now := w.now()
	r := &Rules{UserID: uid, ReceivingAccountID: recv.ID, Destinations: dests, ConsentAt: now, UpdatedAt: now}
	w.rules[uid] = r
	u.ReceivingAccountID = recv.ID // money sent to this user by phone lands here, and is split
	return w.rulesView(r), nil
}

// RunLegView is one share of a run.
type RunLegView struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Label         string `json:"label"`
	BankName      string `json:"bank_name"`
	MaskedAccount string `json:"masked_account_number"`
	AmountKobo    int64  `json:"amount_kobo"`
	State         string `json:"state"`
	Error         string `json:"error,omitempty"`
}

// RunView is the split of one credit.
type RunView struct {
	ID         string       `json:"id"`
	CreditID   string       `json:"credit_id"`
	CreditKobo int64        `json:"credit_amount_kobo"`
	Sender     string       `json:"sender_name"`
	State      string       `json:"state"` // completed or partial
	SentKobo   int64        `json:"sent_kobo"`
	HeldKobo   int64        `json:"held_kobo"`
	KeptKobo   int64        `json:"kept_kobo"`
	Legs       []RunLegView `json:"legs"`
	CreatedAt  time.Time    `json:"created_at"`
}

func (w *World) creditByID(id string) *Credit {
	for _, c := range w.credits {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// runTotals returns sent, held and kept for a run. sent + held + kept always equals the credit.
func runTotals(r *Run) (sent, held, kept int64) {
	for _, l := range r.Legs {
		switch l.State {
		case "sent":
			sent += l.Amount
		default:
			held += l.Amount // a share that has not left the receiving account yet
		}
	}
	return sent, held, r.Amount - sent - held
}

func (w *World) runView(r *Run) RunView {
	c := w.creditByID(r.CreditID)
	v := RunView{ID: r.ID, CreditID: r.CreditID, CreditKobo: r.Amount, Legs: []RunLegView{}, CreatedAt: r.CreatedAt, State: "completed"}
	if c != nil {
		v.Sender = c.Sender
	}
	v.SentKobo, v.HeldKobo, v.KeptKobo = runTotals(r)
	for _, l := range r.Legs {
		if l.State != "sent" {
			v.State = "partial"
		}
		v.Legs = append(v.Legs, RunLegView{ID: l.ID, Kind: l.Destination.Kind, Label: l.Destination.Label,
			BankName: BankName(l.Destination.BankCode), MaskedAccount: MaskNumber(l.Destination.Number),
			AmountKobo: l.Amount, State: l.State, Error: l.Error})
	}
	return v
}

// sendLeg moves one share out of the receiving account. Reusing the leg's reference makes a
// retry safe. Caller holds the lock.
func (w *World) sendLeg(r *Run, l *RunLeg, recv *Account) {
	var to *Account
	if l.Destination.Kind == "own" {
		to = w.accounts[l.Destination.AccountID]
		if to == nil || !w.usable(to) {
			l.State, l.Error = "failed", "destination_unavailable"
			return
		}
	}
	if err := withRetry(func() error { return w.transfer(l.Ref, recv, to, l.Amount, 0) }); err != nil {
		l.State, l.Error = "failed", failureCode(err)
		return
	}
	if l.State != "sent" {
		now := w.now()
		w.addTx(recv, "debit", l.Amount, l.Destination.Label, "split_receive", true, now)
		if to != nil {
			w.addTx(to, "credit", l.Amount, "Split-Receive", "split_receive", true, now)
		}
	}
	l.State, l.Error = "sent", ""
}

// scanLocked splits every new confirmed credit once. Caller holds the lock. It returns the
// runs it created.
func (w *World) scanLocked(only string) []*Run {
	var made []*Run
	cs := append([]*Credit(nil), w.credits...)
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].At.Before(cs[j].At) })
	for _, c := range cs {
		if only != "" && c.UserID != only {
			continue
		}
		rules := w.rules[c.UserID]
		if rules == nil || !c.Verified || c.AccountID != rules.ReceivingAccountID || c.At.Before(rules.ConsentAt) {
			continue
		}
		if internalRef(c.Reference) && !strings.HasPrefix(c.Reference, "p2p_") {
			continue // Apapo's own movements (pay_, rev_, sr_) are never split
		}
		if _, done := w.runByCred[c.ID]; done {
			continue
		}
		recv := w.accounts[rules.ReceivingAccountID]
		if recv == nil || !w.usable(recv) {
			continue
		}
		run := &Run{ID: w.nextID("run"), UserID: c.UserID, CreditID: c.ID, Amount: c.Amount, CreatedAt: w.now()}
		w.runByCred[c.ID] = run.ID // claimed before any money moves: one run per credit, never twice
		remaining := c.Amount
		for i, d := range rules.Destinations {
			amt := d.AmountKobo
			if d.PercentBP > 0 {
				amt = c.Amount * d.PercentBP / 10000
			}
			if amt > remaining {
				amt = remaining
			}
			if amt <= 0 {
				continue
			}
			remaining -= amt
			run.Legs = append(run.Legs, &RunLeg{ID: "leg_" + strconv.Itoa(i+1), Destination: d, Amount: amt, State: "pending", Ref: "sr_" + run.ID + "_" + strconv.Itoa(i+1)})
		}
		w.runs[run.ID] = run
		w.runOrder = append(w.runOrder, run.ID)
		for _, l := range run.Legs {
			w.sendLeg(run, l, recv)
		}
		made = append(made, run)
	}
	return made
}

// Scan splits new confirmed credits for one user and returns the runs created.
func (w *World) Scan(uid string) []RunView {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := []RunView{}
	for _, r := range w.scanLocked(uid) {
		out = append(out, w.runView(r))
	}
	return out
}

// ScanAll is the timer pass over every user.
func (w *World) ScanAll() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.scanLocked(""))
}

// Runs lists the user's runs, newest first.
func (w *World) Runs(uid string) []RunView {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := []RunView{}
	for i := len(w.runOrder) - 1; i >= 0; i-- {
		if r := w.runs[w.runOrder[i]]; r.UserID == uid {
			out = append(out, w.runView(r))
		}
	}
	return out
}

// RetryRun retries the failed shares of a run. Shares are independent: there is no rollback.
func (w *World) RetryRun(uid, id string) (*RunView, *Error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.runs[id]
	if r == nil || r.UserID != uid {
		return nil, errNotFound()
	}
	rules := w.rules[uid]
	var recv *Account
	if rules != nil {
		recv = w.accounts[rules.ReceivingAccountID]
	}
	if c := w.creditByID(r.CreditID); c != nil {
		recv = w.accounts[c.AccountID]
	}
	retried := 0
	for _, l := range r.Legs {
		if l.State == "failed" {
			retried++
			if recv != nil {
				w.sendLeg(r, l, recv)
			}
		}
	}
	if retried == 0 {
		return nil, Errf(http.StatusConflict, "nothing_to_retry", "Every share of this run has already been sent.", nil)
	}
	v := w.runView(r)
	return &v, nil
}

// ReconEntry is one credit and where each naira went.
type ReconEntry struct {
	CreditID   string    `json:"credit_id"`
	AmountKobo int64     `json:"amount_kobo"`
	Sender     string    `json:"sender_name"`
	Verified   bool      `json:"verified"`
	Status     string    `json:"status"` // split, partial, not_split, waiting_for_bank
	RunID      string    `json:"run_id,omitempty"`
	SentKobo   int64     `json:"sent_kobo"`
	HeldKobo   int64     `json:"held_kobo"`
	KeptKobo   int64     `json:"kept_kobo"`
	Balanced   bool      `json:"balanced"`
	PostedAt   time.Time `json:"posted_at"`
}

// ReconTotals adds up every entry.
type ReconTotals struct {
	CreditKobo int64 `json:"credit_kobo"`
	SentKobo   int64 `json:"sent_kobo"`
	KeptKobo   int64 `json:"kept_kobo"`
	HeldKobo   int64 `json:"held_kobo"`
}

// Reconciliation lists every credit and where its money went. The invariant is
// sent + held + kept = credit.
func (w *World) Reconciliation(uid string) (entries []ReconEntry, totals ReconTotals) {
	w.mu.Lock()
	defer w.mu.Unlock()
	entries = []ReconEntry{}
	for i := len(w.credits) - 1; i >= 0; i-- {
		c := w.credits[i]
		if c.UserID != uid || (internalRef(c.Reference) && !strings.HasPrefix(c.Reference, "p2p_")) {
			continue
		}
		en := ReconEntry{CreditID: c.ID, AmountKobo: c.Amount, Sender: c.Sender, Verified: c.Verified, PostedAt: c.At}
		if rid, ok := w.runByCred[c.ID]; ok {
			r := w.runs[rid]
			en.RunID = r.ID
			en.SentKobo, en.HeldKobo, en.KeptKobo = runTotals(r)
			en.Status = "split"
			if en.HeldKobo > 0 {
				en.Status = "partial"
			}
		} else {
			en.KeptKobo = c.Amount
			en.Status = "not_split"
			if !c.Verified {
				en.Status = "waiting_for_bank"
			}
		}
		en.Balanced = en.SentKobo+en.HeldKobo+en.KeptKobo == en.AmountKobo
		totals.CreditKobo += en.AmountKobo
		totals.SentKobo += en.SentKobo
		totals.HeldKobo += en.HeldKobo
		totals.KeptKobo += en.KeptKobo
		entries = append(entries, en)
	}
	return entries, totals
}
