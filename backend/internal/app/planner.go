package app

import (
	"net/http"
	"sort"
)

const (
	legFee      = 1000 // N10 per leg for the demo
	defaultLegs = 4
	maxAmount   = 100_000_000_00
)

// plan picks the fewest accounts that cover amount plus fees, keeping minBalance in each.
// The largest spendable account is the collector and comes first. Caller holds the lock.
func (w *World) plan(uid string, amount, minBalance int64, maxLegs int) ([]*Leg, *Error) {
	if amount < 100 || amount > maxAmount {
		return nil, Errf(http.StatusBadRequest, "invalid_amount", "Amount must be at least N1 and a whole number of kobo.", nil)
	}
	if minBalance < 0 {
		return nil, Errf(http.StatusBadRequest, "invalid_min_balance", "min_balance_kobo cannot be negative.", nil)
	}
	if maxLegs <= 0 {
		maxLegs = defaultLegs
	}
	type cand struct {
		a     *Account
		spend int64
	}
	var cands []cand
	var spendable int64
	for _, a := range w.userAccounts(uid) {
		if w.bankUnavailable(a.BankCode) {
			continue // a bank that is down takes no part
		}
		if s := a.Balance - minBalance - legFee; s > 0 {
			cands = append(cands, cand{a, s})
			spendable += s
		}
	}
	if spendable < amount {
		return nil, Errf(http.StatusUnprocessableEntity, "insufficient_funds", "Not enough spendable money across your accounts.",
			map[string]any{"spendable_kobo": spendable, "shortfall_kobo": amount - spendable})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].spend > cands[j].spend })
	var legs []*Leg
	rem := amount
	for _, c := range cands {
		if rem == 0 {
			break
		}
		take := c.spend
		if take > rem {
			take = rem
		}
		legs = append(legs, &Leg{AccountID: c.a.ID, BankCode: c.a.BankCode, Amount: take, Fee: legFee, State: "pending"})
		rem -= take
	}
	if len(legs) > maxLegs {
		return nil, Errf(http.StatusUnprocessableEntity, "too_many_legs", "This payment needs more accounts than allowed.",
			map[string]any{"legs_needed": len(legs), "max_legs": maxLegs})
	}
	legs[0].IsCollector = true
	return legs, nil
}
