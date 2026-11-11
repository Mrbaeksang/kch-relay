package transactions

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	"github.com/go-chi/chi/v5"
)

const (
	seller = "dev-employee-a"
	buyer  = "dev-employee-b"
	other  = "dev-admin" // a third employee who is neither party of the deal
	origin = "http://relay.test"
)

type harness struct {
	t       *testing.T
	f       *testkit.Fixture
	server  *httptest.Server
	clients map[string]*client
}

type client struct {
	http *http.Client
	csrf string
}

// newHarness assembles the same stack as application.New — real development
// sessions, CSRF, the authenticated module group — over an isolated SQLite file.
func newHarness(t *testing.T, attach ImageAttacher) *harness {
	t.Helper()
	f := testkit.New(t)
	c := config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: origin, SessionTTL: 8 * time.Hour}
	authentication, err := auth.New(f.DB, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	deps := application.Dependencies{DB: f.DB, Config: c, Meta: func(ctx context.Context, user string) (httpx.Meta, error) {
		return database.UnreadMeta(ctx, f.DB, user)
	}}
	handler := httpapi.NewRouter(f.DB, fstest.MapFS{}, func(r chi.Router) {
		r.Route("/api/v1", func(api chi.Router) {
			api.Use(authentication.Authenticate)
			authentication.Register(api)
			api.Group(func(protected chi.Router) {
				protected.Use(httpx.RequireUserMiddleware)
				Module(attach)(protected, deps)
			})
		})
	})
	h := &harness{t: t, f: f, server: httptest.NewServer(handler), clients: map[string]*client{}}
	t.Cleanup(h.server.Close)
	for _, key := range []string{seller, buyer, other} {
		jar, _ := cookiejar.New(nil)
		c := &client{http: &http.Client{Jar: jar}}
		h.clients[key] = c
		var session struct {
			CSRFToken string `json:"csrfToken"`
		}
		if status, code := h.call(key, "POST", "/auth/development-login", map[string]string{"accountKey": key}, &session); status != 200 {
			t.Fatalf("login %s: %d %s", key, status, code)
		}
		c.csrf = session.CSRFToken
	}
	return h
}

// call sends a request as user and decodes data into out. It returns the HTTP
// status and the error code (empty on success).
func (h *harness) call(user, method, path string, body any, out any) (int, string) {
	h.t.Helper()
	status, code, _ := h.callDetails(user, method, path, body, out)
	return status, code
}

func (h *harness) callDetails(user, method, path string, body any, out any) (int, string, map[string]any) {
	h.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
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
	res, err := c.http.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		h.t.Fatalf("%s %s: invalid JSON: %v", method, path, err)
	}
	if envelope.Error != nil {
		return res.StatusCode, envelope.Error.Code, envelope.Error.Details
	}
	if out != nil {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			h.t.Fatalf("%s %s: decode data: %v", method, path, err)
		}
	}
	return res.StatusCode, "", nil
}

// must performs a call that is expected to succeed with the given status.
func (h *harness) must(status int, user, method, path string, body any, out any) {
	h.t.Helper()
	if got, code := h.call(user, method, path, body, out); got != status {
		h.t.Fatalf("%s %s as %s: status %d (%s), want %d", method, path, user, got, code, status)
	}
}

func (h *harness) expect(status int, code, user, method, path string, body any) {
	h.t.Helper()
	got, gotCode := h.call(user, method, path, body, nil)
	if got != status || gotCode != code {
		h.t.Fatalf("%s %s as %s: got %d %q, want %d %q", method, path, user, got, gotCode, status, code)
	}
}

func (h *harness) apply(user, product string, quantity int64) dealOut {
	h.t.Helper()
	var d dealOut
	h.must(201, user, "POST", "/products/"+product+"/deals", map[string]int64{"quantity": quantity}, &d)
	return d
}

func (h *harness) scalar(query string, args ...any) int64 {
	h.t.Helper()
	var n sql.NullInt64
	if err := h.f.DB.QueryRow(query, args...).Scan(&n); err != nil {
		h.t.Fatalf("%s: %v", query, err)
	}
	return n.Int64
}

