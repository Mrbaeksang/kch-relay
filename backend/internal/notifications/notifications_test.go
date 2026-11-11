package notifications_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/notifications"
	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
)

const origin = "http://localhost:5173"

const (
	employeeA = "dev-employee-a"
	employeeB = "dev-employee-b"
	admin     = "dev-admin"
)

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Meta  *meta           `json:"meta"`
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
}
type meta struct {
	HasUnreadMessages      bool `json:"hasUnreadMessages"`
	HasUnreadNotifications bool `json:"hasUnreadNotifications"`
}
type item struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	DealID    string  `json:"dealId"`
	ProductID string  `json:"productId"`
	CreatedAt string  `json:"createdAt"`
	ReadAt    *string `json:"readAt"`
}
type page struct {
	Items      []item  `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

// world는 실제 서버 조립(application.New)과 같은 SQLite 파일을 공유하는 fixture를 묶는다.
type world struct {
	t       *testing.T
	f       *testkit.Fixture
	server  *httptest.Server
	clock   time.Time
	clockMu sync.Mutex
}

func newWorld(t *testing.T) *world {
	t.Helper()
	f := testkit.New(t)
	var seq int
	var name, path string
	if err := f.DB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	w := &world{t: t, f: f, clock: time.Date(2026, 11, 6, 1, 0, 0, 0, time.UTC)}
	dir := t.TempDir()
	c := config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: origin, DatabasePath: path, UploadDir: filepath.Join(dir, "uploads"), WebDir: dir, Port: "0", SessionTTL: 8 * time.Hour}
	app, err := application.New(context.Background(), c, notifications.Module(w.now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Close() })
	w.server = httptest.NewServer(app.HTTP.Handler)
	t.Cleanup(w.server.Close)
	return w
}

func (w *world) now() time.Time {
	w.clockMu.Lock()
	defer w.clockMu.Unlock()
	return w.clock
}
func (w *world) advance(d time.Duration) {
	w.clockMu.Lock()
	w.clock = w.clock.Add(d)
	w.clockMu.Unlock()
}

// rejected는 seller의 상품에 buyer가 신청했다가 거절당한 거래와 그 알림을 만든다.
func (w *world) rejected(seller, buyer string) (notificationID, dealID, productID string) {
	w.t.Helper()
	productID = w.f.Product(seller, "APPROVED", 1000, 3)
	participant := w.f.Participant(productID, buyer, "application")
	dealID = w.f.Deal(participant.ID, "REJECTED", 1, 1000)
	notificationID = w.f.Notification(dealID, buyer)
	w.f.Now = w.f.Now.Add(time.Minute)
	return
}

type user struct {
	w      *world
	client *http.Client
	csrf   string
}

func (w *world) login(account string) *user {
	w.t.Helper()
	jar, _ := cookiejar.New(nil)
	u := &user{w: w, client: &http.Client{Jar: jar}}
	status, body := u.do("POST", "/auth/development-login", `{"accountKey":"`+account+`"}`)
	if status != 200 {
		w.t.Fatalf("login %s = %d", account, status)
	}
	var data struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(body.Data, &data); err != nil {
		w.t.Fatal(err)
	}
	u.csrf = data.CSRFToken
	return u
}
func (w *world) anonymous() *user {
	return &user{w: w, client: &http.Client{}}
}

func (u *user) do(method, path, body string) (int, envelope) {
	u.w.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	r, _ := http.NewRequest(method, u.w.server.URL+"/api/v1"+path, reader)
	r.Header.Set("Origin", origin)
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if u.csrf != "" {
		r.Header.Set("X-CSRF-Token", u.csrf)
	}
	response, err := u.client.Do(r)
	if err != nil {
		u.w.t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		u.w.t.Fatalf("%s %s: %s", method, path, raw)
	}
	return response.StatusCode, e
}

func (u *user) list(query string) (page, *meta) {
	u.w.t.Helper()
	status, body := u.do("GET", "/me/notifications"+query, "")
	if status != 200 {
		u.w.t.Fatalf("list%s = %d %+v", query, status, body.Error)
	}
	var p page
	if err := json.Unmarshal(body.Data, &p); err != nil {
		u.w.t.Fatal(err)
	}
	return p, body.Meta
}

func (u *user) read(id string) (int, item, envelope) {
	u.w.t.Helper()
	status, body := u.do("POST", "/me/notifications/"+id+"/read", "")
	var it item
	if status == 200 {
		if err := json.Unmarshal(body.Data, &it); err != nil {
			u.w.t.Fatal(err)
		}
	}
	return status, it, body
}

func (w *world) unreadCount(recipient string) int {
	w.t.Helper()
	var n int
	if err := w.f.DB.QueryRow("SELECT count(*) FROM notifications WHERE recipient_id=? AND read_at IS NULL", recipient).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func TestBuyerSeesOnlyOwnRejectionNotifications(t *testing.T) {
	w := newWorld(t)
	older, olderDeal, olderProduct := w.rejected(employeeA, employeeB)
	newer, _, _ := w.rejected(admin, employeeB)
	w.rejected(employeeB, employeeA) // A의 알림은 B에게 보이면 안 된다

	b := w.login(employeeB)
	p, m := b.list("")
	if len(p.Items) != 2 || p.NextCursor != nil {
		t.Fatalf("B items=%d next=%v", len(p.Items), p.NextCursor)
	}
	if p.Items[0].ID != newer || p.Items[1].ID != older {
		t.Fatalf("newest first expected, got %s, %s", p.Items[0].ID, p.Items[1].ID)
	}
	got := p.Items[1]
	if got.Type != "APPLICATION_REJECTED" || got.DealID != olderDeal || got.ProductID != olderProduct || got.ReadAt != nil || got.CreatedAt != "2026-11-05T03:00:00.000Z" {
		t.Fatalf("unexpected item %+v", got)
	}
	if m == nil || !m.HasUnreadNotifications {
		t.Fatalf("meta must show unread notifications: %+v", m)
	}
	if w.unreadCount(employeeB) != 2 {
		t.Fatal("listing must not mark notifications as read")
	}
}

func TestEmptyInboxReturnsEmptyArray(t *testing.T) {
	w := newWorld(t)
	status, body := w.login(admin).do("GET", "/me/notifications", "")
	if status != 200 || string(body.Data) != `{"items":[],"nextCursor":null}` {
		t.Fatalf("empty inbox = %d %s", status, body.Data)
	}
	if body.Meta == nil || body.Meta.HasUnreadNotifications {
		t.Fatalf("meta = %+v", body.Meta)
	}
}

func TestInboxPagesWithCursor(t *testing.T) {
	w := newWorld(t)
	var ids []string
	for i := 0; i < 5; i++ {
		id, _, _ := w.rejected(employeeA, employeeB)
		ids = append([]string{id}, ids...) // 최신순
	}
	b := w.login(employeeB)
	first, _ := b.list("?limit=2")
	if len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("first page %+v", first)
	}
	second, _ := b.list("?limit=2&cursor=" + *first.NextCursor)
	third, _ := b.list("?limit=2&cursor=" + *second.NextCursor)
	if third.NextCursor != nil || len(third.Items) != 1 {
		t.Fatalf("last page %+v", third)
	}
	var seen []string
	for _, p := range []page{first, second, third} {
		for _, it := range p.Items {
			seen = append(seen, it.ID)
		}
	}
	for i := range ids {
		if seen[i] != ids[i] {
			t.Fatalf("page order %v != %v", seen, ids)
		}
	}
	exact, _ := b.list("?limit=5")
	if exact.NextCursor != nil {
		t.Fatal("no next cursor when the page holds every remaining item")
	}
	for _, q := range []string{"?limit=0", "?limit=101", "?limit=x", "?cursor=not-base64!"} {
		if status, _ := b.do("GET", "/me/notifications"+q, ""); status != 400 {
			t.Fatalf("%s = %d", q, status)
		}
	}
}

func TestOnlyRecipientCanMarkNotificationRead(t *testing.T) {
	w := newWorld(t)
	first, _, _ := w.rejected(employeeA, employeeB)
	second, _, _ := w.rejected(employeeA, employeeB)

	for _, account := range []string{employeeA, admin} {
		status, _, body := w.login(account).read(first)
		if status != 404 || body.Error == nil || body.Error.Code != "NOT_FOUND" {
			t.Fatalf("%s reading another user's notification = %d", account, status)
		}
	}
	if w.unreadCount(employeeB) != 2 {
		t.Fatal("other users must not change read state")
	}

	b := w.login(employeeB)
	w.advance(time.Hour)
	status, it, body := b.read(first)
	if status != 200 || it.ID != first || it.ReadAt == nil || *it.ReadAt != "2026-11-06T02:00:00.000Z" {
		t.Fatalf("read = %d %+v", status, it)
	}
	if body.Meta == nil || !body.Meta.HasUnreadNotifications {
		t.Fatal("another notification is still unread")
	}

	w.advance(time.Hour)
	status, again, _ := b.read(first)
	if status != 200 || again.ReadAt == nil || *again.ReadAt != *it.ReadAt {
		t.Fatalf("repeated read must keep the first read time: %d %+v", status, again)
	}
	status, _, body = b.read(second)
	if status != 200 || body.Meta == nil || body.Meta.HasUnreadNotifications {
		t.Fatalf("all read: %d %+v", status, body.Meta)
	}
	var total int
	if err := w.f.DB.QueryRow("SELECT count(*) FROM notifications").Scan(&total); err != nil || total != 2 {
		t.Fatalf("reading must not create notifications: %d %v", total, err)
	}
	if status, _, _ := b.read("missing"); status != 404 {
		t.Fatalf("unknown id = %d", status)
	}
}

func TestNewNotificationAfterReadIsUnread(t *testing.T) {
	w := newWorld(t)
	first, _, _ := w.rejected(employeeA, employeeB)
	b := w.login(employeeB)
	if _, _, body := b.read(first); body.Meta.HasUnreadNotifications {
		t.Fatal("expected no unread after reading the only notification")
	}
	later, _, _ := w.rejected(admin, employeeB)
	p, m := b.list("")
	if !m.HasUnreadNotifications || p.Items[0].ID != later || p.Items[0].ReadAt != nil || p.Items[1].ReadAt == nil {
		t.Fatalf("new notification must stay unread: %+v %+v", p.Items, m)
	}
}

func TestHiddenProductNotificationStaysInInbox(t *testing.T) {
	w := newWorld(t)
	id, deal, product := w.rejected(employeeA, employeeB)
	if _, err := w.f.DB.Exec("UPDATE products SET deleted_at=? WHERE id=?", "2026-11-05T04:00:00.000Z", product); err != nil {
		t.Fatal(err)
	}
	p, _ := w.login(employeeB).list("")
	if len(p.Items) != 1 || p.Items[0].ID != id || p.Items[0].DealID != deal {
		t.Fatalf("hidden product notification %+v", p.Items)
	}
}

func TestInboxRequiresLoginAndCSRF(t *testing.T) {
	w := newWorld(t)
	id, _, _ := w.rejected(employeeA, employeeB)
	anon := w.anonymous()
	if status, _ := anon.do("GET", "/me/notifications", ""); status != 401 {
		t.Fatalf("anonymous list = %d", status)
	}
	if status, _, _ := anon.read(id); status != 401 {
		t.Fatalf("anonymous read = %d", status)
	}
	b := w.login(employeeB)
	b.csrf = "wrong"
	if status, _, _ := b.read(id); status != 403 {
		t.Fatalf("read without CSRF = %d", status)
	}
	if w.unreadCount(employeeB) != 1 {
		t.Fatal("rejected requests must not mark read")
	}
}

func TestConcurrentReadsAndNewNotification(t *testing.T) {
	w := newWorld(t)
	id, _, _ := w.rejected(employeeA, employeeB)
	b := w.login(employeeB)
	var wg sync.WaitGroup
	readTimes := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _ := http.NewRequest("POST", w.server.URL+"/api/v1/me/notifications/"+id+"/read", nil)
			r.Header.Set("Origin", origin)
			r.Header.Set("X-CSRF-Token", b.csrf)
			response, err := b.client.Do(r)
			if err != nil {
				t.Error(err)
				return
			}
			defer response.Body.Close()
			var body struct {
				Data item `json:"data"`
			}
			if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&body) != nil || body.Data.ReadAt == nil {
				t.Errorf("concurrent read = %d", response.StatusCode)
				return
			}
			readTimes <- *body.Data.ReadAt
		}()
	}
	w.advance(time.Minute)
	later, _, _ := w.rejected(admin, employeeB) // 읽는 도중 새 거절 알림이 도착한다
	wg.Wait()
	close(readTimes)
	first := ""
	for ts := range readTimes {
		if first == "" {
			first = ts
		} else if ts != first {
			t.Fatalf("read time changed between concurrent reads: %s vs %s", first, ts)
		}
	}
	p, m := b.list("")
	if !m.HasUnreadNotifications || p.Items[0].ID != later || p.Items[0].ReadAt != nil {
		t.Fatalf("notification created during reads must remain unread: %+v", p.Items)
	}
}
