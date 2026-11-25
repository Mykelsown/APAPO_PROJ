package app

import (
	"net/http"
	"sort"
	"time"
)

// BankView is one entry of the bank catalogue.
type BankView struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Supported bool   `json:"supported"`
}

// AccountView is a masked account with its balance.
type AccountView struct {
	ID            string `json:"id"`
	LinkID        string `json:"link_id"`
	BankCode      string `json:"bank_code"`
	BankName      string `json:"bank_name"`
	MaskedAccount string `json:"masked_account_number"`
	BalanceKobo   int64  `json:"balance_kobo"`
	Unavailable   bool   `json:"unavailable"`
	IsReceiving   bool   `json:"is_receiving"`
}

// LinkView is the result of linking a bank.
type LinkView struct {
	LinkID    string      `json:"link_id"`
	BankCode  string      `json:"bank_code"`
	BankName  string      `json:"bank_name"`
	LinkedAt  time.Time   `json:"linked_at"`
	ExpiresAt time.Time   `json:"expires_at"`
	Scopes    []string    `json:"scopes"`
	Account   AccountView `json:"account"`
}

// Consent is what the user agreed to when linking a bank.
type Consent struct {
	Accepted bool     `json:"accepted"`
	Scopes   []string `json:"scopes"`
}

// Banks lists the catalogue.
func Banks() []BankView {
	out := make([]BankView, 0, len(bankCatalogue))
	for _, b := range bankCatalogue {
		out = append(out, BankView{b.Code, b.Name, b.Supported})
	}
	return out
}

func (w *World) accountView(u *User, a *Account) AccountView {
	return AccountView{ID: a.ID, LinkID: a.LinkID, BankCode: a.BankCode, BankName: BankName(a.BankCode),
		MaskedAccount: MaskNumber(a.Number), BalanceKobo: a.Balance, Unavailable: w.bankUnavailable(a.BankCode),
		IsReceiving: u != nil && u.ReceivingAccountID == a.ID}
}

// usable says whether an account may take part in payments: active, link not revoked or expired.
func (w *World) usable(a *Account) bool {
	l := w.links[a.LinkID]
	return a.Active && l != nil && !l.Revoked && w.now().Before(l.ExpiresAt)
}

// userAccounts returns the user's usable accounts in link order. Caller holds the lock.
func (w *World) userAccounts(uid string) []*Account {
	var out []*Account
	for _, id := range w.accOrder {
		if a := w.accounts[id]; a.UserID == uid && w.usable(a) {
			out = append(out, a)
		}
	}
	return out
}

func (w *World) onboarded(uid string) (*User, *Error) {
	u := w.users[uid]
	if !onboarding(u).Complete {
		return nil, errOnboarding()
	}
	return u, nil
}

// LinkBank links a bank after consent. Linking twice returns the same link (200, not 201).
func (w *World) LinkBank(uid, bank string, c Consent) (LinkView, bool, *Error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	u, e := w.onboarded(uid)
	if e != nil {
		return LinkView{}, false, e
	}
	known, supported := bankSupported(bank)
	switch {
	case !known:
		return LinkView{}, false, Errf(http.StatusBadRequest, "unknown_bank", "That bank is not in the catalogue.", nil)
	case !supported:
		return LinkView{}, false, Errf(http.StatusBadRequest, "bank_not_supported", "That bank is coming soon.", nil)
	}
	need := map[string]bool{"accounts": false, "balances": false, "transactions": false}
	for _, s := range c.Scopes {
		if _, ok := need[s]; ok {
			need[s] = true
		}
	}
	if !c.Accepted || !need["accounts"] || !need["balances"] || !need["transactions"] {
		return LinkView{}, false, Errf(http.StatusBadRequest, "consent_required",
			"Consent to share accounts, balances and transactions is required to link a bank.", nil)
	}
	before := len(w.links)
	a := w.linkBank(u, bank, []string{"accounts", "balances", "transactions"}, w.now())
	if w.receiving(u) == nil {
		u.ReceivingAccountID = a.ID
	}
	l := w.links[a.LinkID]
	return LinkView{LinkID: l.ID, BankCode: bank, BankName: BankName(bank), LinkedAt: l.LinkedAt, ExpiresAt: l.ExpiresAt,
		Scopes: l.Scopes, Account: w.accountView(u, a)}, len(w.links) > before, nil
}

