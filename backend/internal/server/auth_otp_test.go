package server

import "testing"

func TestOTPRequest(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name string
		body any
		want int
		code string
	}{
		{"known user", map[string]string{"phone": "08012345678"}, 200, ""},
		{"unknown number answers the same", map[string]string{"phone": "+2347011112222"}, 200, ""},
		{"bad phone", map[string]string{"phone": "123"}, 400, "invalid_phone"},
		{"bad json", "{nope", 400, "invalid_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/auth/otp", "", tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
		})
	}
}

func TestOTPRateLimitPerPhone(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 3; i++ {
		if r := e.call("POST", "/api/auth/otp", "", map[string]string{"phone": "08055550001"}); r.Status != 200 {
			t.Fatalf("send %d: %d", i, r.Status)
		}
	}
	r := e.call("POST", "/api/auth/otp", "", map[string]string{"phone": "+2348055550001"})
	if r.Status != 429 || errCode(r) != "rate_limited" || r.Header.Get("Retry-After") == "" {
		t.Fatalf("got %d %q retry=%q", r.Status, errCode(r), r.Header.Get("Retry-After"))
	}
	// A different phone is not affected.
	if r := e.call("POST", "/api/auth/otp", "", map[string]string{"phone": "08055550002"}); r.Status != 200 {
		t.Fatalf("other phone blocked: %d", r.Status)
	}
}

func TestAuthIPBucket(t *testing.T) {
	e := newEnv(t)
	var last reply
	for i := 0; i < 31; i++ {
		last = e.call("POST", "/api/auth/verify-otp", "", map[string]string{"phone": "08011110000", "code": "000000"})
	}
	if last.Status != 429 || errCode(last) != "rate_limited" {
		t.Fatalf("31st call from one IP: %d %v", last.Status, last.Body)
	}
}

func TestVerifyOTP(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name  string
		body  any
		want  int
		code  string
		fresh bool
	}{
		{"right code, existing user", map[string]string{"phone": "08012345678", "code": "123456"}, 200, "", false},
		{"right code, new user", map[string]string{"phone": "+2347022223333", "code": "123456"}, 200, "", true},
		{"wrong code", map[string]string{"phone": "08012345678", "code": "000000"}, 401, "invalid_code", false},
		{"bad phone", map[string]string{"phone": "x", "code": "123456"}, 400, "invalid_phone", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/auth/verify-otp", "", tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
			if tc.want == 200 {
				if r.Body["access_token"] == "" || r.Body["expires_at"] == nil {
					t.Fatalf("missing token fields: %v", r.Body)
				}
				if got := get(r.Body, "onboarding", "complete"); got != !tc.fresh {
					t.Fatalf("onboarding.complete = %v, fresh=%v", got, tc.fresh)
				}
			}
		})
	}
}

func TestVerifyOTPLocksAfterFiveWrongTries(t *testing.T) {
	e := newEnv(t)
	body := map[string]string{"phone": "08033334444", "code": "999999"}
	for i := 0; i < 4; i++ {
		if r := e.call("POST", "/api/auth/verify-otp", "", body); r.Status != 401 {
			t.Fatalf("try %d: %d", i, r.Status)
		}
		e.s.world.AuthIPLimit("reset-noise") // keep calls cheap; IP bucket is separate
	}
	r := e.call("POST", "/api/auth/verify-otp", "", body)
	if r.Status != 423 || errCode(r) != "otp_locked" {
		t.Fatalf("5th wrong try: %d %v", r.Status, r.Body)
	}
	// Even the right code is refused while locked.
	r = e.call("POST", "/api/auth/verify-otp", "", map[string]string{"phone": "08033334444", "code": "123456"})
	if r.Status != 423 {
		t.Fatalf("locked user got %d", r.Status)
	}
}
