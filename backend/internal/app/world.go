package app

import (
	"time"
)

// New builds a seeded world.
func New(cfg Config) *World {
	w := &World{cfg: cfg, clock: func() time.Time { return time.Now().UTC() }, Async: true}
	w.seed()
	return w
}

// SetClock replaces the clock. Tests use it to move time forward.
func (w *World) SetClock(f func() time.Time) { w.mu.Lock(); w.clock = f; w.mu.Unlock() }

// DemoEnabled says whether the demo helper endpoints are on.
func (w *World) DemoEnabled() bool { return w.cfg.DemoEnabled }

// Seed identities, documented in the README. Demo only.
const (
	SeedUser1Phone = "+2348012345678"
	SeedUser1PIN   = "4821"
	SeedUser2Phone = "+2348098765432"
	SeedUser2PIN   = "5912"
	FlaggedBank    = "apex"
	FlaggedNumber  = "9000000001"
	GoodPayee      = "1000000001"
)

func (w *World) reset() {
	w.seq = 0
	w.users = map[string]*User{}
	w.byPhone = map[string]string{}
	w.links = map[string]*Link{}
	w.accounts = map[string]*Account{}
	w.accOrder = nil
	w.txs = nil
	w.credits = nil
	w.payments = map[string]*Payment{}
	w.payOrder = nil
	w.idem = map[string]idemRec{}
	w.rules = map[string]*Rules{}
	w.runs = map[string]*Run{}
	w.runOrder = nil
	w.runByCred = map[string]string{}
	w.reports = map[string]map[string]time.Time{}
	w.repLog = map[string][]time.Time{}
	w.repSalt = []byte(randHex(16))
	w.dismissed = map[string]bool{}
	w.otp = map[string]*otpState{}
	w.buckets = map[string][]time.Time{}
	w.sim = newBankSim()
	w.goodPayees = map[string]bool{GoodPayee: true}
}

func (w *World) seed() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seedLocked()
}

func (w *World) seedLocked() {
	w.reset()
	now := w.now()
	type acct struct {
		bank   string
		number string
		naira  int64
	}
	mk := func(name, phone, pin, bvn string, accts []acct) *User {
		u := &User{ID: w.nextID("usr"), Phone: phone, DisplayName: name}
		u.BVNHash, u.BVNLast4 = sha("bvn", bvn), bvn[len(bvn)-4:]
		u.PINHash, u.PINSalt = newPINHash(pin)
		w.users[u.ID] = u
		w.byPhone[phone] = u.ID
		for _, a := range accts {
			acc := w.linkBank(u, a.bank, []string{"accounts", "balances", "transactions"}, now)
			acc.Balance = a.naira * 100
			acc.Number = a.number
		}
		return u
	}
	u1 := mk("Samuel Pelumi", SeedUser1Phone, SeedUser1PIN, "22212345678", []acct{
		{"apex", "2001001001", 70000}, {"delta", "2002002002", 60000},
		{"savanna", "2003003003", 45000}, {"harbor", "2004004004", 20000},
	})
	u2 := mk("Bola Adeyemi", SeedUser2Phone, SeedUser2PIN, "22298765432", []acct{
		{"apex", "2009009009", 30000},
	})
	for _, u := range []*User{u1, u2} {
		for _, id := range w.accOrder {
			if a := w.accounts[id]; a.UserID == u.ID && a.BankCode == "apex" {
				u.ReceivingAccountID = a.ID
			}
		}
	}
	// A little history so the dashboard feed is not empty.
	apex1, delta1, sav1 := w.userAccount(u1, "apex"), w.userAccount(u1, "delta"), w.userAccount(u1, "savanna")
	hist := []struct {
		a        *Account
		dir      string
		amt      int64
		who, cat string
		ago      time.Duration
	}{
		{apex1, "credit", 25000000, "Brightpath Ltd salary", "income", 72 * time.Hour},
		{delta1, "debit", 1850000, "Shoprite", "groceries", 50 * time.Hour},
		{sav1, "debit", 350000, "MTN data", "airtime", 30 * time.Hour},
		{apex1, "debit", 420000, "Uber", "transport", 20 * time.Hour},
		{delta1, "debit", 1250000, "EKEDC electricity", "utilities", 8 * time.Hour},
	}
	for _, h := range hist {
		w.addTx(h.a, h.dir, h.amt, h.who, h.cat, true, now.Add(-h.ago))
	}
	w.addTx(w.userAccount(u2, "apex"), "credit", 3000000, "Opening deposit", "income", true, now.Add(-96*time.Hour))
	// The N80,000 credit the bank has not confirmed yet, for Shield.
	c := &Credit{ID: w.nextID("crd"), UserID: u1.ID, AccountID: apex1.ID, Amount: 8000000,
		Sender: "Greenfield Traders", Reference: "bank_" + randHex(4), Verified: false, At: now.Add(-2 * time.Hour)}
	w.credits = append(w.credits, c)
	w.addTx(apex1, "credit", c.Amount, c.Sender, "income", false, c.At)
	// The flagged recipient is reported by several independent people. Salted hashes only.
	ah := w.acctHash(FlaggedBank, FlaggedNumber)
	w.reports[ah] = map[string]time.Time{}
	for i := 0; i < 4; i++ {
		w.reports[ah][sha(string(w.repSalt), "seed-reporter", string(rune('a'+i)), ah)] = now.Add(-time.Duration(i+1) * 24 * time.Hour)
	}
}

func (w *World) userAccount(u *User, bank string) *Account {
	for _, id := range w.accOrder {
		a := w.accounts[id]
		if a.UserID == u.ID && a.BankCode == bank && a.Active {
			return a
		}
	}
	return nil
}

// addTx appends a feed line. Caller holds the lock.
func (w *World) addTx(a *Account, dir string, amt int64, who, cat string, verified bool, at time.Time) {
	w.txs = append(w.txs, &Tx{ID: w.nextID("txn"), UserID: a.UserID, AccountID: a.ID, Direction: dir,
		Amount: amt, Counterparty: who, Category: cat, Verified: verified, At: at})
}

// Reset restores every seed, clears payments, runs, reports and failure switches.
func (w *World) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seedLocked()
}
