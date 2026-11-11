package config

import (
	"testing"
	"time"
)

func clean(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_ENV", "AUTH_MODE", "PORT", "DATABASE_PATH", "UPLOAD_DIR", "WEB_DIR", "APP_ORIGIN", "SESSION_TTL", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URI"} {
		t.Setenv(key, "")
	}
}

func TestSafeDefaultsAndExplicitDevelopment(t *testing.T) {
	clean(t)
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.AppEnv != "production" || c.AuthMode != "disabled" || c.Port != "8080" || c.SessionTTL != 8*time.Hour {
		t.Fatalf("unsafe defaults: %+v", c)
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("AUTH_MODE", "development")
	c, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.AppOrigin != "http://localhost:5173" {
		t.Fatalf("origin=%s", c.AppOrigin)
	}
}

func TestInvalidSettingsFailBeforeStartup(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"APP_ENV", "other"}, {"AUTH_MODE", "development"}, {"AUTH_MODE", "google"}, {"PORT", "0"}, {"PORT", "65536"}, {"PORT", "abc"}, {"SESSION_TTL", "0s"}, {"SESSION_TTL", "-1h"}, {"SESSION_TTL", "soon"}, {"APP_ORIGIN", "http://example.com"}, {"APP_ORIGIN", "https://example.com/path"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			clean(t)
			t.Setenv(tc.key, tc.value)
			if _, err := LoadConfig(); err == nil {
				t.Fatal("invalid setting accepted")
			}
		})
	}
}

func TestGoogleConfigRequiresExactCallback(t *testing.T) {
	clean(t)
	t.Setenv("AUTH_MODE", "google")
	t.Setenv("APP_ORIGIN", "https://market.example.org")
	t.Setenv("GOOGLE_CLIENT_ID", "client")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REDIRECT_URI", "https://market.example.org/api/v1/auth/google/callback")
	if _, err := LoadConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_REDIRECT_URI", "https://other.example.org/api/v1/auth/google/callback")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("mismatched callback accepted")
	}
}
