package server

import (
	"net/http"

	"github.com/Mykelsown/apapo-backend/internal/app"
)

func (s *Server) handlePreview(r *http.Request, uid string) (result, error) {
	var in app.PreviewIn
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	out, e := s.world.Preview(uid, in)
	if e != nil {
		return result{}, e
	}
	return ok(out)
}

// executeHandler is written out by hand because it sets a response header on replays.
func (s *Server) executeHandler() http.HandlerFunc {
	return s.authedW(func(w http.ResponseWriter, r *http.Request, uid string) (result, error) {
		var in app.ExecuteIn
		if err := decode(r, &in); err != nil {
			return result{}, err
		}
		v, replay, e := s.world.Execute(uid, in, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return result{}, e
		}
		if replay {
			w.Header().Set("Idempotent-Replay", "true")
		}
		return accepted(v)
	})
}

func (s *Server) handlePaymentStatus(r *http.Request, uid string) (result, error) {
	v, e := s.world.PaymentStatus(uid, r.PathValue("id"))
	if e != nil {
		return result{}, e
	}
	return ok(v)
}

func (s *Server) handlePayments(r *http.Request, uid string) (result, error) {
	list, next, e := s.world.Payments(uid, queryInt(r, "limit", 20, 1, 100), r.URL.Query().Get("before"))
	if e != nil {
		return result{}, e
	}
	var nb any
	if next != "" {
		nb = next
	}
	return ok(map[string]any{"payments": list, "next_before": nb})
}

func (s *Server) handleRecent(r *http.Request, uid string) (result, error) {
	return ok(map[string]any{"recipients": s.world.RecentRecipients(uid)})
}
