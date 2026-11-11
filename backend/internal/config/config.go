package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv, AuthMode, Port, DatabasePath, UploadDir, WebDir, AppOrigin string
	GoogleClientID, GoogleClientSecret, GoogleRedirectURI              string
	SessionTTL                                                         time.Duration
}

func LoadConfig() (Config, error) {
	get := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}
	c := Config{AppEnv: get("APP_ENV", "production"), AuthMode: get("AUTH_MODE", "disabled"), Port: get("PORT", "8080"), DatabasePath: get("DATABASE_PATH", "./data/market.db"), UploadDir: get("UPLOAD_DIR", "./data/uploads"), WebDir: get("WEB_DIR", "../frontend/dist")}
	if c.AppEnv != "development" && c.AppEnv != "production" {
		return c, fmt.Errorf("APP_ENV must be development or production")
	}
	if c.AuthMode != "disabled" && c.AuthMode != "development" && c.AuthMode != "google" {
		return c, fmt.Errorf("AUTH_MODE is unsupported")
	}
	if c.AppEnv == "production" && c.AuthMode == "development" {
		return c, fmt.Errorf("development authentication is forbidden in production")
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("PORT must be between 1 and 65535")
	}
	c.SessionTTL, err = time.ParseDuration(get("SESSION_TTL", "8h"))
	if err != nil || c.SessionTTL <= 0 {
		return c, fmt.Errorf("SESSION_TTL must be a positive duration")
	}
	originDefault := ""
	if c.AppEnv == "development" {
		originDefault = "http://localhost:5173"
	}
	c.AppOrigin = get("APP_ORIGIN", originDefault)
	if c.AppOrigin != "" {
		u, err := url.Parse(c.AppOrigin)
		if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") || (c.AppEnv == "production" && u.Scheme != "https") {
			return c, fmt.Errorf("APP_ORIGIN must be an origin, with HTTPS in production")
		}
	}

	c.GoogleClientID = get("GOOGLE_CLIENT_ID", "")
	c.GoogleClientSecret = get("GOOGLE_CLIENT_SECRET", "")
	c.GoogleRedirectURI = get("GOOGLE_REDIRECT_URI", "")
	if c.AuthMode == "google" && (c.GoogleClientID == "" || c.GoogleClientSecret == "" || c.AppOrigin == "" || c.GoogleRedirectURI != c.AppOrigin+"/api/v1/auth/google/callback") {
		return c, fmt.Errorf("Google credentials and matching GOOGLE_REDIRECT_URI / APP_ORIGIN are required")
	}
	return c, nil
}
