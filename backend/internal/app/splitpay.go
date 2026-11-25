package app

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RecipientIn is the recipient in a preview or execute body. Only the fields below are
// read; the frontend's display fields (recipient_type, masked_display_name) and anything
// else are accepted and ignored.
type RecipientIn struct {
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	ApapoPhone    string `json:"apapo_phone"`
	RecentID      string `json:"recent_id"`
}

// PreviewIn is the body of preview and execute (execute adds PIN and risk acknowledgement).
type PreviewIn struct {
	AmountKobo     int64       `json:"amount_kobo"`
	Recipient      RecipientIn `json:"recipient"`
	Strategy       string      `json:"strategy"`
	MinBalanceKobo int64       `json:"min_balance_kobo"`
	MaxLegs        int         `json:"max_legs"`
}

// ExecuteIn adds the PIN and the Safe Send acknowledgement.
type ExecuteIn struct {
	PreviewIn
	PIN             string `json:"pin"`
	AcknowledgeRisk bool   `json:"acknowledge_risk"`
}

// LegView is one leg as the frontend sees it.
type LegView struct {
	AccountID     string `json:"account_id"`
	BankCode      string `json:"bank_code"`
	BankName      string `json:"bank_name"`
	MaskedAccount string `json:"masked_account_number"`
	AmountKobo    int64  `json:"amount_kobo"`
	FeeKobo       int64  `json:"fee_kobo"`
	State         string `json:"state"`
	IsCollector   bool   `json:"is_collector"`
}

// RecipientView never carries a resolved Apapo account.
type RecipientView struct {
	Type          string `json:"recipient_type"`
	MaskedName    string `json:"masked_display_name,omitempty"`
	BankCode      string `json:"bank_code,omitempty"`
	BankName      string `json:"bank_name,omitempty"`
	MaskedAccount string `json:"masked_account_number,omitempty"`
	AccountName   string `json:"account_name,omitempty"`
}

// PreviewOut is the response of preview.
type PreviewOut struct {
	Legs             []LegView     `json:"legs"`
	TotalAmountKobo  int64         `json:"total_amount_kobo"`
	TotalFeeKobo     int64         `json:"total_fee_kobo"`
	TotalDebitKobo   int64         `json:"total_debit_kobo"`
	CollectorAccount string        `json:"collector_account_id"`
	Recipient        RecipientView `json:"recipient"`
	Risk             Risk          `json:"risk"`
	Flags            []string      `json:"flags"`
	DuplicateWarning bool          `json:"duplicate_warning"`
	ShieldWarning    *ShieldAlert  `json:"shield_warning"`
}

