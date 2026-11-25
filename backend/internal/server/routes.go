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
}
