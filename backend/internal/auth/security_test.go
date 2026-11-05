package auth_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/auth"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpapi"
	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
	"github.com/go-chi/chi/v5"
)

func TestForgeryRevocationSuspensionAndUnreadIsolation(t *testing.T) {
	f := testkit.New(t)
	origin := "http://localhost:5173"
	service, err := auth.New(f.DB, config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: origin, SessionTTL: time.Hour}, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.NewRouter(f.DB, fstest.MapFS{}, func(r chi.Router) {
		r.Route("/api/v1", func(api chi.Router) { api.Use(service.Authenticate); service.Register(api) })
	}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	call := func(method, path, body, requestOrigin, cookie string) (int, map[string]any, []*http.Cookie) {
		t.Helper()
		r, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", requestOrigin)
		r.Header.Set("X-User-ID", "dev-admin")
		caller := client
		if cookie != "" {
			r.Header.Set("Cookie", cookie)
			caller = &http.Client{}
		}
		response, err := caller.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(response.Body)
		var out map[string]any
		if json.Unmarshal(raw, &out) != nil {
			t.Fatalf("invalid response %s", raw)
		}
		return response.StatusCode, out, response.Cookies()
	}
	for _, body := range []string{`{"accountKey":"arbitrary"}`, `{"accountKey":"dev-admin","role":"HR_ADMIN"}`, `{"userId":"dev-admin"}`} {
		if code, _, _ := call("POST", "/auth/development-login", body, origin, ""); code != 400 {
			t.Fatal("forgery accepted", code)
		}
	}
	if code, _, _ := call("POST", "/auth/development-login", `{"accountKey":"dev-admin"}`, "http://attacker.invalid", ""); code != 403 {
		t.Fatal("foreign login origin accepted", code)
	}
	code, _, cookies := call("POST", "/auth/development-login", `{"accountKey":"dev-admin"}`, origin, "")
	if code != 200 || len(cookies) != 1 {
		t.Fatal(code, cookies)
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.Path != "/api" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unsafe cookie: %+v", cookie)
	}
	var stored string
	if err := f.DB.QueryRow("SELECT token_hash FROM sessions LIMIT 1").Scan(&stored); err != nil || stored == cookie.Value || len(stored) != 64 {
		t.Fatal("raw token persisted", err)
	}
	oldCookie := cookie.Name + "=" + cookie.Value
	if code, _, _ := call("POST", "/auth/development-login", `{"accountKey":"dev-employee-a"}`, origin, ""); code != 200 {
		t.Fatal(code)
	}
	if code, _, _ := call("GET", "/auth/me", "", origin, oldCookie); code != 401 {
		t.Fatal("old session survived account switch", code)
	}
	if code, _, _ := call("GET", "/admin/access", "", origin, ""); code != 403 {
		t.Fatal("header role spoof worked", code)
	}
	if _, err := f.DB.Exec("UPDATE users SET status='SUSPENDED' WHERE id='dev-employee-a'"); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := call("GET", "/auth/me", "", origin, ""); code != 403 {
		t.Fatal("suspended user accepted", code)
	}
	if _, err := f.DB.Exec("UPDATE users SET status='ACTIVE' WHERE id='dev-employee-a'"); err != nil {
		t.Fatal(err)
	}
	product := f.Product("dev-admin", "APPROVED", 10000, 3)
	p := f.Participant(product, "dev-employee-a", "inquiry")
	f.Message(p.ID, "dev-admin", "dev-employee-a", false)
	f.Notification(f.Deal(p.ID, "REJECTED", 1, 0), "dev-employee-a")
	code, out, _ := call("GET", "/auth/me", "", origin, "")
	if code != 200 {
		t.Fatal(code)
	}
	meta := out["meta"].(map[string]any)
	if meta["hasUnreadMessages"] != true || meta["hasUnreadNotifications"] != true {
		t.Fatal(meta)
	}
	code, out, _ = call("POST", "/auth/development-login", `{"accountKey":"dev-employee-b"}`, origin, "")
	if code != 200 {
		t.Fatal(code)
	}
	meta = out["meta"].(map[string]any)
	if meta["hasUnreadMessages"] != false || meta["hasUnreadNotifications"] != false {
		t.Fatal("another user's unread state leaked", meta)
	}
}

func TestDisabledModeHasNoDevelopmentLogin(t *testing.T) {
	f := testkit.New(t)
	service, err := auth.New(f.DB, config.Config{AppEnv: "production", AuthMode: "disabled"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.NewRouter(f.DB, fstest.MapFS{}, func(r chi.Router) {
		r.Route("/api/v1", func(api chi.Router) { api.Use(service.Authenticate); service.Register(api) })
	}))
	defer server.Close()
	for _, tc := range []struct {
		path string
		code int
	}{{"/auth/development-login", 404}, {"/auth/me", 401}, {"/admin/access", 401}} {
		r, _ := http.NewRequest("POST", server.URL+"/api/v1"+tc.path, bytes.NewBufferString(`{"accountKey":"dev-admin"}`))
		if tc.code != 404 {
			r.Method = "GET"
		}
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != tc.code {
			t.Fatalf("%s=%d", tc.path, response.StatusCode)
		}
	}
	if _, err := auth.New(f.DB, config.Config{AppEnv: "production", AuthMode: "development"}, nil); err == nil {
		t.Fatal("production dev auth accepted")
	}
}
