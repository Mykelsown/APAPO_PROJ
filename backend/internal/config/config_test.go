package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
		check   func(t *testing.T, c Config)
	}{
		{name: "defaults", env: map[string]string{}, check: func(t *testing.T, c Config) {
			if c.Addr != "0.0.0.0:8080" || c.Provider != "mock" || c.DevOTP != "123456" || c.Version != "build1" {
				t.Fatalf("bad defaults: %+v", c)
			}
			if !c.SecretGenerated || len(c.AppSecret) == 0 {
				t.Fatal("expected generated secret")
			}
		}},
		{name: "port", env: map[string]string{"PORT": "9000"}, check: func(t *testing.T, c Config) {
			if c.Addr != "0.0.0.0:9000" {
				t.Fatalf("addr %s", c.Addr)
			}
		}},
		{name: "addr overrides port", env: map[string]string{"PORT": "9000", "ADDR": "127.0.0.1:7000"}, check: func(t *testing.T, c Config) {
			if c.Addr != "127.0.0.1:7000" {
				t.Fatalf("addr %s", c.Addr)
			}
		}},
		{name: "bad port", env: map[string]string{"PORT": "abc"}, wantErr: "PORT"},
		{name: "real provider refused", env: map[string]string{"PROVIDER": "real"}, wantErr: "not supported"},
		{name: "unknown provider", env: map[string]string{"PROVIDER": "x"}, wantErr: "unknown PROVIDER"},
		{name: "bad otp", env: map[string]string{"DEV_OTP": "12ab56"}, wantErr: "DEV_OTP"},
		{name: "bad trust proxy", env: map[string]string{"TRUST_PROXY": "maybe"}, wantErr: "TRUST_PROXY"},
		{name: "origins", env: map[string]string{"ALLOWED_ORIGIN": " https://a.test/ , http://localhost:5173,,"}, check: func(t *testing.T, c Config) {
			if len(c.AllowedOrigins) != 2 || c.AllowedOrigins[0] != "https://a.test" || c.AllowAnyOrigin {
				t.Fatalf("origins %v", c.AllowedOrigins)
			}
		}},
		{name: "star origin", env: map[string]string{"ALLOWED_ORIGIN": "*"}, check: func(t *testing.T, c Config) {
			if !c.AllowAnyOrigin {
				t.Fatal("expected any origin")
			}
		}},
		{name: "version env wins", env: map[string]string{"VERSION": "0.2.0"}, check: func(t *testing.T, c Config) {
			if c.Version != "0.2.0" {
				t.Fatal(c.Version)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Load(env(tt.env), "build1")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, c)
		})
	}
}

func TestSummaryHidesSecrets(t *testing.T) {
	c, err := Load(env(map[string]string{"APP_SECRET": "super-secret-value", "DEV_OTP": "654321"}), "")
	if err != nil {
		t.Fatal(err)
	}
	s := c.Summary()
	if strings.Contains(s, "super-secret-value") || strings.Contains(s, "654321") {
		t.Fatalf("summary leaks a secret: %s", s)
	}
}
