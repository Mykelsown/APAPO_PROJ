package server

import (
	"net/http"

	"github.com/Mykelsown/apapo-backend/internal/app"
)

func (s *Server) handlePutRules(r *http.Request, uid string) (result, error) {
	var in app.RulesIn
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	v, e := s.world.PutRules(uid, in)
	if e != nil {
		return result{}, e
	}
	return ok(map[string]any{"rules": v})
}

func (s *Server) handleGetRules(r *http.Request, uid string) (result, error) {
	return ok(map[string]any{"rules": s.world.GetRules(uid)})
}

func (s *Server) handleScan(r *http.Request, uid string) (result, error) {
	runs := s.world.Scan(uid)
	return ok(map[string]any{"new_runs": runs, "count": len(runs)})
}

func (s *Server) handleRuns(r *http.Request, uid string) (result, error) {
	return ok(map[string]any{"runs": s.world.Runs(uid)})
}

func (s *Server) handleRetryRun(r *http.Request, uid string) (result, error) {
	v, e := s.world.RetryRun(uid, r.PathValue("id"))
	if e != nil {
		return result{}, e
	}
	return ok(map[string]any{"run": v})
}

func (s *Server) handleRecon(r *http.Request, uid string) (result, error) {
	entries, totals := s.world.Reconciliation(uid)
	return ok(map[string]any{"credits": entries, "totals": totals})
}
