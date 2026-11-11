package catalog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/catalog"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
)

type harness struct {
	t       *testing.T
	fixture *testkit.Fixture
	server  *httptest.Server
	client  *http.Client
}

func setup(t *testing.T) *harness {
	t.Helper()
	f := testkit.New(t)
	var seq int
	var name, path string
	if err := f.DB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	c := config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: "http://localhost:5173", SessionTTL: 8 * time.Hour, Port: "0", DatabasePath: path, UploadDir: filepath.Join(t.TempDir(), "uploads"), WebDir: t.TempDir()}
	app, err := application.New(context.Background(), c, catalog.Register)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.HTTP.Handler)
	t.Cleanup(func() { server.Close(); app.Close() })
	jar, _ := cookiejar.New(nil)
	return &harness{t, f, server, &http.Client{Jar: jar}}
}
func (h *harness) request(method, path, body string) (int, []byte) {
	h.t.Helper()
	r, _ := http.NewRequest(method, h.server.URL+"/api/v1"+path, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://localhost:5173")
	res, err := h.client.Do(r)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	return res.StatusCode, b
}
func (h *harness) login(user string) {
	h.t.Helper()
	status, b := h.request("POST", "/auth/development-login", `{"accountKey":"`+user+`"}`)
	if status != 200 {
		h.t.Fatalf("login %d %s", status, b)
	}
}
func (h *harness) exec(query string, args ...any) {
	h.t.Helper()
	if _, err := h.fixture.DB.Exec(query, args...); err != nil {
		h.t.Fatal(err)
	}
}
func data(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var v struct{ Data map[string]any }
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v.Data
}

func TestEmployeesOnlyBrowseApprovedAnonymousProducts(t *testing.T) {
	h := setup(t)
	public := h.fixture.Product("dev-employee-a", "APPROVED", 0, 3)
	pending := h.fixture.Product("dev-employee-a", "PENDING", 100000000, 2)
	h.fixture.Product("dev-employee-a", "REJECTED", 100, 1)
	hidden := h.fixture.Product("dev-employee-a", "APPROVED", 100, 1)
	h.exec("UPDATE products SET deleted_at=created_at WHERE id=?", hidden)
	if status, _ := h.request("GET", "/products", ""); status != 401 {
		t.Fatalf("anonymous list=%d", status)
	}
	h.login("dev-employee-b")
	status, b := h.request("GET", "/products", "")
	if status != 200 {
		t.Fatalf("list=%d %s", status, b)
	}
	items := data(t, b)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != public {
		t.Fatalf("private catalog leaked: %s", b)
	}
	for _, secret := range []string{"sellerId", "uploaderId", "dev-employee-a", "realName", "department", "jobLevel", "email"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("identity leaked: %s", b)
		}
	}
	status, b = h.request("GET", "/products/"+public, "")
	if status != 200 || data(t, b)["canApply"] != true {
		t.Fatalf("detail %d %s", status, b)
	}
	if status, _ := h.request("GET", "/products/"+pending, ""); status != 404 {
		t.Fatalf("pending exposed=%d", status)
	}
	h.login("dev-employee-a")
	status, b = h.request("GET", "/products/"+pending, "")
	if status != 200 || data(t, b)["canApply"] != false {
		t.Fatalf("owner detail=%d %s", status, b)
	}
}

func TestSearchPaginationAndAvailability(t *testing.T) {
	h := setup(t)
	h.login("dev-employee-b")
	ids := []string{}
	for i := 0; i < 3; i++ {
		id := h.fixture.Product("dev-employee-a", "APPROVED", 100000000, 2)
		ids = append(ids, id)
		h.exec("UPDATE products SET title='사과 100%_ 할인',description='직접 재배한 사과' WHERE id=?", id)
	}
	h.exec("UPDATE products SET id='a' WHERE id=?", ids[0])
	h.exec("UPDATE products SET id='b',remaining_quantity=0 WHERE id=?", ids[1])
	h.exec("UPDATE products SET id='c',recruitment_closed=1 WHERE id=?", ids[2])
	status, b := h.request("GET", "/products?q=%25_&limit=2", "")
	if status != 200 {
		t.Fatalf("search %d %s", status, b)
	}
	page := data(t, b)
	items := page["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["id"] != "c" || items[1].(map[string]any)["id"] != "b" {
		t.Fatalf("sort %s", b)
	}
	for _, i := range items {
		if i.(map[string]any)["canApply"] != false {
			t.Fatalf("unavailable apply %s", b)
		}
	}
	cursor := page["nextCursor"].(string)
	status, b = h.request("GET", "/products?q=%25_&limit=2&cursor="+cursor, "")
	if status != 200 {
		t.Fatal(status)
	}
	last := data(t, b)
	if len(last["items"].([]any)) != 1 || last["items"].([]any)[0].(map[string]any)["id"] != "a" || last["nextCursor"] != nil {
		t.Fatalf("pagination %s", b)
	}
	_, b = h.request("GET", "/products?q=없는검색어", "")
	if len(data(t, b)["items"].([]any)) != 0 {
		t.Fatal(string(b))
	}
	_, b = h.request("GET", "/products?q=%27%20OR%201%3D1--", "")
	if len(data(t, b)["items"].([]any)) != 0 {
		t.Fatalf("search interpolation %s", b)
	}
	for _, query := range []string{"limit=0", "limit=101", "cursor=bad", "q=" + strings.Repeat("a", 201)} {
		if status, _ := h.request("GET", "/products?"+query, ""); status != 400 {
			t.Fatalf("invalid %s = %d", query, status)
		}
	}
}

