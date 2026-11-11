package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	jose "github.com/go-jose/go-jose/v4"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGoogleCodeFlowVerificationAndReplay(t *testing.T) {
	f := testkit.New(t)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	wrongKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	c := config.Config{AppEnv: "development", AuthMode: "google", AppOrigin: "http://localhost:5173", GoogleClientID: "test-client", GoogleClientSecret: "test-secret", GoogleRedirectURI: "http://localhost:5173/api/v1/auth/google/callback"}
	s, err := New(f.DB, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.google.verifier = oidc.NewVerifier("https://accounts.google.com", &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{ClientID: c.GoogleClientID})

	var nonce, verifier string
	mode := "valid"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("code_verifier") != verifier || r.Form.Get("client_secret") != "test-secret" || r.Form.Get("redirect_uri") != c.GoogleRedirectURI {
			t.Error("code exchange missing binding")
		}
		claims := map[string]any{"iss": "https://accounts.google.com", "aud": "test-client", "sub": "google-subject", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": nonce, "email": "employee@example.org", "email_verified": true}
		signingKey := key
		switch mode {
		case "audience":
			claims["aud"] = "other-client"
		case "nonce":
			claims["nonce"] = "wrong"
		case "expired":
			claims["exp"] = time.Now().Add(-time.Hour).Unix()
		case "unverified":
			claims["email_verified"] = false
		case "signature":
			signingKey = wrongKey
		case "issuer":
			claims["iss"] = "https://attacker.example"
		}
		signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: signingKey}, nil)
		payload, _ := json.Marshal(claims)
		signed, _ := signer.Sign(payload)
		jwt, _ := signed.CompactSerialize()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"access_token": "fake-token", "token_type": "Bearer", "id_token": jwt})
	}))
	defer provider.Close()
	s.google.oauth.Endpoint.TokenURL = provider.URL
	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) { r.Use(s.Authenticate); s.Register(r) })
	start := func() (string, *http.Cookie) {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/auth/google/start", nil))
		if w.Code != 302 {
			t.Fatal(w.Body)
		}
		location, _ := url.Parse(w.Header().Get("Location"))
		nonce = location.Query().Get("nonce")
		state := location.Query().Get("state")
		if location.Query().Get("code_challenge_method") != "S256" || nonce == "" {
			t.Fatal("Missing PKCE/nonce")
		}
		f.DB.QueryRow(`SELECT verifier FROM oauth_flows WHERE state_hash=?`, tokenHash(state)).Scan(&verifier)
		return state, w.Result().Cookies()[0]
	}
	callback := func(state string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=fake-code&state="+url.QueryEscape(state), nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	for _, m := range []string{"audience", "nonce", "expired", "unverified", "signature", "issuer"} {
		mode = m
		state, cookie := start()
		w := callback(state, cookie)
		if !strings.Contains(w.Header().Get("Location"), "loginError") {
			t.Fatalf("accepted invalid %s", m)
		}
	}
	mode = "valid"
	state, cookie := start()
	if w := callback(state, nil); !strings.Contains(w.Header().Get("Location"), "loginError") {
		t.Fatal("missing browser binding accepted")
	}
	if w := callback(state, &http.Cookie{Name: flowCookie, Value: strings.Repeat("b", 64)}); !strings.Contains(w.Header().Get("Location"), "loginError") {
		t.Fatal("wrong browser binding accepted")
	}
	w := callback(state, cookie)
	if w.Code != 303 || w.Header().Get("Location") != c.AppOrigin+"/" {
		t.Fatalf("callback %d %s", w.Code, w.Header().Get("Location"))
	}
	var sessionCookie *http.Cookie
	for _, v := range w.Result().Cookies() {
		if v.Name == CookieName {
			sessionCookie = v
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly {
		t.Fatal("missing protected session")
	}
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "PROFILE_REQUIRED") {
		t.Fatal(w.Body)
	}
	if w := callback(state, cookie); !strings.Contains(w.Header().Get("Location"), "loginError") {
		t.Fatal("replayed state accepted")
	}
	state, cookie = start()
	f.DB.Exec(`UPDATE oauth_flows SET expires_at='2000-01-01T00:00:00Z'`)
	if w := callback(state, cookie); !strings.Contains(w.Header().Get("Location"), "loginError") {
		t.Fatal("expired flow accepted")
	}
	state, cookie = start()
	if w := callback(state, cookie); strings.Contains(w.Header().Get("Location"), "loginError") {
		t.Fatal("returning login failed")
	}
	var count int
	f.DB.QueryRow(`SELECT count(*) FROM users WHERE google_subject='google-subject'`).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate user", count)
	}

}
