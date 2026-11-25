package app

import (
	"strconv"
	"time"
)

var defaultBalanceNaira = map[string]int64{"apex": 70000, "delta": 60000, "savanna": 45000, "harbor": 20000}

const linkTTL = 90 * 24 * time.Hour

// linkBank creates (or returns) the active link and account for a bank. Caller holds the lock.
// Linking twice is idempotent: the existing account comes back.
func (w *World) linkBank(u *User, bank string, scopes []string, now time.Time) *Account {
	for _, l := range w.links {
		if l.UserID == u.ID && l.BankCode == bank && !l.Revoked && now.Before(l.ExpiresAt) {
			for _, id := range w.accOrder {
				if a := w.accounts[id]; a.LinkID == l.ID {
					return a
				}
			}
		}
	}
	l := &Link{ID: w.nextID("lnk"), UserID: u.ID, BankCode: bank, Scopes: scopes, LinkedAt: now, ExpiresAt: now.Add(linkTTL)}
	w.links[l.ID] = l
	// A revoked link's account comes back with its balance, so money never vanishes.
	for _, id := range w.accOrder {
		if a := w.accounts[id]; a.UserID == u.ID && a.BankCode == bank {
			a.LinkID, a.Active = l.ID, true
			return a
		}
	}
	a := &Account{ID: w.nextID("acc"), UserID: u.ID, LinkID: l.ID, BankCode: bank, Active: true,
		Number: deriveNumber(u.ID, bank), Balance: defaultBalanceNaira[bank] * 100}
	w.accounts[a.ID] = a
	w.accOrder = append(w.accOrder, a.ID)
	return a
}

// deriveNumber makes a stable 10 digit account number.
func deriveNumber(userID, bank string) string {
	h := sha("acct", userID, bank)
	n := ""
	for i := 0; len(n) < 9; i++ {
		v, _ := strconv.ParseUint(h[i*2:i*2+2], 16, 8)
		n += strconv.Itoa(int(v) % 10)
	}
	return "3" + n
}

func (w *World) acctHash(bank, number string) string {
	return sha(string(w.repSalt), "acct", bank, number)
}