// PaymentView is a payment as the frontend sees it.
type PaymentView struct {
	ID               string        `json:"id"`
	State            string        `json:"state"`
	TotalAmountKobo  int64         `json:"total_amount_kobo"`
	TotalFeeKobo     int64         `json:"total_fee_kobo"`
	Legs             []LegView     `json:"legs"`
	CollectorAccount string        `json:"collector_account_id"`
	Recipient        RecipientView `json:"recipient"`
	FailureReason    string        `json:"failure_reason,omitempty"`
	RevAttempts      int           `json:"reversal_attempt_count"`
	NextRevRetryAt   *time.Time    `json:"next_reversal_retry_at"`
	LastRevAttemptAt *time.Time    `json:"last_reversal_attempt_at"`
	HumanReview      bool          `json:"human_review"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

func (r Recipient) view() RecipientView {
	if r.Type == "apapo" {
		return RecipientView{Type: "apapo", MaskedName: r.Masked}
	}
	return RecipientView{Type: "outside", BankCode: r.BankCode, BankName: BankName(r.BankCode),
		MaskedAccount: MaskNumber(r.Number), AccountName: r.AccountName}
}

func (w *World) legViews(legs []*Leg) []LegView {
	out := make([]LegView, 0, len(legs))
	for _, l := range legs {
		v := LegView{AccountID: l.AccountID, BankCode: l.BankCode, BankName: BankName(l.BankCode), AmountKobo: l.Amount,
			FeeKobo: l.Fee, State: l.State, IsCollector: l.IsCollector}
		if a := w.accounts[l.AccountID]; a != nil {
			v.MaskedAccount = MaskNumber(a.Number)
		}
		out = append(out, v)
	}
	return out
}

func (w *World) paymentView(p *Payment) PaymentView {
	v := PaymentView{ID: p.ID, State: p.State, TotalAmountKobo: p.Amount, TotalFeeKobo: p.Fee, Legs: w.legViews(p.Legs),
		CollectorAccount: p.CollectorID, Recipient: p.Recipient.view(), FailureReason: p.FailureReason,
		RevAttempts: p.RevAttempts, HumanReview: p.HumanReview, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
	if p.State == "needs_attention" {
		t := p.NextRevAt
		v.NextRevRetryAt = &t
	}
	if !p.LastRevAt.IsZero() {
		t := p.LastRevAt
		v.LastRevAttemptAt = &t
	}
	return v
}

// resolveRecipient turns the request recipient into a server side Recipient.
func (w *World) resolveRecipient(uid string, in RecipientIn) (Recipient, *Error) {
	if in.RecentID != "" {
		p := w.payments[in.RecentID]
		if p == nil || p.UserID != uid {
			return Recipient{}, errNotFound()
		}
		if p.Recipient.Type == "apapo" {
			u := w.users[p.Recipient.UserID]
			if u == nil || w.receiving(u) == nil {
				return Recipient{}, errNotFound()
			}
			r := p.Recipient
			r.AccountID = w.receiving(u).ID
			return r, nil
		}
		return p.Recipient, nil
	}
	if in.ApapoPhone != "" {
		phone, ok := NormalizePhone(in.ApapoPhone)
		if !ok {
			return Recipient{}, Errf(http.StatusBadRequest, "invalid_phone", "Enter a valid Nigerian phone number.", nil)
		}
		if w.users[uid].Phone == phone {
			return Recipient{}, Errf(http.StatusBadRequest, "self_send", "You cannot send money to yourself.", nil)
		}
		u := w.users[w.byPhone[phone]]
		if u == nil || !onboarding(u).Complete || w.receiving(u) == nil {
			return Recipient{}, errNotFound()
		}
		return Recipient{Type: "apapo", UserID: u.ID, AccountID: w.receiving(u).ID, Masked: MaskName(u.DisplayName), Key: "apapo:" + u.ID}, nil
	}
	name, e := w.resolveOutside(in.BankCode, in.AccountNumber)
	if e != nil {
		return Recipient{}, e
	}
	return Recipient{Type: "outside", BankCode: in.BankCode, Number: in.AccountNumber, AccountName: name,
		Key: "outside:" + in.BankCode + ":" + in.AccountNumber}, nil
}

func validStrategy(s string) bool { return s == "" || s == "fewest" }

// Preview plans a payment without moving money.
func (w *World) Preview(uid string, in PreviewIn) (*PreviewOut, *Error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	out, _, e := w.previewLocked(uid, in)
	return out, e
}

func (w *World) previewLocked(uid string, in PreviewIn) (*PreviewOut, []*Leg, *Error) {
	if _, e := w.onboarded(uid); e != nil {
		return nil, nil, e
	}
	if !validStrategy(in.Strategy) {
		return nil, nil, Errf(http.StatusBadRequest, "invalid_strategy", "strategy must be fewest.", nil)
	}
	rec, e := w.resolveRecipient(uid, in.Recipient)
	if e != nil {
		return nil, nil, e
	}
	legs, e := w.plan(uid, in.AmountKobo, in.MinBalanceKobo, in.MaxLegs)
	if e != nil {
		return nil, nil, e
	}
	risk, flags, dup := w.assess(uid, rec, in.Recipient.AccountName, in.AmountKobo)
	var fee int64
	for _, l := range legs {
		fee += l.Fee
	}
	if flags == nil {
		flags = []string{}
	}
	return &PreviewOut{Legs: w.legViews(legs), TotalAmountKobo: in.AmountKobo, TotalFeeKobo: fee, TotalDebitKobo: in.AmountKobo + fee,
		CollectorAccount: legs[0].AccountID, Recipient: rec.view(), Risk: risk, Flags: flags, DuplicateWarning: dup,
		ShieldWarning: w.shieldAlert(uid, in.AmountKobo)}, legs, nil
}

func bodyHash(in PreviewIn, rec Recipient) string {
	strat := in.Strategy
	if strat == "" {
		strat = "fewest"
	}
	return sha("exec", strconv.FormatInt(in.AmountKobo, 10), rec.Key, strat, strconv.FormatInt(in.MinBalanceKobo, 10), strconv.Itoa(in.MaxLegs))
}

// Execute checks the PIN, replans on the server and starts the payment. The same
// Idempotency-Key with the same body returns the same payment (replayed true).
func (w *World) Execute(uid string, in ExecuteIn, key string) (*PaymentView, bool, *Error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 128 {
		return nil, false, Errf(http.StatusBadRequest, "idempotency_key_required", "Send an Idempotency-Key header (1 to 128 characters).", nil)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	u, e := w.onboarded(uid)
	if e != nil {
		return nil, false, e
	}
	rec, e := w.resolveRecipient(uid, in.Recipient)
	if e != nil {
		return nil, false, e
	}
	h := bodyHash(in.PreviewIn, rec)
	if prev, ok := w.idem[uid+"|"+key]; ok {
		if prev.BodyHash != h {
			return nil, false, Errf(http.StatusConflict, "idempotency_conflict", "This Idempotency-Key was already used for a different payment.", nil)
		}
		v := w.paymentView(w.payments[prev.PaymentID])
		return &v, true, nil
	}
	if e := w.checkPIN(u, in.PIN); e != nil {
		return nil, false, e
	}
	prev, legs, e := w.previewLocked(uid, in.PreviewIn)
	if e != nil {
		return nil, false, e
	}
	if prev.Risk.Tier == "high" && !in.AcknowledgeRisk {
		return nil, false, Errf(http.StatusConflict, "risk_acknowledgement_required",
			"This recipient is high risk. Send acknowledge_risk:true only if you are sure.", map[string]any{"reasons": prev.Risk.Reasons})
	}
	now := w.now()
	p := &Payment{ID: w.nextID("pay"), Seq: w.seq, UserID: uid, State: "planned", Amount: in.AmountKobo, Legs: legs,
		CollectorID: legs[0].AccountID, Recipient: rec, IdemKey: key, BodyHash: h, CreatedAt: now, UpdatedAt: now}
	for i, l := range legs {
		p.Fee += l.Fee
		l.Ref = "pay_" + p.ID + "_leg" + strconv.Itoa(i)
		l.RevRef = "rev_" + p.ID + "_leg" + strconv.Itoa(i)
	}
	w.payments[p.ID] = p
	w.payOrder = append(w.payOrder, p.ID)
	w.idem[uid+"|"+key] = idemRec{BodyHash: h, PaymentID: p.ID}
	if w.Async {
		go func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			w.runPayment(p)
		}()
	} else {
		w.runPayment(p)
	}
	v := w.paymentView(p)
	return &v, false, nil
}

// withRetry repeats an operation only on timeouts, reusing the same reference so money
// can never move twice.
func withRetry(op func() error) error {
	var err error
	for i := 0; i < 3; i++ {
		if err = op(); err != ErrTimeout {
			return err
		}
	}
	return err
}

func failureCode(err error) string {
	switch err {
	case ErrInsufficient:
		return "insufficient_funds"
	case ErrUnavailable:
		return "bank_unavailable"
	case ErrTimeout:
		return "timeout"
	}
	return "bank_error"
}

func (w *World) touch(p *Payment) { p.UpdatedAt = w.now() }

// runPayment collects every leg into the collector, then sends. Caller holds the lock.
func (w *World) runPayment(p *Payment) {
	coll := w.accounts[p.CollectorID]
	p.State = "collecting"
	w.touch(p)
	for _, l := range p.Legs {
		if l.IsCollector {
			l.State = "collected"
			continue
		}
		src := w.accounts[l.AccountID]
		if err := withRetry(func() error { return w.transfer(l.Ref, src, coll, l.Amount, l.Fee) }); err != nil {
			l.State = "failed"
			p.FailureReason = failureCode(err)
			w.rollback(p)
			return
		}
		l.State = "collected"
	}
	p.State = "sending"
	w.touch(p)
	var to *Account
	if p.Recipient.Type == "apapo" {
		to = w.accounts[p.Recipient.AccountID]
	}
	cl := p.Legs[0]
	payRef := "pay_" + p.ID + "_out"
	if err := withRetry(func() error { return w.transfer(payRef, coll, to, p.Amount, cl.Fee) }); err != nil {
		p.FailureReason = failureCode(err)
		w.rollback(p)
		return
	}
	p.PayoutDone = true
	who := p.Recipient.AccountName
	if p.Recipient.Type == "apapo" {
		who = p.Recipient.Masked
	}
	now := w.now()
	for _, l := range p.Legs {
		w.addTx(w.accounts[l.AccountID], "debit", l.Amount+l.Fee, who, "transfer", true, now)
	}
	if to != nil {
		sender := w.users[p.UserID]
		w.credits = append(w.credits, &Credit{ID: w.nextID("crd"), UserID: to.UserID, AccountID: to.ID, Amount: p.Amount,
			Sender: MaskName(sender.DisplayName), Reference: "p2p_" + p.ID, Verified: true, At: now})
		w.addTx(to, "credit", p.Amount, MaskName(sender.DisplayName), "transfer", true, now)
	}
	p.State = "completed"
	w.touch(p)
}

// rollback reverses collected legs in reverse order. If a reversal fails, the payment becomes
// needs_attention: the money stays safe in the collector account and the worker keeps
// retrying with the same references. Caller holds the lock.
func (w *World) rollback(p *Payment) {
	w.touch(p)
	w.reversePass(p)
}

// reversePass makes one attempt at every leg still collected. It returns true when done.
func (w *World) reversePass(p *Payment) bool {
	coll := w.accounts[p.CollectorID]
	now := w.now()
	pending := false
	for i := len(p.Legs) - 1; i >= 0; i-- {
		l := p.Legs[i]
		if l.State != "collected" || l.IsCollector {
			continue
		}
		src := w.accounts[l.AccountID]
		if err := w.reverse(l.RevRef, src, coll, l.Amount, l.Fee); err != nil {
			pending = true
			p.AttemptLog = append(p.AttemptLog, now.Format(time.RFC3339)+" leg "+strconv.Itoa(i)+" reversal failed: "+err.Error())
			continue
		}
		l.State = "reversed"
		w.addTx(src, "credit", l.Amount+l.Fee, "Refund: payment rolled back", "refund", true, now)
	}
	if pending {
		p.RevAttempts++
		p.LastRevAt = now
		delay := time.Second << min(p.RevAttempts-1, 8) // 1s, 2s, 4s ... capped near 4 minutes
		p.NextRevAt = now.Add(delay)
		if p.RevAttempts >= 10 {
			p.HumanReview = true
		}
		p.State = "needs_attention"
		log.Printf("reversal attempt payment=%s attempt=%d result=failed next_in=%s human_review=%t", p.ID, p.RevAttempts, delay, p.HumanReview)
		w.touch(p)
		return false
	}
	if p.State == "needs_attention" {
		log.Printf("reversal attempt payment=%s attempt=%d result=ok", p.ID, p.RevAttempts+1)
	}
	for _, l := range p.Legs {
		if l.State == "pending" || (l.IsCollector && l.State == "collected") {
			l.State = "reversed" // nothing moved for this leg, so nothing is left to undo
		}
	}
	p.State = "rolled_back"
	p.NextRevAt = time.Time{}
	w.touch(p)
	return true
}

// RetryReversals runs one worker pass over payments that need attention.
func (w *World) RetryReversals() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	for _, id := range w.payOrder {
		p := w.payments[id]
		if p.State == "needs_attention" && !now.Before(p.NextRevAt) {
			w.reversePass(p)
		}
	}
}

// Rescan runs on startup: unfinished payments are picked up again and retried at once.
func (w *World) Rescan() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, id := range w.payOrder {
		p := w.payments[id]
		switch p.State {
		case "needs_attention":
			p.NextRevAt = w.now()
			n++
		case "planned", "collecting", "sending":
			w.runOrRollback(p)
			n++
		}
	}
	return n
}

// runOrRollback finishes a payment that was cut off. It rolls back: the safe choice.
func (w *World) runOrRollback(p *Payment) {
	if p.PayoutDone {
		p.State = "completed"
		return
	}
	p.FailureReason = "interrupted"
	w.rollback(p)
}

// PaymentStatus returns one payment to its owner.
func (w *World) PaymentStatus(uid, id string) (*PaymentView, *Error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.payments[id]
	if p == nil || p.UserID != uid {
		return nil, errNotFound()
	}
	v := w.paymentView(p)
	return &v, nil
}

// Payments lists the owner's payments, newest first, with an id cursor.
func (w *World) Payments(uid string, limit int, before string) ([]PaymentView, string, *Error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var cursor int64 = 1 << 62
	if before != "" {
		b := w.payments[before]
		if b == nil || b.UserID != uid {
			return nil, "", Errf(http.StatusBadRequest, "invalid_cursor", "The before cursor is not valid.", nil)
		}
		cursor = b.Seq
	}
	out := []PaymentView{}
	next := ""
	for i := len(w.payOrder) - 1; i >= 0; i-- {
		p := w.payments[w.payOrder[i]]
		if p.UserID != uid || p.Seq >= cursor {
			continue
		}
		if len(out) == limit {
			next = out[len(out)-1].ID
			break
		}
		out = append(out, w.paymentView(p))
	}
	return out, next, nil
}

// RecentRecipient is a masked past recipient.
type RecentRecipient struct {
	ID         string        `json:"id"`
	Recipient  RecipientView `json:"recipient"`
	LastPaidAt time.Time     `json:"last_paid_at"`
	LastAmount int64         `json:"last_amount_kobo"`
}

// RecentRecipients lists people the user paid, newest first, masked. The id can be sent as
// recipient.recent_id to pay them again.
func (w *World) RecentRecipients(uid string) []RecentRecipient {
	w.mu.Lock()
	defer w.mu.Unlock()
	seen := map[string]bool{}
	out := []RecentRecipient{}
	for i := len(w.payOrder) - 1; i >= 0 && len(out) < 8; i-- {
		p := w.payments[w.payOrder[i]]
		if p.UserID != uid || p.State != "completed" || seen[p.Recipient.Key] {
			continue
		}
		seen[p.Recipient.Key] = true
		out = append(out, RecentRecipient{ID: p.ID, Recipient: p.Recipient.view(), LastPaidAt: p.UpdatedAt, LastAmount: p.Amount})
	}
	return out
}
