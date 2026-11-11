package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
	"time"
)

const flowCookie = "relay_google_flow"

type googleIdentity struct{ Subject, Email string }
type googleAuth struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	exchange func(context.Context, string, string, string) (googleIdentity, error)
}

func newGoogleAuth(c config.Config) *googleAuth {
	client := &http.Client{Timeout: 10 * time.Second}
	ctx := oidc.ClientContext(context.Background(), client)
	g := &googleAuth{oauth: oauth2.Config{ClientID: c.GoogleClientID, ClientSecret: c.GoogleClientSecret, RedirectURL: c.GoogleRedirectURI, Scopes: []string{oidc.ScopeOpenID, "email"}, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", AuthStyle: oauth2.AuthStyleInParams}}, verifier: oidc.NewVerifier("https://accounts.google.com", oidc.NewRemoteKeySet(ctx, "https://www.googleapis.com/oauth2/v3/certs"), &oidc.Config{ClientID: c.GoogleClientID})}
	g.exchange = func(ctx context.Context, code, verifier, nonce string) (googleIdentity, error) {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
		token, err := g.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			return googleIdentity{}, errors.New("Google token exchange failed")
		}
		raw, _ := token.Extra("id_token").(string)
		id, err := g.verifier.Verify(ctx, raw)
		if err != nil {
			return googleIdentity{}, errors.New("Google identity verification failed")
		}
		var claims struct {
			Email    string `json:"email"`
			Verified bool   `json:"email_verified"`
		}
		if err := id.Claims(&claims); err != nil || !claims.Verified || !strings.Contains(claims.Email, "@") || id.Subject == "" || subtle.ConstantTimeCompare([]byte(id.Nonce), []byte(nonce)) != 1 {
			return googleIdentity{}, errors.New("Google identity claims invalid")
		}
		return googleIdentity{Subject: id.Subject, Email: strings.ToLower(claims.Email)}, nil
	}
	return g
}
func (s *Service) flowCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: flowCookie, Value: value, Path: "/api/v1/auth/google", MaxAge: maxAge, HttpOnly: true, Secure: s.config.AppEnv == "production", SameSite: http.SameSiteLaxMode}
}
func (s *Service) googleStart(w http.ResponseWriter, r *http.Request) {
	state, binding, nonce, verifier := httpx.NewID(), httpx.NewID(), httpx.NewID(), oauth2.GenerateVerifier()
	err := database.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM oauth_flows WHERE expires_at<=?`, database.Timestamp(s.now())); err != nil {
			return err
		}
		if old, err := r.Cookie(flowCookie); err == nil {
			if _, err := tx.ExecContext(r.Context(), `DELETE FROM oauth_flows WHERE binding_hash=?`, tokenHash(old.Value)); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(r.Context(), `INSERT INTO oauth_flows(state_hash,binding_hash,nonce,verifier,expires_at) VALUES(?,?,?,?,?)`, tokenHash(state), tokenHash(binding), nonce, verifier, database.Timestamp(s.now().Add(10*time.Minute)))
		return err
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	http.SetCookie(w, s.flowCookie(binding, 600))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, s.google.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("prompt", "select_account")), http.StatusFound)
}
func (s *Service) googleCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	fail := func() {
		http.SetCookie(w, s.flowCookie("", -1))
		http.Redirect(w, r, s.config.AppOrigin+"/?loginError=google", http.StatusSeeOther)
	}
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie(flowCookie)
	if err != nil || len(state) != 64 || len(cookie.Value) != 64 {
		fail()
		return
	}
	var nonce, verifier string
	err = s.db.QueryRowContext(r.Context(), `DELETE FROM oauth_flows WHERE state_hash=? AND binding_hash=? AND expires_at>? RETURNING nonce,verifier`, tokenHash(state), tokenHash(cookie.Value), database.Timestamp(s.now())).Scan(&nonce, &verifier)
	if err != nil {
		fail()
		return
	}
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		fail()
		return
	}
	identity, err := s.google.exchange(r.Context(), r.URL.Query().Get("code"), verifier, nonce)
	if err != nil {
		fail()
		return
	}
	raw, csrf := httpx.NewID(), httpx.NewID()
	now := s.now()
	err = database.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var id string
		err := tx.QueryRowContext(r.Context(), `SELECT id FROM users WHERE google_subject=?`, identity.Subject).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			id = httpx.NewID()
			_, err = tx.ExecContext(r.Context(), `INSERT INTO users(id,google_subject,email,real_name,department,job_level,membership_status) VALUES(?,?,?,'','','','PROFILE_REQUIRED')`, id, identity.Subject, identity.Email)
		} else if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE users SET email=?,updated_at=? WHERE id=?`, identity.Email, database.Timestamp(now), id)
		}
		if err != nil {
			return err
		}
		if old, err := r.Cookie(CookieName); err == nil {
			if _, err := tx.ExecContext(r.Context(), `UPDATE sessions SET revoked_at=? WHERE token_hash=? AND revoked_at IS NULL`, database.Timestamp(now), tokenHash(old.Value)); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO sessions(id,user_id,token_hash,csrf_token,created_at,expires_at) VALUES(?,?,?,?,?,?)`, httpx.NewID(), id, tokenHash(raw), csrf, database.Timestamp(now), database.Timestamp(now.Add(s.config.SessionTTL)))
		return err
	})
	if err != nil {
		fail()
		return
	}
	http.SetCookie(w, s.flowCookie("", -1))
	http.SetCookie(w, s.cookie(raw, int(s.config.SessionTTL.Seconds()), now.Add(s.config.SessionTTL)))
	http.Redirect(w, r, s.config.AppOrigin+"/", http.StatusSeeOther)
}
