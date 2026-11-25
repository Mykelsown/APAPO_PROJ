package app

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	reportersForHigh = 3
	duplicateWindow  = 10 * time.Minute
	unusualAmount    = 10000000 // N100,000
	shieldTolerance  = 10       // percent
)

// Risk is the Safe Send verdict for a recipient.
type Risk struct {
	Tier    string   `json:"tier"` // low, unknown or high
	Reasons []string `json:"reasons"`
}

// ShieldAlert warns that a credit the bank has not confirmed may be about to be sent on.
type ShieldAlert struct {
	CreditID   string    `json:"credit_id"`
	AmountKobo int64     `json:"amount_kobo"`
	Sender     string    `json:"sender_name"`
	PostedAt   time.Time `json:"posted_at"`
	Message    string    `json:"message"`
}

const shieldMessage = "This money has not been confirmed by the bank yet. Wait for confirmation before sending it on."

var directoryNames = []string{"Chinedu Eze", "Funke Adebayo", "Musa Ibrahim", "Kelechi Nwosu", "Tolu Ogunleye", "Zainab Bello", "Obinna Okoro", "Ronke Lawal"}

// resolveOutside validates an outside account and returns its simulated account name.
func (w *World) resolveOutside(bank, number string) (string, *Error) {
	known, supported := bankSupported(bank)
	switch {
	case bank == "":
		return "", Errf(http.StatusBadRequest, "invalid_recipient", "Choose the recipient's bank.", nil)
	case !known:
		return "", Errf(http.StatusBadRequest, "unknown_bank", "That bank is not in the catalogue.", nil)
	case !supported:
		return "", Errf(http.StatusBadRequest, "bank_not_supported", "That bank is coming soon.", nil)
	}
	if len(number) != 10 || !allDigits(number) {
		return "", Errf(http.StatusBadRequest, "invalid_account_number", "An account number has exactly 10 digits.", nil)
	}
	switch {
	case bank == FlaggedBank && number == FlaggedNumber:
		return "Quick Returns Investments", nil
	case number == GoodPayee:
		return "Ada Okafor", nil
	}
	sum := 0
	for _, r := range number {
		sum += int(r - '0')
	}
	return directoryNames[sum%len(directoryNames)], nil
}

func (w *World) reporterCount(bank, number string) int {
	return len(w.reports[w.acctHash(bank, number)])
}

func normName(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// assess builds the Safe Send verdict. Caller holds the lock.
func (w *World) assess(uid string, rec Recipient, enteredName string, amount int64) (Risk, []string, bool) {
	var flags, reasons []string
	high := false
	if rec.Type == "outside" && w.reporterCount(rec.BankCode, rec.Number) >= reportersForHigh {
		high = true
		flags = append(flags, "reported_account")
		reasons = append(reasons, "Several different people have reported this account.")
	}
	if rec.Type == "outside" && enteredName != "" && normName(enteredName) != normName(rec.AccountName) {
		flags = append(flags, "name_mismatch")
		reasons = append(reasons, "The name you entered does not match the name on the account.")
	}
	now := w.now()
	var biggest int64
	first, dup := true, false
	for _, id := range w.payOrder {
		p := w.payments[id]
		if p.UserID != uid || p.State == "rolled_back" {
			continue
		}
		if p.Recipient.Key == rec.Key {
			if p.State == "completed" {
				first = false
			}
			if p.Amount == amount && now.Sub(p.CreatedAt) < duplicateWindow {
				dup = true
			}
		}
		if p.State == "completed" && p.Amount > biggest {
			biggest = p.Amount
		}
	}
	if first {
		flags = append(flags, "first_payment")
		reasons = append(reasons, "You have not paid this recipient before.")
	}
	if amount >= unusualAmount || (biggest > 0 && amount > 3*biggest) {
		flags = append(flags, "unusual_amount")
		reasons = append(reasons, "This amount is larger than you usually send.")
	}
	if dup {
		flags = append(flags, "possible_duplicate")
		reasons = append(reasons, "You sent the same amount to this recipient a few minutes ago.")
	}
	tier := "unknown"
	switch {
	case high:
		tier = "high"
	case len(flags) == 0:
		tier = "low"
	case len(flags) == 1 && flags[0] == "first_payment" && (rec.Type == "apapo" || w.goodPayees[rec.Number]):
		tier = "low" // a known Apapo user or a payee with a clean record
		if rec.Type == "apapo" {
			reasons = []string{"This is an Apapo user with a verified profile."}
		} else {
			reasons = []string{"This payee has a clean record."}
		}
	}
	if len(reasons) == 0 {
		reasons = []string{"Nothing unusual found."}
	}
	return Risk{Tier: tier, Reasons: reasons}, flags, dup
}

func internalRef(ref string) bool {
	for _, p := range []string{"pay_", "rev_", "sr_", "p2p_"} {
		if strings.HasPrefix(ref, p) {
			return true
		}
	}
	return false
}

// shieldAlert returns a warning when amount is within 10 percent of a recent unverified
// credit. Apapo's own movements and P2P credits are ignored. Caller holds the lock.
func (w *World) shieldAlert(uid string, amount int64) *ShieldAlert {
	if amount <= 0 {
		return nil
	}
	now := w.now()
	cs := make([]*Credit, 0)
	for _, c := range w.credits {
		cs = append(cs, c)
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].At.After(cs[j].At) })
	for _, c := range cs {
		if c.UserID != uid || c.Verified || w.dismissed[c.ID] || internalRef(c.Reference) || now.Sub(c.At) > 24*time.Hour {
			continue
		}
		diff := amount - c.Amount
		if diff < 0 {
			diff = -diff
		}
		if diff*100 <= c.Amount*shieldTolerance {
			return &ShieldAlert{CreditID: c.ID, AmountKobo: c.Amount, Sender: c.Sender, PostedAt: c.At, Message: shieldMessage}
		}
	}
	return nil
}
