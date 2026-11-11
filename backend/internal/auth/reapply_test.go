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

func TestRejectedMembersCanReapplyAndAdminCanReopenOrApprove(t *testing.T) {
	f := testkit.New(t)
	s, err := New(f.DB, config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: "http://localhost:5173"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) { r.Use(s.Authenticate); s.Register(r) })
	for _, id := range []string{"reapply", "reopen", "approve"} {
		_, err := f.DB.Exec(`INSERT INTO users(id,google_subject,email,real_name,department,department_id,job_level,membership_status) VALUES(?,?,?,'가상 직원','인사팀','hr','사원','REJECTED')`, id, "google-"+id, id+"@example.org")
		if err != nil {
			t.Fatal(err)
		}
	}
	raw := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err = f.DB.Exec(`INSERT INTO sessions(id,user_id,token_hash,csrf_token,created_at,expires_at) VALUES('reapply-session','reapply',?,'csrf','2026-01-01T00:00:00Z','2099-01-01T00:00:00Z')`, tokenHash(raw))
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: CookieName, Value: raw}
	csrf := "csrf"
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("X-CSRF-Token", csrf)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	body := `{"realName":"수정한 이름","departmentId":"hr","jobLevel":"대리"}`
	if w := call("POST", "/auth/register", body); w.Code != 200 {
		t.Fatalf("reapply %d %s", w.Code, w.Body)
	}
	if w := call("POST", "/auth/register", body); w.Code != 409 {
		t.Fatalf("duplicate %d", w.Code)
	}
	if w := call("GET", "/access", ""); w.Code != 403 {
		t.Fatalf("reapplication bypassed approval %d", w.Code)
	}
	if w := call("POST", "/admin/members/reopen/reopen", `{}`); w.Code != 403 {
		t.Fatalf("pending self approval %d", w.Code)
	}
	cookie = nil
	csrf = ""
	w := call("POST", "/auth/development-login", `{"accountKey":"dev-admin"}`)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	cookie = w.Result().Cookies()[0]
	var e struct {
		Data struct {
			CSRF string `json:"csrfToken"`
		}
	}
	json.Unmarshal(w.Body.Bytes(), &e)
	csrf = e.Data.CSRF
	if w := call("POST", "/admin/members/reopen/reopen", `{}`); w.Code != 200 {
		t.Fatalf("reopen %d %s", w.Code, w.Body)
	}
	if w := call("POST", "/admin/members/reopen/reopen", `{}`); w.Code != 409 {
		t.Fatalf("duplicate reopen %d", w.Code)
	}
	var status, role string
	f.DB.QueryRow(`SELECT membership_status,role FROM users WHERE id='reopen'`).Scan(&status, &role)
	if status != "PENDING" || role != "MEMBER" {
		t.Fatal(status, role)
	}
	if w := call("POST", "/admin/members/approve/approve", `{}`); w.Code != 200 {
		t.Fatalf("approve rejected %d %s", w.Code, w.Body)
	}
	f.DB.QueryRow(`SELECT membership_status,role FROM users WHERE id='approve'`).Scan(&status, &role)
	if status != "ACTIVE" || role != "HR_ADMIN" {
		t.Fatal(status, role)
	}
	f.DB.Exec(`UPDATE users SET status='SUSPENDED' WHERE id='reopen'`)
	if w := call("POST", "/admin/members/reopen/approve", `{}`); w.Code != 409 {
		t.Fatalf("approval bypassed suspension %d", w.Code)
	}
	csrf = "wrong"
	if w := call("POST", "/admin/members/reapply/approve", `{}`); w.Code != 403 {
		t.Fatalf("CSRF bypassed %d", w.Code)
	}
}
