package app

import (
	"net/http"
	"strings"
	"time"
)

const (
	reportsPerDay = 5
	maxEvidence   = 500
)

// RiskCheckOut is the response of POST /api/risk/check.
type RiskCheckOut struct {
	Recipient        RecipientView `json:"recipient"`
	Risk             Risk          `json:"risk"`
	Flags            []string      `json:"flags"`
	DuplicateWarning bool          `json:"duplicate_warning"`
}

// RiskCheck judges a recipient without planning a payment. amount_kobo may be 0.
func (w *World) RiskCheck(uid string, in PreviewIn) (*RiskCheckOut, *Error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, e := w.onboarded(uid); e != nil {
		return nil, e
	}
	if in.AmountKobo < 0 || in.AmountKobo > maxAmount {
		return nil, Errf(http.StatusBadRequest, "invalid_amount", "Amount cannot be negative.", nil)
	}
	rec, e := w.resolveRecipient(uid, in.Recipient)
	if e != nil {
		return nil, e
	}
	risk, flags, dup := w.assess(uid, rec, in.Recipient.AccountName, in.AmountKobo)
	if flags == nil {
		flags = []string{}
	}
	return &RiskCheckOut{Recipient: rec.view(), Risk: risk, Flags: flags, DuplicateWarning: dup}, nil
}

// ReportIn is the body of POST /api/reports.
type ReportIn struct {
	Recipient   RecipientIn `json:"recipient"`
	EvidenceRef string      `json:"evidence_ref"`
}

// Report records that a user reported an outside account. It stores salted hashes only:
// no account number and no user id are kept. Returns created=false for a repeat report.
func (w *World) Report(uid string, in ReportIn) (created bool, e *Error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, e := w.onboarded(uid); e != nil {
		return false, e
	}
	ev := strings.TrimSpace(in.EvidenceRef)
	if ev == "" || len(ev) > maxEvidence {
		return false, Errf(http.StatusBadRequest, "evidence_required", "Add a reference to your evidence (for example a screenshot id or transaction reference).", nil)
	}
	if _, e := w.resolveOutside(in.Recipient.BankCode, in.Recipient.AccountNumber); e != nil {
		return false, e
	}
	ah := w.acctHash(in.Recipient.BankCode, in.Recipient.AccountNumber)
	rh := sha(string(w.repSalt), "reporter", uid, ah)
	if _, dup := w.reports[ah][rh]; dup {
		return false, nil
	}
	now := w.now()
	recent := w.repLog[uid][:0]
	for _, t := range w.repLog[uid] {
		if now.Sub(t) < 24*time.Hour {
			recent = append(recent, t)
		}
	}
	w.repLog[uid] = recent
	if len(recent) >= reportsPerDay {
		wait := int((24*time.Hour - now.Sub(recent[0])).Seconds()) + 1
		return false, &Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "You can send 5 reports a day.", RetryAfter: wait}
	}
	if w.reports[ah] == nil {
		w.reports[ah] = map[string]time.Time{}
	}
	w.reports[ah][rh] = now
	w.repLog[uid] = append(w.repLog[uid], now)
	return true, nil
}

// ShieldCheck returns the Shield alert for an amount, or nil.
func (w *World) ShieldCheck(uid string, amount int64) (*ShieldAlert, *Error) {
	if amount <= 0 || amount > maxAmount {
		return nil, Errf(http.StatusBadRequest, "invalid_amount", "amount_kobo must be a positive number of kobo.", nil)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.shieldAlert(uid, amount), nil
}

// ShieldDismiss hides an alert for one credit. Repeating it is harmless.
func (w *World) ShieldDismiss(uid, creditID string) *Error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.credits {
		if c.ID == creditID && c.UserID == uid {
			w.dismissed[c.ID] = true
			return nil
		}
	}
	return errNotFound()
}

// DebugReports dumps the report store as text. Tests use it to prove no account numbers or
// user ids are kept in the clear.
func (w *World) DebugReports() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var b strings.Builder
	for ah, m := range w.reports {
		b.WriteString(ah)
		for rh := range m {
			b.WriteString(" " + rh)
		}
		b.WriteString("\n")
	}
	return b.String()
}
