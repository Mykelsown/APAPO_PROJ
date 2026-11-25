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
}
