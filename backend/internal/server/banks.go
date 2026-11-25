package server

import (
	"net/http"

	"github.com/Mykelsown/apapo-backend/internal/app"
)

func (s *Server) handleBanks(r *http.Request, uid string) (result, error) {
	return ok(map[string]any{"banks": app.Banks()})
}

func (s *Server) handleLink(r *http.Request, uid string) (result, error) {
	var in struct {
		BankCode string      `json:"bank_code"`
		Consent  app.Consent `json:"consent"`
	}
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	v, isNew, e := s.world.LinkBank(uid, in.BankCode, in.Consent)
	if e != nil {
		return result{}, e
	}
	if isNew {
		return created(v)
	}
	return ok(v)
}

func (s *Server) handleAccounts(r *http.Request, uid string) (result, error) {
	return ok(map[string]any{"accounts": s.world.Accounts(uid)})
}

func (s *Server) handleRevoke(r *http.Request, uid string) (result, error) {
	v, e := s.world.RevokeLink(uid, r.PathValue("link_id"))
	if e != nil {
		return result{}, e
	}
	return ok(v)
}

func (s *Server) handleDashboard(r *http.Request, uid string) (result, error) {
	return ok(s.world.Dashboard(uid, queryInt(r, "limit", 20, 1, 100)))
}
