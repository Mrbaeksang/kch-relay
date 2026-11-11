package auth

import (
	"bytes"
	"encoding/json"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOnboardingApprovalAndDepartmentBoundary(t *testing.T) {
	f := testkit.New(t)
	s, err := New(f.DB, config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: "http://localhost:5173"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) { r.Use(s.Authenticate); s.Register(r) })
	var cookie *http.Cookie
	csrf := ""
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewBufferString(body))
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	// An identity verified by Google has a session, but cannot enter the market.
	_, err = f.DB.Exec(`INSERT INTO users(id,google_subject,email,real_name,department,job_level,membership_status) VALUES('new','google-new','new@example.org','','','','PROFILE_REQUIRED')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.DB.Exec(`INSERT INTO sessions(id,user_id,token_hash,csrf_token,created_at,expires_at) VALUES('new-session','new',?,'new-csrf','2026-01-01T00:00:00Z','2099-01-01T00:00:00Z')`, tokenHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	if err != nil {
		t.Fatal(err)
	}
	cookie = &http.Cookie{Name: CookieName, Value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	csrf = "new-csrf"
	if w := call("GET", "/access", ""); w.Code != 403 {
		t.Fatalf("pending market access=%d %s", w.Code, w.Body)
	}
	if w := call("POST", "/auth/register", `{"realName":"가상 직원","departmentId":"hr","jobLevel":"사원"}`); w.Code != 200 {
		t.Fatalf("register=%d %s", w.Code, w.Body)
	}
	if w := call("GET", "/admin/members", ""); w.Code != 403 {
		t.Fatalf("pending HR became admin %d", w.Code)
	}
	// Pending submissions cannot be overwritten to bypass review.
	if w := call("POST", "/auth/register", `{"realName":"변경","departmentId":"hr","jobLevel":"사원"}`); w.Code != 409 {
		t.Fatalf("duplicate registration=%d", w.Code)
	}
	cookie = nil
	csrf = ""
	w := call("POST", "/auth/development-login", `{"accountKey":"dev-admin"}`)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	cookie = w.Result().Cookies()[0]
	var envelope struct {
		Data struct {
			CSRF string `json:"csrfToken"`
		}
	}
	json.Unmarshal(w.Body.Bytes(), &envelope)
	csrf = envelope.Data.CSRF
	if w := call("POST", "/admin/members/new/approve", `{}`); w.Code != 200 {
		t.Fatalf("approve=%d %s", w.Code, w.Body)
	}
	var role, status string
	f.DB.QueryRow(`SELECT role,membership_status FROM users WHERE id='new'`).Scan(&role, &status)
	if role != "HR_ADMIN" || status != "ACTIVE" {
		t.Fatalf("%s %s", role, status)
	}
	if w := call("POST", "/admin/departments", `{"name":"개발팀"}`); w.Code != 201 {
		t.Fatalf("create=%d %s", w.Code, w.Body)
	}
	if w := call("PATCH", "/admin/departments/hr", `{"name":"피플팀"}`); w.Code != 200 {
		t.Fatalf("rename=%d %s", w.Code, w.Body)
	}
	if w := call("DELETE", "/admin/departments/hr", ""); w.Code != 409 {
		t.Fatalf("deleted HR=%d", w.Code)
	}
	if w := call("POST", "/admin/members/new/suspend", `{}`); w.Code != 200 {
		t.Fatalf("suspend=%d %s", w.Code, w.Body)
	}
	cookie = &http.Cookie{Name: CookieName, Value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	csrf = "new-csrf"
	if w := call("GET", "/access", ""); w.Code != 403 {
		t.Fatalf("suspended access=%d", w.Code)
	}
}
