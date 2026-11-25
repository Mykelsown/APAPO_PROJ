package server

import (
	"net/http"
)

func (s *Server) handleOTP(r *http.Request) (result, error) {
	var in struct {
		Phone string `json:"phone"`
	}
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	if e := s.world.RequestOTP(in.Phone); e != nil {
		return result{}, e
	}
	return ok(map[string]any{"sent": true, "expires_in_seconds": 300, "message": "If the number can receive a code, it was sent. In this demo the code is the DEV_OTP value."})
}

func (s *Server) handleVerifyOTP(r *http.Request) (result, error) {
	var in struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	res, e := s.world.VerifyOTP(in.Phone, in.Code)
	if e != nil {
		return result{}, e
	}
	return ok(res)
}

func (s *Server) handleBVN(r *http.Request, uid string) (result, error) {
	var in struct {
		BVN string `json:"bvn"`
	}
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	res, e := s.world.SubmitBVN(uid, in.BVN)
	if e != nil {
		return result{}, e
	}
	return ok(res)
}

func (s *Server) handlePIN(r *http.Request, uid string) (result, error) {
	var in struct {
		PIN string `json:"pin"`
	}
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	res, e := s.world.SetPIN(uid, in.PIN)
	if e != nil {
		return result{}, e
	}
	return ok(res)
}

func (s *Server) handleLogin(r *http.Request) (result, error) {
	var in struct {
		Phone string `json:"phone"`
		PIN   string `json:"pin"`
	}
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	res, e := s.world.Login(in.Phone, in.PIN)
	if e != nil {
		return result{}, e
	}
	return ok(res)
}

func (s *Server) handleMe(r *http.Request, uid string) (result, error) {
	return ok(s.world.Me(uid))
}

func (s *Server) handleProfile(r *http.Request, uid string) (result, error) {
	var in struct {
		DisplayName string `json:"display_name"`
	}
	if err := decode(r, &in); err != nil {
		return result{}, err
	}
	me, e := s.world.UpdateProfile(uid, in.DisplayName)
	if e != nil {
		return result{}, e
	}
	return ok(me)
}
