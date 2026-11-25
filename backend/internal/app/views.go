package app

import "time"

// UserView is the public shape of the signed in user.
type UserView struct {
	ID          string `json:"id"`
	Phone       string `json:"phone"`
	DisplayName string `json:"display_name"`
}

// OnboardingView says how far sign up has gone.
type OnboardingView struct {
	BVNVerified bool `json:"bvn_verified"`
	PINSet      bool `json:"pin_set"`
	Complete    bool `json:"complete"`
}

// AccountRef is a masked account reference.
type AccountRef struct {
	AccountID     string `json:"account_id"`
	BankCode      string `json:"bank_code"`
	BankName      string `json:"bank_name"`
	MaskedAccount string `json:"masked_account_number"`
}

// MeView is the response of GET /api/me.
type MeView struct {
	User             UserView       `json:"user"`
	Onboarding       OnboardingView `json:"onboarding"`
	ReceivingAccount *AccountRef    `json:"receiving_account"`
}

// AuthResult is returned by verify-otp.
type AuthResult struct {
	AccessToken string         `json:"access_token"`
	ExpiresAt   time.Time      `json:"expires_at"`
	User        UserView       `json:"user"`
	Onboarding  OnboardingView `json:"onboarding"`
}

func (w *World) userView(u *User) UserView {
	return UserView{ID: u.ID, Phone: u.Phone, DisplayName: u.DisplayName}
}

func onboarding(u *User) OnboardingView {
	o := OnboardingView{BVNVerified: u.BVNHash != "", PINSet: len(u.PINHash) > 0}
	o.Complete = o.BVNVerified && o.PINSet && u.DisplayName != ""
	return o
}

func (w *World) accountRef(a *Account) *AccountRef {
	if a == nil {
		return nil
	}
	return &AccountRef{AccountID: a.ID, BankCode: a.BankCode, BankName: BankName(a.BankCode), MaskedAccount: MaskNumber(a.Number)}
}

// receiving returns the user's active receiving account, or nil.
func (w *World) receiving(u *User) *Account {
	if a := w.accounts[u.ReceivingAccountID]; a != nil && a.Active {
		return a
	}
	return nil
}