func (h *harness) text(query string, args ...any) string {
	h.t.Helper()
	var s sql.NullString
	if err := h.f.DB.QueryRow(query, args...).Scan(&s); err != nil {
		h.t.Fatalf("%s: %v", query, err)
	}
	return s.String
}

func (h *harness) remaining(product string) int64 {
	return h.scalar("SELECT remaining_quantity FROM products WHERE id=?", product)
}

func (h *harness) dealStatus(deal string) string {
	return h.text("SELECT status FROM deals WHERE id=?", deal)
}

func (h *harness) notifications(deal string) int64 {
	return h.scalar("SELECT count(*) FROM notifications WHERE deal_id=?", deal)
}

type identityOut struct {
	RealName   string `json:"realName"`
	Department string `json:"department"`
	JobLevel   string `json:"jobLevel"`
}

type dealOut struct {
	ID                string  `json:"id"`
	ProductID         string  `json:"productId"`
	ConversationID    string  `json:"conversationId"`
	Status            string  `json:"status"`
	ViewerRole        string  `json:"viewerRole"`
	RequestedQuantity int64   `json:"requestedQuantity"`
	AgreedUnitPrice   *int64  `json:"agreedUnitPrice"`
	AgreedQuantity    *int64  `json:"agreedQuantity"`
	TotalPrice        *int64  `json:"totalPrice"`
	RequestedAt       string  `json:"requestedAt"`
	QuantityUpdatedAt *string `json:"quantityUpdatedAt"`
	AcceptedAt        *string `json:"acceptedAt"`
	CompletedAt       *string `json:"completedAt"`
	Product           struct {
		ID                string `json:"id"`
		Title             string `json:"title"`
		UnitPrice         int64  `json:"unitPrice"`
		RemainingQuantity int64  `json:"remainingQuantity"`
		Hidden            bool   `json:"hidden"`
	} `json:"product"`
	Counterparty struct {
		Label    string       `json:"label"`
		Identity *identityOut `json:"identity"`
	} `json:"counterparty"`
	Actions []string `json:"actions"`
}

type productOut struct {
	ID                      string   `json:"id"`
	Title                   string   `json:"title"`
	Description             string   `json:"description"`
	UnitPrice               int64    `json:"unitPrice"`
	RemainingQuantity       int64    `json:"remainingQuantity"`
	RecruitmentClosed       bool     `json:"recruitmentClosed"`
	ApprovalStatus          string   `json:"approvalStatus"`
	Hidden                  bool     `json:"hidden"`
	ImageIDs                []string `json:"imageIds"`
	PendingApplicationCount int64    `json:"pendingApplicationCount"`
}

type dealPage struct {
	Items      []dealOut `json:"items"`
	NextCursor *string   `json:"nextCursor"`
}

// image inserts a real images row owned by uploader, as the photo feature would.
func (h *harness) image(uploader string) string {
	h.t.Helper()
	id := httpx.NewID()
	_, err := h.f.DB.Exec(`INSERT INTO images(id,uploader_id,storage_key,created_at) VALUES(?,?,?,?)`, id, uploader, "test/"+id, database.Timestamp(h.f.Now))
	if err != nil {
		h.t.Fatal(err)
	}
	return id
}

// testAttachImages is a test-only stand-in for the AttachImagesTx contract of
// the product registration feature: it checks ownership and replaces links.
func testAttachImages(ctx context.Context, tx *sql.Tx, productID, sellerID string, imageIDs []string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, "UPDATE images SET product_id=NULL WHERE product_id=?", productID); err != nil {
		return err
	}
	for i, id := range imageIDs {
		res, err := tx.ExecContext(ctx, "UPDATE images SET product_id=?,sort_order=? WHERE id=? AND uploader_id=? AND product_id IS NULL", productID, i, id, sellerID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return httpx.Invalid(fmt.Sprintf("사진 %d을 사용할 수 없습니다.", i+1))
		}
	}
	return nil
}