// Accounts lists the user's usable accounts.
func (w *World) Accounts(uid string) []AccountView {
	w.mu.Lock()
	defer w.mu.Unlock()
	u := w.users[uid]
	out := []AccountView{}
	for _, a := range w.userAccounts(uid) {
		out = append(out, w.accountView(u, a))
	}
	return out
}

// RevokeLink revokes a bank link. Revoked accounts never take part in payments.
func (w *World) RevokeLink(uid, linkID string) (map[string]any, *Error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	l := w.links[linkID]
	if l == nil || l.UserID != uid {
		return nil, errNotFound()
	}
	l.Revoked = true
	u := w.users[uid]
	for _, a := range w.accounts {
		if a.LinkID == l.ID {
			a.Active = false
			if u.ReceivingAccountID == a.ID {
				u.ReceivingAccountID = ""
				for _, o := range w.userAccounts(uid) { // fall back to another linked account
					u.ReceivingAccountID = o.ID
					break
				}
			}
		}
	}
	return map[string]any{"link_id": l.ID, "revoked": true}, nil
}

// BankCard is one bank on the dashboard.
type BankCard struct {
	LinkID      string        `json:"link_id"`
	BankCode    string        `json:"bank_code"`
	BankName    string        `json:"bank_name"`
	BalanceKobo int64         `json:"balance_kobo"`
	Unavailable bool          `json:"unavailable"`
	ExpiresAt   time.Time     `json:"expires_at"`
	Accounts    []AccountView `json:"accounts"`
}

// FeedItem is one line of the merged transaction feed.
type FeedItem struct {
	ID           string    `json:"id"`
	Direction    string    `json:"direction"`
	AmountKobo   int64     `json:"amount_kobo"`
	BankName     string    `json:"bank_name"`
	BankCode     string    `json:"bank_code"`
	Counterparty string    `json:"counterparty"`
	Category     string    `json:"category"`
	Verification string    `json:"verification"`
	PostedAt     time.Time `json:"posted_at"`
}

// Dashboard is the response of GET /api/dashboard.
type Dashboard struct {
	TotalBalanceKobo int64      `json:"total_balance_kobo"`
	Banks            []BankCard `json:"banks"`
	Transactions     []FeedItem `json:"transactions"`
}

// Dashboard builds the unified view. A bank that is down is shown as unavailable and is left
// out of the total.
func (w *World) Dashboard(uid string, limit int) Dashboard {
	w.mu.Lock()
	defer w.mu.Unlock()
	u := w.users[uid]
	d := Dashboard{Banks: []BankCard{}, Transactions: []FeedItem{}}
	cards := map[string]*BankCard{}
	own := map[string]*Account{}
	for _, a := range w.userAccounts(uid) {
		own[a.ID] = a
		l := w.links[a.LinkID]
		c := cards[l.ID]
		if c == nil {
			c = &BankCard{LinkID: l.ID, BankCode: a.BankCode, BankName: BankName(a.BankCode), ExpiresAt: l.ExpiresAt,
				Unavailable: w.bankUnavailable(a.BankCode), Accounts: []AccountView{}}
			cards[l.ID] = c
			d.Banks = append(d.Banks, BankCard{}) // placeholder keeps link order
			d.Banks[len(d.Banks)-1].LinkID = l.ID
		}
		c.BalanceKobo += a.Balance
		c.Accounts = append(c.Accounts, w.accountView(u, a))
	}
	for i := range d.Banks {
		c := cards[d.Banks[i].LinkID]
		d.Banks[i] = *c
		if !c.Unavailable {
			d.TotalBalanceKobo += c.BalanceKobo
		}
	}
	txs := make([]*Tx, 0, len(w.txs))
	for _, t := range w.txs {
		if _, ok := own[t.AccountID]; ok {
			txs = append(txs, t)
		}
	}
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].At.After(txs[j].At) })
	for _, t := range txs {
		if len(d.Transactions) >= limit {
			break
		}
		v := "verified"
		if !t.Verified {
			v = "unverified"
		}
		d.Transactions = append(d.Transactions, FeedItem{ID: t.ID, Direction: t.Direction, AmountKobo: t.Amount,
			BankName: BankName(own[t.AccountID].BankCode), BankCode: own[t.AccountID].BankCode, Counterparty: t.Counterparty,
			Category: t.Category, Verification: v, PostedAt: t.At})
	}
	return d
}
