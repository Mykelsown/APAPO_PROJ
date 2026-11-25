package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// NormalizePhone accepts 080..., +23480... or 23480... and returns +234XXXXXXXXXX.
func NormalizePhone(in string) (string, bool) {
	s := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' {
			return -1
		}
		return r
	}, in)
	var rest string
	switch {
	case strings.HasPrefix(s, "+234"):
		rest = s[4:]
	case strings.HasPrefix(s, "234"):
		rest = s[3:]
	case strings.HasPrefix(s, "0"):
		rest = s[1:]
	default:
		return "", false
	}
	if len(rest) != 10 || !allDigits(rest) || rest[0] < '7' || rest[0] > '9' {
		return "", false
	}
	return "+234" + rest, true
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// MaskName turns "Bola Adeyemi" into "Bola A.".
func MaskName(name string) string {
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
		return "Apapo user"
	case 1:
		return parts[0]
	}
	r := []rune(parts[len(parts)-1])
	return parts[0] + " " + strings.ToUpper(string(r[0])) + "."
}

// MaskNumber shows only the last four digits.
func MaskNumber(n string) string {
	if len(n) <= 4 {
		return n
	}
	return strings.Repeat("*", len(n)-4) + n[len(n)-4:]
}

func sha(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// nextID returns a sortable id with a prefix, for example pay_000012_ab12cd.
func (w *World) nextID(prefix string) string {
	w.seq++
	return fmt.Sprintf("%s_%06d_%s", prefix, w.seq, randHex(3))
}

func (w *World) now() time.Time { return w.clock() }

// allow is a sliding window limiter: at most max events per window for a key.
// It returns false (and how long to wait) when the limit is hit. Caller holds the lock.
func (w *World) allow(key string, max int, window time.Duration) (bool, int) {
	now := w.now()
	kept := w.buckets[key][:0]
	for _, t := range w.buckets[key] {
		if now.Sub(t) < window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= max {
		w.buckets[key] = kept
		wait := int(window.Seconds() - now.Sub(kept[0]).Seconds())
		if wait < 1 {
			wait = 1
		}
		return false, wait
	}
	w.buckets[key] = append(kept, now)
	return true, 0
}
