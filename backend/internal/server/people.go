package server

import "net/http"

func (s *Server) handleLookup(r *http.Request, uid string) (result, error) {
	v, e := s.world.Lookup(uid, r.URL.Query().Get("phone"))
	if e != nil {
		return result{}, e
	}
	return ok(v)
}
