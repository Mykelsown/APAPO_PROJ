package app

import "net/http"

// SetFailure switches a simulated bank fault on or off. Demo only.
//
//	none                 clears faults for the account or bank (and reversal outages for a bank);
//	                     with neither it clears every fault
//	insufficient_funds   debits from the account or bank fail
//	bank_unavailable     the bank is down: transfers fail and the dashboard marks it unavailable
//	timeout              the next transfer touching the target moves money but reports a timeout
//	reversal_down        reversals touching the bank fail while normal transfers still work
func (w *World) SetFailure(accountID, bankCode, mode string) *Error {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch mode {
	case ModeNone, ModeInsufficient, ModeUnavailable, ModeTimeout, ModeReversalDown:
	default:
		return Errf(http.StatusBadRequest, "invalid_mode", "Mode must be none, insufficient_funds, bank_unavailable, timeout or reversal_down.", nil)
	}
	if accountID != "" && bankCode != "" {
		return Errf(http.StatusBadRequest, "invalid_target", "Send either account_id or bank_code, not both.", nil)
	}
	if accountID != "" && w.accounts[accountID] == nil {
		return Errf(http.StatusNotFound, "not_found", "Unknown account.", nil)
	}
	if bankCode != "" {
		if known, _ := bankSupported(bankCode); !known {
			return Errf(http.StatusBadRequest, "unknown_bank", "That bank is not in the catalogue.", nil)
		}
	}
	s := w.sim
	if mode == ModeNone {
		switch {
		case accountID != "":
			delete(s.modes, "acct:"+accountID)
		case bankCode != "":
			delete(s.modes, "bank:"+bankCode)
			delete(s.reverseDown, bankCode)
		default:
			s.modes, s.reverseDown = map[string]string{}, map[string]bool{}
		}
		return nil
	}
	if mode == ModeReversalDown {
		if bankCode == "" {
			return Errf(http.StatusBadRequest, "bank_code_required", "reversal_down needs a bank_code.", nil)
		}
		s.reverseDown[bankCode] = true
		return nil
	}
	switch {
	case accountID != "":
		s.modes["acct:"+accountID] = mode
	case bankCode != "":
		s.modes["bank:"+bankCode] = mode
	default:
		return Errf(http.StatusBadRequest, "invalid_target", "Send account_id or bank_code.", nil)
	}
	return nil
}
