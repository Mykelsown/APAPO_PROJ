package server

import (
	"net/http"
	"strconv"

	"github.com/Mykelsown/apapo-backend/internal/app"
)

func (s *Server) handleRiskCheck(r *http.Request, uid string) (result, error) {
	var in app.PreviewIn
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	out, e := s.world.RiskCheck(uid, in)
	if e != nil {
		return result{}, e
	}
	return ok(out)
}

func (s *Server) handleReport(r *http.Request, uid string) (result, error) {
	var in app.ReportIn
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	made, e := s.world.Report(uid, in)
	if e != nil {
		return result{}, e
	}
	if !made {
		return ok(map[string]any{"status": "already_reported"})
	}
	return created(map[string]any{"status": "reported", "message": "Thank you. Your report helps protect other people."})
}

func (s *Server) handleShieldCheck(r *http.Request, uid string) (result, error) {
	n, err := strconv.ParseInt(r.URL.Query().Get("amount_kobo"), 10, 64)
	if err != nil {
		n = 0
	}
	alert, e := s.world.ShieldCheck(uid, n)
	if e != nil {
		return result{}, e
	}
	return ok(map[string]any{"alert": alert})
}

func (s *Server) handleShieldDismiss(r *http.Request, uid string) (result, error) {
	var in struct {
		CreditID string `json:"credit_id"`
	}
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	if e := s.world.ShieldDismiss(uid, in.CreditID); e != nil {
		return result{}, e
	}
	return ok(map[string]any{"dismissed": true})
}
