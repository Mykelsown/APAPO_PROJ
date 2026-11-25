package server

import "testing"

func TestShieldDismiss(t *testing.T) {
	e := newEnv(t)
	sam, bola := e.sam(), e.bola()
	alert := shield(e, sam, "8000000").Body["alert"].(map[string]any)
	id := alert["credit_id"].(string)
	tests := []struct {
		name  string
		token string
		body  any
		want  int
		code  string
	}{
		{"another user cannot dismiss my alert", bola, map[string]string{"credit_id": id}, 404, "not_found"},
		{"unknown credit", sam, map[string]string{"credit_id": "crd_nope"}, 404, "not_found"},
		{"missing id", sam, map[string]string{}, 404, "not_found"},
		{"no token", "", map[string]string{"credit_id": id}, 401, "unauthorized"},
		{"bad json", sam, "{", 400, "invalid_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := e.call("POST", "/api/shield/dismiss", tc.token, tc.body)
			if r.Status != tc.want || errCode(r) != tc.code {
				t.Fatalf("got %d %q (%v)", r.Status, errCode(r), r.Body)
			}
		})
	}
	if shield(e, sam, "8000000").Body["alert"] == nil {
		t.Fatal("a failed dismiss must not hide the alert")
	}
	r := e.call("POST", "/api/shield/dismiss", sam, map[string]string{"credit_id": id})
	if r.Status != 200 || r.Body["dismissed"] != true {
		t.Fatalf("%d %v", r.Status, r.Body)
	}
	if shield(e, sam, "8000000").Body["alert"] != nil {
		t.Fatal("alert still shown after dismiss")
	}
	if r := e.call("POST", "/api/shield/dismiss", sam, map[string]string{"credit_id": id}); r.Status != 200 {
		t.Fatalf("dismiss twice should be harmless: %d", r.Status)
	}
	// Preview no longer carries the warning either.
	p := e.call("POST", "/api/splitpay/preview", sam, previewBody(80000, outside("delta", "1000000001")))
	if p.Body["shield_warning"] != nil {
		t.Fatalf("preview still warns: %v", p.Body["shield_warning"])
	}
}