func TestDetailPhotosDoNotRevealStorageOrPeople(t *testing.T) {
	h := setup(t)
	public := h.fixture.Product("dev-employee-a", "APPROVED", 100000000, 1)
	h.exec(`INSERT INTO images(id,uploader_id,product_id,storage_key,sort_order,created_at) VALUES('img-b','dev-employee-a',?,'private-b/real-name.jpg',1,'2026-11-05T03:00:00.000Z'),('img-a','dev-employee-a',?,'private-a/real-name.jpg',0,'2026-11-05T03:00:00.000Z')`, public, public)
	h.login("dev-employee-b")
	status, b := h.request("GET", "/products/"+public, "")
	if status != 200 {
		t.Fatal(string(b))
	}
	d := data(t, b)
	imgs := d["images"].([]any)
	if len(imgs) != 2 || imgs[0].(map[string]any)["url"] != "/api/v1/images/img-a" || imgs[1].(map[string]any)["sortOrder"] != float64(1) {
		t.Fatalf("photo contract %s", b)
	}
	if d["unitPrice"] != float64(100000000) {
		t.Fatal(string(b))
	}
	if strings.Contains(string(b), "private-") || strings.Contains(string(b), "real-name") || strings.Contains(string(b), "dev-employee-a") {
		t.Fatalf("private data %s", b)
	}
	h.exec("UPDATE products SET deleted_at=created_at WHERE id=?", public)
	if status, _ := h.request("GET", "/products/"+public, ""); status != 404 {
		t.Fatalf("hidden=%d", status)
	}
	h.login("dev-admin")
	if status, _ := h.request("GET", "/products/"+public, ""); status != 404 {
		t.Fatalf("admin must use separate private route=%d", status)
	}
	h.login("dev-employee-a")
	status, b = h.request("GET", "/products/"+public, "")
	if status != 200 || data(t, b)["canApply"] != false {
		t.Fatalf("hidden owner %d %s", status, b)
	}
	h.exec("UPDATE users SET status='SUSPENDED' WHERE id='dev-employee-a'")
	if status, _ := h.request("GET", "/products", ""); status != 403 {
		t.Fatalf("suspended=%d", status)
	}
}

func TestOwnerSeesOwnReviewStatesWithoutExposingThemToOthers(t *testing.T) {
	h := setup(t)
	approved := h.fixture.Product("dev-employee-a", "APPROVED", 100, 1)
	pending := h.fixture.Product("dev-employee-a", "PENDING", 100, 1)
	rejected := h.fixture.Product("dev-employee-a", "REJECTED", 100, 1)
	hidden := h.fixture.Product("dev-employee-a", "APPROVED", 100, 1)
	h.exec("UPDATE products SET deleted_at=created_at WHERE id=?", hidden)
	h.login("dev-employee-a")
	status, b := h.request("GET", "/products", "")
	if status != 200 {
		t.Fatal(status, string(b))
	}
	items := data(t, b)["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("owner products missing: %s", b)
	}
	seen := map[string]string{}
	for _, v := range items {
		p := v.(map[string]any)
		if p["isOwn"] != true || p["canApply"] != false {
			t.Fatalf("own product state: %s", b)
		}
		seen[p["id"].(string)] = p["approvalStatus"].(string)
	}
	if seen[approved] != "APPROVED" || seen[pending] != "PENDING" || seen[rejected] != "REJECTED" {
		t.Fatal(seen)
	}
	h.login("dev-employee-b")
	_, b = h.request("GET", "/products", "")
	items = data(t, b)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["isOwn"] != false {
		t.Fatalf("unapproved product leaked: %s", b)
	}
	for _, id := range []string{pending, rejected} {
		if status, _ := h.request("GET", "/products/"+id, ""); status != 404 {
			t.Fatal("private detail leaked", status)
		}
	}
}
