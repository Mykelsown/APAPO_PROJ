package app

import (
	"errors"
)

// Bank failure modes the demo can switch on.
const (
	ModeNone         = "none"
	ModeInsufficient = "insufficient_funds"
	ModeUnavailable  = "bank_unavailable"
	ModeTimeout      = "timeout"
	ModeReversalDown = "reversal_down"
)

// Errors a simulated bank can return.
var (
	ErrInsufficient = errors.New("insufficient funds at the bank")
	ErrUnavailable  = errors.New("bank unavailable")
	ErrTimeout      = errors.New("bank timed out, outcome unknown")
	ErrReversalDown = errors.New("reversal could not be completed")
)

// Banks is the catalogue. The last two are "coming soon".
var bankCatalogue = []struct {
	Code      string
	Name      string
	Supported bool
}{
	{"apex", "Apex Bank", true},
	{"delta", "Delta Trust Bank", true},
	{"savanna", "Savanna Bank", true},
	{"harbor", "Harbor MFB", true},
	{"cedar", "Cedar Trust Bank", false},
	{"meridian", "Meridian MFB", false},
}

// BankName returns the display name for a code.
func BankName(code string) string {
	for _, b := range bankCatalogue {
		if b.Code == code {
			return b.Name
		}
	}
	return code
}

func bankSupported(code string) (known, supported bool) {
	for _, b := range bankCatalogue {
		if b.Code == code {
			return true, b.Supported
		}
	}
	return false, false
}

type bankSim struct {
	modes       map[string]string // "bank:<code>" or "acct:<id>" to mode
	reverseDown map[string]bool
	done        map[string]bool // references already applied, so a retry never moves money twice
}

func newBankSim() *bankSim {
	return &bankSim{modes: map[string]string{}, reverseDown: map[string]bool{}, done: map[string]bool{}}
}

func (s *bankSim) modeFor(a *Account) (key, mode string) {
	if m, ok := s.modes["acct:"+a.ID]; ok {
		return "acct:" + a.ID, m
	}
	if m, ok := s.modes["bank:"+a.BankCode]; ok {
		return "bank:" + a.BankCode, m
	}
	return "", ""
}

// BankUnavailable reports whether a bank is switched off.
func (w *World) bankUnavailable(code string) bool {
	return w.sim.modes["bank:"+code] == ModeUnavailable
}

// transfer moves amount from one account to another (to may be nil for a payout to an
// outside account) and charges fee to the source. The reference makes it idempotent:
// asking again with the same reference never moves money twice.
func (w *World) transfer(ref string, from, to *Account, amount, fee int64) error {
	s := w.sim
	if s.done[ref] {
		return nil
	}
	timeout := false
	for _, a := range []*Account{from, to} {
		if a == nil {
			continue
		}
		key, mode := s.modeFor(a)
		switch mode {
		case ModeUnavailable:
			return ErrUnavailable
		case ModeInsufficient:
			if a == from {
				return ErrInsufficient
			}
		case ModeTimeout:
			delete(s.modes, key) // a timeout fires once
			timeout = true
		}
	}
	if from.Balance < amount+fee {
		return ErrInsufficient
	}
	from.Balance -= amount + fee
	if to != nil {
		to.Balance += amount
	}
	s.done[ref] = true
	if timeout {
		return ErrTimeout // money moved, but the caller does not know it
	}
	return nil
}

// reverse takes amount back from the collector and returns amount+fee to the source.
// It fails while a reversal outage is switched on for either bank. Idempotent by reference.
func (w *World) reverse(ref string, src, collector *Account, amount, fee int64) error {
	s := w.sim
	if s.done[ref] {
		return nil
	}
	if s.reverseDown[src.BankCode] || s.reverseDown[collector.BankCode] {
		return ErrReversalDown
	}
	if collector.Balance < amount {
		return ErrInsufficient
	}
	collector.Balance -= amount
	src.Balance += amount + fee
	s.done[ref] = true
	return nil
}
