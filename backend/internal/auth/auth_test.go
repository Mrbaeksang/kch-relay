package auth_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/auth"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpapi"
	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
	"github.com/go-chi/chi/v5"
	"testing/fstest"
)

func TestDevelopmentSessionAndAdminBoundary(t *testing.T) {
	f := testkit.New(t)
	var clock atomic.Int64
	clock.Store(time.Now().UnixMilli())
	c := config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: "http://localhost:5173", SessionTTL: 8 * time.Hour}
	service, err := auth.New(f.DB, c, func() time.Time { return time.UnixMilli(clock.Load()) })
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewRouter(f.DB, fstest.MapFS{}, func(r chi.Router) {
		r.Route("/api/v1", func(api chi.Router) { api.Use(service.Authenticate); service.Register(api) })
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	request := func(method, path, body, csrf string) (int, map[string]any) {
		t.Helper()
		r, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewBufferString(body))
		r.Header.Set("Origin", c.AppOrigin)
		r.Header.Set("Content-Type", "application/json")
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(response.Body)
		var result map[string]any
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatalf("%s %s: %s", method, path, raw)
		}
		return response.StatusCode, result
	}
	if status, _ := request("GET", "/auth/me", "", ""); status != 401 {
		t.Fatal(status)
	}
	status, body := request("POST", "/auth/development-login", `{"accountKey":"dev-admin"}`, "")
	if status != 200 {
		t.Fatalf("login=%d %+v", status, body)
	}
	data := body["data"].(map[string]any)
	csrf := data["csrfToken"].(string)
	if status, _ := request("GET", "/admin/access", "", ""); status != 200 {
		t.Fatal(status)
	}
	if status, _ := request("POST", "/auth/logout", `{}`, "wrong"); status != 403 {
		t.Fatal("missing CSRF enforcement", status)
	}
	if status, _ := request("POST", "/auth/logout", `{}`, csrf); status != 200 {
		t.Fatal(status)
	}
	if status, _ := request("GET", "/auth/me", "", ""); status != 401 {
		t.Fatal(status)
	}
	if status, _ := request("POST", "/auth/development-login", `{"accountKey":"dev-employee-a"}`, ""); status != 200 {
		t.Fatal(status)
	}
	if status, _ := request("GET", "/admin/access", "", ""); status != 403 {
		t.Fatal(status)
	}
	clock.Add(int64(8 * time.Hour / time.Millisecond))
	if status, _ := request("GET", "/auth/me", "", ""); status != 401 {
		t.Fatal("expired session accepted", status)
	}
}
