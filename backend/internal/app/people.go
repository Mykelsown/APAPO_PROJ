package app

import (
	"net/http"
	"time"
)

// Lookup finds another Apapo user by phone and returns only a masked name.
func (w *World) Lookup(uid, phone string) (map[string]any, *Error) {
	p, ok := NormalizePhone(phone)
	if !ok {
		return nil, Errf(http.StatusBadRequest, "invalid_phone", "Enter a valid Nigerian phone number.", nil)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if allowed, wait := w.allow("lookup:"+uid, 30, time.Minute); !allowed {
		return nil, &Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "Too many lookups. Please wait a moment.", RetryAfter: wait}
	}
	if w.users[uid].Phone == p {
		return nil, Errf(http.StatusBadRequest, "self_send", "That is your own number.", nil)
	}
	u := w.users[w.byPhone[p]]
	if u == nil || !onboarding(u).Complete || w.receiving(u) == nil {
		return nil, errNotFound() // unknown, still signing up, or cannot receive: all look the same
	}
	return map[string]any{"user": map[string]any{"display_name": MaskName(u.DisplayName), "recipient_type": "apapo"}}, nil
}
