package messages

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/auth"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpapi"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
	"github.com/Mrbaeksang/kch-relay/backend/internal/transactions"
	"github.com/go-chi/chi/v5"
)

const (
	seller = "dev-employee-a"
	buyer  = "dev-employee-b"
	other  = "dev-admin" // a third employee who is neither party of the conversation
	origin = "http://relay.test"
)

type harness struct {
	t       *testing.T
	f       *testkit.Fixture
	server  *httptest.Server
	clients map[string]*client
	clock   time.Time // the service reads this; tests move it forward
}

type client struct {
	http *http.Client
	csrf string
}

// newHarness assembles the same stack as application.New — real development
// sessions, CSRF, the authenticated module group — over an isolated SQLite
// file. The deal module is mounted too, so tests can use the real #18 paths.
func newHarness(t *testing.T) *harness {
	t.Helper()
	f := testkit.New(t)
	h := &harness{t: t, f: f, clients: map[string]*client{}, clock: f.Now}
	c := config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: origin, SessionTTL: 8 * time.Hour}
	authentication, err := auth.New(f.DB, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	meta := func(ctx context.Context, user string) (httpx.Meta, error) {
		return database.UnreadMeta(ctx, f.DB, user)
	}
	deps := application.Dependencies{DB: f.DB, Config: c, Meta: meta}
	handler := httpapi.NewRouter(f.DB, fstest.MapFS{}, func(r chi.Router) {
		r.Route("/api/v1", func(api chi.Router) {
			api.Use(authentication.Authenticate)
			authentication.Register(api)
			api.Group(func(protected chi.Router) {
				protected.Use(httpx.RequireUserMiddleware)
				transactions.Module(nil)(protected, deps)
				newService(f.DB, meta, func() time.Time { return h.clock }).register(protected)
			})
		})
	})
	h.server = httptest.NewServer(handler)
	t.Cleanup(h.server.Close)
	for _, key := range []string{seller, buyer, other} {
		jar, _ := cookiejar.New(nil)
		h.clients[key] = &client{http: &http.Client{Jar: jar}}
		var session struct {
			CSRFToken string `json:"csrfToken"`
		}
		if status, code := h.call(key, "POST", "/auth/development-login", map[string]string{"accountKey": key}, &session); status != 200 {
			t.Fatalf("login %s: %d %s", key, status, code)
		}
		h.clients[key].csrf = session.CSRFToken
	}
	return h
}

// tick moves the service clock forward.
func (h *harness) tick(d time.Duration) { h.clock = h.clock.Add(d) }

// callRaw sends a request as user with extra headers and returns the status
// and the raw response body.
func (h *harness) callRaw(user, method, path string, body any, headers map[string]string) (int, []byte) {
	h.t.Helper()
	var reader io.Reader = http.NoBody
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, h.server.URL+"/api/v1"+path, reader)
	req.Header.Set("Origin", origin)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c := h.clients[user]
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := c.http.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Meta  *httpx.Meta     `json:"meta"`
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
}

// send performs a request with headers, decodes data into out, and returns
// the status, the error code (empty on success) and the response meta.
func (h *harness) send(user, method, path string, body any, headers map[string]string, out any) (int, string, *httpx.Meta) {
	h.t.Helper()
	status, raw := h.callRaw(user, method, path, body, headers)
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		h.t.Fatalf("%s %s: invalid JSON %q: %v", method, path, raw, err)
	}
	if e.Error != nil {
		return status, e.Error.Code, e.Meta
	}
	if out != nil {
		if err := json.Unmarshal(e.Data, out); err != nil {
			h.t.Fatalf("%s %s: decode data: %v", method, path, err)
		}
	}
	return status, "", e.Meta
}

func (h *harness) call(user, method, path string, body any, out any) (int, string) {
	h.t.Helper()
	status, code, _ := h.send(user, method, path, body, nil, out)
	return status, code
}

// must performs a call that is expected to succeed with the given status.
func (h *harness) must(status int, user, method, path string, body any, out any) {
	h.t.Helper()
	if got, code := h.call(user, method, path, body, out); got != status {
		h.t.Fatalf("%s %s as %s: status %d (%s), want %d", method, path, user, got, code, status)
	}
}

// expect checks a call's status and error code.
func (h *harness) expect(status int, code, user, method, path string, body any) {
	h.t.Helper()
	if got, gotCode := h.call(user, method, path, body, nil); got != status || gotCode != code {
		h.t.Fatalf("%s %s as %s: %d %q, want %d %q", method, path, user, got, gotCode, status, code)
	}
}

// startConversation opens (or reuses) the buyer's conversation on a product.
func (h *harness) startConversation(user, product string) string {
	h.t.Helper()
	var out struct {
		ConversationID string `json:"conversationId"`
	}
	if status, code := h.call(user, "POST", "/products/"+product+"/conversations", nil, &out); status != 201 && status != 200 {
		h.t.Fatalf("start conversation: %d %s", status, code)
	}
	return out.ConversationID
}

func (h *harness) exec(query string, args ...any) {
	h.t.Helper()
	if _, err := h.f.DB.Exec(query, args...); err != nil {
		h.t.Fatal(err)
	}
}
