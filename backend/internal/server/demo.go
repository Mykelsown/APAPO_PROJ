package server

import (
	"net/http"

	"github.com/Mykelsown/apapo-backend/internal/app"
)

// demoOnly hides the demo helpers entirely when DEMO_ENDPOINTS=false.
func (s *Server) demoOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.world.DemoEnabled() {
			s.respond(w, result{}, app.Errf(http.StatusNotFound, "not_found", "This endpoint does not exist.", nil))
			return
		}
		next(w, r)
	}
}

func (s *Server) handleSimulateCredit(r *http.Request, uid string) (result, error) {
	var in app.SimulateIn
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	out, e := s.world.SimulateCredit(uid, in)
	if e != nil {
		return result{}, e
	}
	return created(out)
}

func (s *Server) handleFailure(r *http.Request) (result, error) {
	var in struct {
		AccountID string `json:"account_id"`
		BankCode  string `json:"bank_code"`
		Mode      string `json:"mode"`
	}
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	if e := s.world.SetFailure(in.AccountID, in.BankCode, in.Mode); e != nil {
		return result{}, e
	}
	return ok(map[string]any{"mode": in.Mode, "account_id": in.AccountID, "bank_code": in.BankCode})
}

func (s *Server) handleReset(r *http.Request) (result, error) {
	s.world.Reset()
	return ok(map[string]any{"reset": true, "message": "Demo data restored. Seeded users keep their ids, so signed in demo users stay signed in."})
}
