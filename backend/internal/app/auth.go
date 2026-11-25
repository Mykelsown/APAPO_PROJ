package app

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

const (
	otpWindow   = 10 * time.Minute
	otpMaxSends = 3
	otpMaxWrong = 5
	otpLockFor  = 10 * time.Minute
	pinMaxFails = 5
	pinLockFor  = 15 * time.Minute
)

var mockNames = []string{"Chioma Eze", "Tunde Bakare", "Amina Yusuf", "Emeka Obi", "Ngozi Nwosu", "Seyi Balogun", "Hauwa Musa", "Ifeanyi Udeh"}

// AuthIPLimit applies the per IP bucket shared by all /api/auth/* endpoints.
func (w *World) AuthIPLimit(ip string) *Error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ok, wait := w.allow("authip:"+ip, 30, time.Minute); !ok {
		return &Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "Too many sign in requests. Please wait a moment.", RetryAfter: wait}
	}
	return nil
}

// RequestOTP "sends" a code. The code is always DEV_OTP. It answers the same for known and
// unknown numbers so it never reveals who has an account.
func (w *World) RequestOTP(phone string) *Error {
	p, ok := NormalizePhone(phone)
	if !ok {
		return Errf(http.StatusBadRequest, "invalid_phone", "Enter a valid Nigerian phone number.", nil)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if allowed, wait := w.allow("otp:"+p, otpMaxSends, otpWindow); !allowed {
		return &Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "Too many codes requested. Try again later.", RetryAfter: wait}
	}
	return nil
}

func (w *World) otpFor(p string) *otpState {
	s := w.otp[p]
	if s == nil {
		s = &otpState{}
		w.otp[p] = s
	}
	return s
}

// VerifyOTP checks the code and signs the user in, creating a new user if needed.
func (w *World) VerifyOTP(phone, code string) (*AuthResult, *Error) {
	p, ok := NormalizePhone(phone)
	if !ok {
		return nil, Errf(http.StatusBadRequest, "invalid_phone", "Enter a valid Nigerian phone number.", nil)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.otpFor(p)
	now := w.now()
	if now.Before(s.LockedUntil) {
		secs := int(s.LockedUntil.Sub(now).Seconds()) + 1
		return nil, &Error{Status: http.StatusLocked, Code: "otp_locked", Message: "Too many wrong codes. Try again later.",
			Details: map[string]any{"retry_after_seconds": secs}, RetryAfter: secs}
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(code)), []byte(w.cfg.DevOTP)) != 1 {
		s.WrongTries++
		left := otpMaxWrong - s.WrongTries
		if left <= 0 {
			s.LockedUntil, s.WrongTries = now.Add(otpLockFor), 0
			return nil, &Error{Status: http.StatusLocked, Code: "otp_locked", Message: "Too many wrong codes. Try again in 10 minutes.",
				Details: map[string]any{"retry_after_seconds": int(otpLockFor.Seconds())}, RetryAfter: int(otpLockFor.Seconds())}
		}
		return nil, Errf(http.StatusUnauthorized, "invalid_code", "That code is not correct.", map[string]any{"attempts_left": left})
	}
	s.WrongTries = 0
	u := w.users[w.byPhone[p]]
	if u == nil {
		u = &User{ID: w.nextID("usr"), Phone: p}
		w.users[u.ID] = u
		w.byPhone[p] = u.ID
	}
	tok, exp := w.issueToken(u.ID)
	return &AuthResult{AccessToken: tok, ExpiresAt: exp, User: w.userView(u), Onboarding: onboarding(u)}, nil
}

