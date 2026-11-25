package app

import (
	"testing"
	"time"
)

func testWorld(t *testing.T) *World {
	t.Helper()
	w := New(Config{Secret: []byte("test-secret"), DevOTP: "123456", DemoEnabled: true})
	w.Async = false
	return w
}

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"08012345678", "+2348012345678", true},
		{"+2348012345678", "+2348012345678", true},
		{"2348012345678", "+2348012345678", true},
		{"0801 234 5678", "+2348012345678", true},
		{"0601234567", "", false},
		{"0801234567", "", false},
		{"abc", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizePhone(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizePhone(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMasks(t *testing.T) {
	if got := MaskName("Bola Adeyemi"); got != "Bola A." {
		t.Fatalf("got %q", got)
	}
	if got := MaskName("Madonna"); got != "Madonna" {
		t.Fatalf("got %q", got)
	}
	if got := MaskNumber("2001001001"); got != "******1001" {
		t.Fatalf("got %q", got)
	}
}

func TestTokenRoundTripAndTamper(t *testing.T) {
	w := testWorld(t)
	tok, _ := w.issueToken("usr_1")
	if id, ok := w.verifyToken(tok); !ok || id != "usr_1" {
		t.Fatalf("valid token rejected")
	}
	if _, ok := w.verifyToken(tok + "x"); ok {
		t.Fatalf("tampered signature accepted")
	}
	now := time.Now().UTC()
	w.SetClock(func() time.Time { return now.Add(25 * time.Hour) })
	if _, ok := w.verifyToken(tok); ok {
		t.Fatalf("expired token accepted")
	}
}

func TestPINHash(t *testing.T) {
	u := &User{}
	u.PINHash, u.PINSalt = newPINHash("4821")
	if !pinMatches(u, "4821") || pinMatches(u, "4822") {
		t.Fatalf("pin check wrong")
	}
	u2 := &User{}
	u2.PINHash, u2.PINSalt = newPINHash("4821")
	if string(u.PINHash) == string(u2.PINHash) {
		t.Fatalf("salts must differ")
	}
}

func TestSeedBalancesAndIdempotentTransfer(t *testing.T) {
	w := testWorld(t)
	u := w.users[w.byPhone[SeedUser1Phone]]
	var total int64
	for _, id := range w.accOrder {
		if a := w.accounts[id]; a.UserID == u.ID {
			total += a.Balance
		}
	}
	if total != 19500000 {
		t.Fatalf("seed total = %d, want 19500000", total)
	}
	a, b := w.userAccount(u, "apex"), w.userAccount(u, "delta")
	if err := w.transfer("r1", a, b, 100, 10); err != nil {
		t.Fatal(err)
	}
	if err := w.transfer("r1", a, b, 100, 10); err != nil {
		t.Fatal(err)
	}
	if a.Balance != 7000000-110 || b.Balance != 6000000+100 {
		t.Fatalf("same reference moved money twice: %d %d", a.Balance, b.Balance)
	}
}

func TestTimeoutMovesMoneyOnceThenRetryIsSafe(t *testing.T) {
	w := testWorld(t)
	u := w.users[w.byPhone[SeedUser1Phone]]
	a, b := w.userAccount(u, "apex"), w.userAccount(u, "delta")
	w.sim.modes["acct:"+b.ID] = ModeTimeout
	if err := w.transfer("r2", a, b, 500, 0); err != ErrTimeout {
		t.Fatalf("want timeout, got %v", err)
	}
	if err := w.transfer("r2", a, b, 500, 0); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if a.Balance != 7000000-500 {
		t.Fatalf("double debit: %d", a.Balance)
	}
}
