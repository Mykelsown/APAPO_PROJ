package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

const tokenTTL = 24 * time.Hour

type tokenClaims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
}

var b64 = base64.RawURLEncoding

func (w *World) sign(payload []byte) string {
	m := hmac.New(sha256.New, w.cfg.Secret)
	m.Write(payload)
	return b64.EncodeToString(m.Sum(nil))
}

// IssueToken returns a stateless HMAC signed bearer token and its expiry.
func (w *World) issueToken(userID string) (string, time.Time) {
	exp := w.now().Add(tokenTTL)
	p, _ := json.Marshal(tokenClaims{Sub: userID, Exp: exp.Unix()})
	return b64.EncodeToString(p) + "." + w.sign(p), exp
}

// verifyToken checks signature and expiry and returns the user id.
func (w *World) verifyToken(tok string) (string, bool) {
	i := strings.IndexByte(tok, '.')
	if i < 1 {
		return "", false
	}
	p, err := b64.DecodeString(tok[:i])
	if err != nil {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(w.sign(p)), []byte(tok[i+1:])) != 1 {
		return "", false
	}
	var c tokenClaims
	if json.Unmarshal(p, &c) != nil || c.Sub == "" || w.now().Unix() >= c.Exp {
		return "", false
	}
	return c.Sub, true
}