// Authenticate turns a bearer token into a user id.
func (w *World) Authenticate(token string) (string, *Error) {
	if token == "" {
		return "", Errf(http.StatusUnauthorized, "unauthorized", "Sign in to continue.", nil)
	}
	id, ok := w.verifyToken(token)
	if !ok {
		return "", Errf(http.StatusUnauthorized, "invalid_token", "Your session is not valid. Sign in again.", nil)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.users[id] == nil {
		return "", Errf(http.StatusUnauthorized, "invalid_token", "Your session is not valid. Sign in again.", nil)
	}
	return id, nil
}

// SubmitBVN runs the mock BVN check. Any 11 digits pass. Only a hash and the last 4 are kept.
func (w *World) SubmitBVN(uid, bvn string) (map[string]any, *Error) {
	bvn = strings.TrimSpace(bvn)
	if len(bvn) != 11 || !allDigits(bvn) {
		return nil, Errf(http.StatusBadRequest, "invalid_bvn", "A BVN has exactly 11 digits.", nil)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	u := w.users[uid]
	u.BVNHash, u.BVNLast4 = sha("bvn", bvn), bvn[7:]
	if u.DisplayName == "" {
		// The mock identity service returns a name; the user can change it later.
		u.DisplayName = mockNames[int(bvn[10]-'0')%len(mockNames)]
	}
	return map[string]any{"bvn_verified": true, "display_name": u.DisplayName, "onboarding": onboarding(u)}, nil
}

// SetPIN sets the transaction PIN once, after BVN.
func (w *World) SetPIN(uid, pin string) (map[string]any, *Error) {
	if len(pin) != 4 || !allDigits(pin) {
		return nil, Errf(http.StatusBadRequest, "invalid_pin_format", "A PIN has exactly 4 digits.", nil)
	}
	if weakPINs[pin] {
		return nil, Errf(http.StatusBadRequest, "weak_pin", "Choose a PIN that is not a simple pattern.", nil)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	u := w.users[uid]
	if u.BVNHash == "" {
		return nil, Errf(http.StatusForbidden, "bvn_required", "Verify your BVN before setting a PIN.", nil)
	}
	if len(u.PINHash) > 0 {
		return nil, Errf(http.StatusConflict, "pin_already_set", "A PIN is already set.", nil)
	}
	u.PINHash, u.PINSalt = newPINHash(pin)
	return map[string]any{"pin_set": true, "onboarding": onboarding(u)}, nil
}

// checkPIN verifies a PIN with lockout. Caller holds the lock.
func (w *World) checkPIN(u *User, pin string) *Error {
	now := w.now()
	if now.Before(u.LockedUntil) {
		secs := int(u.LockedUntil.Sub(now).Seconds()) + 1
		return &Error{Status: http.StatusLocked, Code: "pin_locked", Message: "Too many wrong PIN attempts. Try again later.",
			Details: map[string]any{"retry_after_seconds": secs}, RetryAfter: secs}
	}
	if len(pin) != 4 || !pinMatches(u, pin) {
		u.PINFails++
		if u.PINFails >= pinMaxFails {
			u.PINFails, u.LockedUntil = 0, now.Add(pinLockFor)
			return &Error{Status: http.StatusLocked, Code: "pin_locked", Message: "Too many wrong PIN attempts. Try again in 15 minutes.",
				Details: map[string]any{"retry_after_seconds": int(pinLockFor.Seconds())}, RetryAfter: int(pinLockFor.Seconds())}
		}
		return Errf(http.StatusUnauthorized, "invalid_pin", "That PIN is not correct.", map[string]any{"attempts_left": pinMaxFails - u.PINFails})
	}
	u.PINFails = 0
	return nil
}

// Login signs in a user who finished onboarding, with phone and PIN.
func (w *World) Login(phone, pin string) (*AuthResult, *Error) {
	bad := Errf(http.StatusUnauthorized, "invalid_credentials", "Phone number or PIN is not correct.", nil)
	p, ok := NormalizePhone(phone)
	if !ok {
		return nil, bad
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	u := w.users[w.byPhone[p]]
	if u == nil || len(u.PINHash) == 0 {
		return nil, bad
	}
	if e := w.checkPIN(u, pin); e != nil {
		if e.Code == "invalid_pin" {
			return nil, Errf(http.StatusUnauthorized, "invalid_credentials", "Phone number or PIN is not correct.", e.Details)
		}
		return nil, e
	}
	tok, exp := w.issueToken(u.ID)
	return &AuthResult{AccessToken: tok, ExpiresAt: exp, User: w.userView(u), Onboarding: onboarding(u)}, nil
}

// Me returns the signed in user's profile.
func (w *World) Me(uid string) MeView {
	w.mu.Lock()
	defer w.mu.Unlock()
	u := w.users[uid]
	return MeView{User: w.userView(u), Onboarding: onboarding(u), ReceivingAccount: w.accountRef(w.receiving(u))}
}

// UpdateProfile changes the display name.
func (w *World) UpdateProfile(uid, name string) (MeView, *Error) {
	name = strings.Join(strings.Fields(name), " ")
	if n := len([]rune(name)); n < 1 || n > 80 {
		return MeView{}, Errf(http.StatusBadRequest, "invalid_display_name", "Display name must be 1 to 80 characters.", nil)
	}
	w.mu.Lock()
	w.users[uid].DisplayName = name
	w.mu.Unlock()
	return w.Me(uid), nil
}
