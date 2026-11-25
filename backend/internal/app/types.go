// Package app holds the Apapo domain: users, simulated banks, payments and ledgers.
// Everything lives in memory behind one lock. There is no database and no real bank.
package app

import (
	"net/http"
	"sync"
	"time"
)

// Error is a domain error that maps straight onto the API error shape.
type Error struct {
	Status     int
	Code       string
	Message    string
	Details    map[string]any
	RetryAfter int // seconds, 0 when not applicable
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Errf builds an Error.
func Errf(status int, code, msg string, details map[string]any) *Error {
	return &Error{Status: status, Code: code, Message: msg, Details: details}
}

// Common errors reused across the domain.
func errNotFound() *Error {
	return Errf(http.StatusNotFound, "not_found", "We could not find that.", nil)
}
func errOnboarding() *Error {
	return Errf(http.StatusForbidden, "onboarding_required", "Finish sign up first: verify your BVN and set a PIN.", nil)
}

// Config carries the settings the domain needs.
type Config struct {
	Secret      []byte
	DevOTP      string
	DemoEnabled bool
}

// User is an Apapo customer.
type User struct {
	ID                 string
	Phone              string // +234XXXXXXXXXX
	DisplayName        string
	BVNHash            string
	BVNLast4           string
	PINHash            []byte
	PINSalt            []byte
	PINFails           int
	LockedUntil        time.Time
	ReceivingAccountID string
}

// Link is a consented connection to one bank.
type Link struct {
	ID        string
	UserID    string
	BankCode  string
	Scopes    []string
	LinkedAt  time.Time
	ExpiresAt time.Time
	Revoked   bool
}

// Account is one simulated bank account owned by a user.
type Account struct {
	ID       string
	UserID   string
	LinkID   string
	BankCode string
	Number   string
	Balance  int64 // kobo
	Active   bool
}

// Tx is one line in the merged transaction feed.
type Tx struct {
	ID           string
	UserID       string
	AccountID    string
	Direction    string // credit or debit
	Amount       int64
	Counterparty string
	Category     string
	Verified     bool
	At           time.Time
}

// Credit is money that arrived in an account from outside (or from another Apapo user).
type Credit struct {
	ID        string
	UserID    string
	AccountID string
	Amount    int64
	Sender    string
	Reference string
	Verified  bool
	At        time.Time
}

// Recipient identifies who a payment goes to.
type Recipient struct {
	Type        string // outside or apapo
	BankCode    string
	Number      string // outside account number
	AccountName string // resolved name for outside recipients
	UserID      string // apapo recipient
	AccountID   string // apapo recipient's receiving account (server side only)
	Masked      string // masked display name for apapo recipients
	Key         string // stable identity used for history and risk
}

// Leg is one account's share of a Split-Pay payment.
type Leg struct {
	AccountID   string
	BankCode    string
	Amount      int64
	Fee         int64
	State       string // pending, collected, failed, reversed
	IsCollector bool
	Ref         string
	RevRef      string
}

// Payment is one Split-Pay payment.
type Payment struct {
	ID               string
	Seq              int64
	UserID           string
	State            string
	Amount           int64
	Fee              int64
	Legs             []*Leg
	CollectorID      string
	Recipient        Recipient
	FailureReason    string
	IdemKey          string
	BodyHash         string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RevAttempts      int
	LastRevAt        time.Time
	NextRevAt        time.Time
	HumanReview      bool
	PayoutDone       bool
	RiskAcknowledged bool
	AttemptLog       []string
}

// Destination is one Split-Receive target.
type Destination struct {
	ID          string
	Kind        string // own or outside
	AccountID   string
	BankCode    string
	Number      string
	AccountName string
	Label       string
	PercentBP   int64 // basis points (percent * 100), 0 when fixed
	AmountKobo  int64 // fixed amount, 0 when percent
}

// Rules are a user's Split-Receive settings.
type Rules struct {
	UserID             string
	ReceivingAccountID string
	Destinations       []Destination
	ConsentAt          time.Time
	UpdatedAt          time.Time
}

// RunLeg is one share of a Split-Receive run.
type RunLeg struct {
	ID          string
	Destination Destination
	Amount      int64
	State       string // pending, sent, failed
	Ref         string
	Error       string
}

// Run is the split of one incoming credit.
type Run struct {
	ID        string
	UserID    string
	CreditID  string
	Amount    int64
	Legs      []*RunLeg
	CreatedAt time.Time
}

type idemRec struct {
	BodyHash  string
	PaymentID string
}

type otpState struct {
	Sends       []time.Time
	WrongTries  int
	LockedUntil time.Time
}

// World is the whole in-memory state.
type World struct {
	mu    sync.Mutex
	cfg   Config
	clock func() time.Time
	// Async runs payments in the background. Tests turn it off for determinism.
	Async bool

	seq        int64
	users      map[string]*User
	byPhone    map[string]string
	links      map[string]*Link
	accounts   map[string]*Account
	accOrder   []string
	txs        []*Tx
	credits    []*Credit
	payments   map[string]*Payment
	payOrder   []string
	idem       map[string]idemRec
	rules      map[string]*Rules
	runs       map[string]*Run
	runOrder   []string
	runByCred  map[string]string
	reports    map[string]map[string]time.Time // account hash -> reporter hash -> when
	repLog     map[string][]time.Time          // user id -> report times
	repSalt    []byte
	dismissed  map[string]bool
	otp        map[string]*otpState
	buckets    map[string][]time.Time
	sim        *bankSim
	goodPayees map[string]bool
}
