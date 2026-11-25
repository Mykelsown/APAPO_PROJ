package server

import "net/http"

// routes registers every API route. Each endpoint is added in its own commit.
func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/otp", s.authLimited(s.handleOTP))
	mux.HandleFunc("POST /api/auth/verify-otp", s.authLimited(s.handleVerifyOTP))
	mux.HandleFunc("POST /api/auth/bvn", s.authed(s.handleBVN))
	mux.HandleFunc("POST /api/auth/pin", s.authed(s.handlePIN))
	mux.HandleFunc("POST /api/auth/login", s.authLimited(s.handleLogin))
	mux.HandleFunc("GET /api/me", s.authed(s.handleMe))
	mux.HandleFunc("PUT /api/me/profile", s.authed(s.handleProfile))
	mux.HandleFunc("GET /api/banks", s.authed(s.handleBanks))
	mux.HandleFunc("POST /api/banks/link", s.authed(s.handleLink))
	mux.HandleFunc("GET /api/accounts", s.authed(s.handleAccounts))
	mux.HandleFunc("DELETE /api/banks/{link_id}", s.authed(s.handleRevoke))
	mux.HandleFunc("GET /api/dashboard", s.authed(s.handleDashboard))
	mux.HandleFunc("GET /api/users/lookup", s.authed(s.handleLookup))
	mux.HandleFunc("POST /api/splitpay/preview", s.authed(s.handlePreview))
	mux.HandleFunc("POST /api/splitpay/execute", s.executeHandler())
	mux.HandleFunc("GET /api/splitpay/{id}", s.authed(s.handlePaymentStatus))
	mux.HandleFunc("GET /api/payments", s.authed(s.handlePayments))
	mux.HandleFunc("GET /api/recipients/recent", s.authed(s.handleRecent))
	mux.HandleFunc("POST /api/risk/check", s.authed(s.handleRiskCheck))
	mux.HandleFunc("POST /api/reports", s.authed(s.handleReport))
	mux.HandleFunc("GET /api/shield/check", s.authed(s.handleShieldCheck))
	mux.HandleFunc("POST /api/shield/dismiss", s.authed(s.handleShieldDismiss))
	mux.HandleFunc("PUT /api/splitreceive/rules", s.authed(s.handlePutRules))
	mux.HandleFunc("GET /api/splitreceive/rules", s.authed(s.handleGetRules))
	mux.HandleFunc("POST /api/splitreceive/scan", s.authed(s.handleScan))
}
