// Package config reads all settings from environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Config holds runtime settings. Secret values must never be logged.
type Config struct {
	Addr            string
	AppSecret       []byte
	SecretGenerated bool
	AllowedOrigins  []string
	AllowAnyOrigin  bool
	DevOTP          string
	DemoEnabled     bool
	Provider        string
	TrustProxy      bool
	Version         string
}

// Load builds a Config from a lookup function (os.LookupEnv in production).
// buildVersion is the version baked in at build time, used when VERSION is unset.
func Load(getenv func(string) (string, bool), buildVersion string) (Config, error) {
	get := func(k string) string {
		v, _ := getenv(k)
		return strings.TrimSpace(v)
	}
	c := Config{}

	port := get("PORT")
	if port == "" {
		port = "8080"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return c, fmt.Errorf("PORT must be a number between 1 and 65535")
	}
	c.Addr = "0.0.0.0:" + port
	if a := get("ADDR"); a != "" {
		c.Addr = a
	}

	if s := get("APP_SECRET"); s != "" {
		c.AppSecret = []byte(s)
	} else {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return c, fmt.Errorf("generate APP_SECRET: %w", err)
		}
		c.AppSecret = []byte(hex.EncodeToString(b))
		c.SecretGenerated = true
	}

	for _, o := range strings.Split(get("ALLOWED_ORIGIN"), ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		switch {
		case o == "":
		case o == "*":
			c.AllowAnyOrigin = true
		default:
			c.AllowedOrigins = append(c.AllowedOrigins, o)
		}
	}

	c.DevOTP = get("DEV_OTP")
	if c.DevOTP == "" {
		c.DevOTP = "123456"
	}
	if len(c.DevOTP) != 6 || strings.Trim(c.DevOTP, "0123456789") != "" {
		return c, errors.New("DEV_OTP must be exactly 6 digits")
	}

	c.DemoEnabled = true
	if d := get("DEMO_ENDPOINTS"); d != "" {
		b, err := strconv.ParseBool(d)
		if err != nil {
			return c, errors.New("DEMO_ENDPOINTS must be true or false")
		}
		c.DemoEnabled = b
	}

	c.Provider = strings.ToLower(get("PROVIDER"))
	if c.Provider == "" {
		c.Provider = "mock"
	}
	switch c.Provider {
	case "mock":
	case "real":
		return c, errors.New("PROVIDER=real is not supported: there is no real bank provider, banks are simulated. Use PROVIDER=mock")
	default:
		return c, fmt.Errorf("unknown PROVIDER %q, only mock is supported", c.Provider)
	}

	if tp := get("TRUST_PROXY"); tp != "" {
		b, err := strconv.ParseBool(tp)
		if err != nil {
			return c, errors.New("TRUST_PROXY must be true or false")
		}
		c.TrustProxy = b
	}

	c.Version = get("VERSION")
	if c.Version == "" {
		c.Version = buildVersion
	}
	if c.Version == "" {
		c.Version = "dev"
	}
	return c, nil
}

// Summary is a one line description safe to log: no secret values.
func (c Config) Summary() string {
	secret := "set"
	if c.SecretGenerated {
		secret = "generated (random per start, tokens reset on restart)"
	}
	origins := strings.Join(c.AllowedOrigins, ",")
	if c.AllowAnyOrigin {
		origins = "* (any, demo only)"
	}
	if origins == "" {
		origins = "none (same origin or dev proxy only)"
	}
	return fmt.Sprintf("addr=%s provider=%s version=%s trust_proxy=%t allowed_origin=%s app_secret=%s dev_otp=set(masked)",
		c.Addr, c.Provider, c.Version, c.TrustProxy, origins, secret)
}
