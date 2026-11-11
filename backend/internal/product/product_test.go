package product_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/product"
)

const origin = "http://localhost:5173"

type env struct {
	t      *testing.T
	dir    string
	cfg    config.Config
	app    *application.Application
	server *httptest.Server
}

func start(t *testing.T, dir string) *env {
	t.Helper()
	cfg := config.Config{AppEnv: "development", AuthMode: "development", Port: "0", AppOrigin: origin, SessionTTL: 8 * time.Hour,
		DatabasePath: filepath.Join(dir, "market.db"), UploadDir: filepath.Join(dir, "uploads"), WebDir: dir}
	app, err := application.New(context.Background(), cfg, product.Module)
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, dir: dir, cfg: cfg, app: app, server: httptest.NewServer(app.HTTP.Handler)}
	t.Cleanup(e.stop)
	return e
}
func (e *env) stop() {
	if e.server != nil {
		e.server.Close()
		e.app.Close()
		e.server = nil
	}
}

type client struct {
	e    *env
	http *http.Client
	csrf string
}

func (e *env) login(key string) *client {
	e.t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &client{e: e, http: &http.Client{Jar: jar}}
	status, body := c.do("POST", "/auth/development-login", map[string]string{"accountKey": key})
	if status != 200 {
		e.t.Fatalf("login %s: %d %v", key, status, body)
	}
	c.csrf = body["data"].(map[string]any)["csrfToken"].(string)
	return c
}
func (c *client) raw(method, path string, body io.Reader, contentType string) *http.Response {
	c.e.t.Helper()
	r, _ := http.NewRequest(method, c.e.server.URL+"/api/v1"+path, body)
	r.Header.Set("Origin", origin)
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	if c.csrf != "" {
		r.Header.Set("X-CSRF-Token", c.csrf)
	}
	res, err := c.http.Do(r)
	if err != nil {
		c.e.t.Fatal(err)
	}
	return res
}
func (c *client) do(method, path string, payload any) (int, map[string]any) {
	c.e.t.Helper()
	var body io.Reader
	contentType := ""
	if payload != nil {
		raw, _ := json.Marshal(payload)
		body = bytes.NewReader(raw)
		contentType = "application/json"
	}
	return decode(c.e.t, c.raw(method, path, body, contentType))
}
func decode(t *testing.T, res *http.Response) (int, map[string]any) {
	t.Helper()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("not json (%d): %s", res.StatusCode, raw)
	}
	return res.StatusCode, out
}
func (c *client) upload(name string, content []byte) (int, map[string]any) {
	c.e.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", name)
	part.Write(content)
	w.Close()
	return decode(c.e.t, c.raw("POST", "/images", &buf, w.FormDataContentType()))
}
func (c *client) uploadID(content []byte) string {
	c.e.t.Helper()
	status, body := c.upload("홍길동_사원증.jpg", content)
	if status != 201 {
		c.e.t.Fatalf("upload: %d %v", status, body)
	}
	return data(body)["id"].(string)
}
func (c *client) imageStatus(id string) int {
	c.e.t.Helper()
	res := c.raw("GET", "/images/"+id, nil, "")
	res.Body.Close()
	return res.StatusCode
}
func data(body map[string]any) map[string]any { return body["data"].(map[string]any) }
func items(body map[string]any) []any         { return data(body)["items"].([]any) }
func code(body map[string]any) string {
	if e, ok := body["error"].(map[string]any); ok {
		return e["code"].(string)
	}
	return ""
}

// JPEG with an EXIF APP1 segment carrying a fake author, to prove re-encoding strips it.
func jpegWithExif(t *testing.T) []byte {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.RGBA{200, 10, 10, 255})
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	exif := append([]byte("Exif\x00\x00"), []byte("Artist=홍길동")...)
	segment := append([]byte{0xFF, 0xE1, byte((len(exif) + 2) >> 8), byte(len(exif) + 2)}, exif...)
	return append(append(append([]byte{}, raw[:2]...), segment...), raw[2:]...)
}
func pngBytes(t *testing.T) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func valid(images ...string) map[string]any {
	if images == nil {
		images = []string{}
	}
	return map[string]any{"title": "사과 한 박스", "description": "한 개는 사과 한 박스입니다.", "unitPrice": 10000, "quantity": 3, "imageIds": images}
}

func TestRegisterReviewRejectResubmitApprove(t *testing.T) {
	e := start(t, t.TempDir())
	a, b, admin := e.login("dev-employee-a"), e.login("dev-employee-b"), e.login("dev-admin")

	photo := a.uploadID(jpegWithExif(t))
	status, body := a.do("POST", "/products", valid(photo))
	if status != 201 {
		t.Fatalf("create: %d %v", status, body)
	}
	created := data(body)
	id := created["id"].(string)
	if created["approvalStatus"] != "PENDING" || created["remainingQuantity"].(float64) != 3 || len(created["images"].([]any)) != 1 {
		t.Fatalf("created product: %v", created)
	}
	if _, leaked := created["seller"]; leaked {
		t.Fatal("seller identity must not be part of product response")
	}

	// Employees cannot review; images of unapproved products are hidden from others.
	for _, path := range []string{"/admin/products", "/admin/products/" + id, "/admin/products/" + id + "/participants"} {
		if status, body := b.do("GET", path, nil); status != 403 {
			t.Fatalf("employee GET %s: %d %v", path, status, body)
		}
	}
	if status, _ := b.do("POST", "/admin/products/"+id+"/approve", map[string]any{}); status != 403 {
		t.Fatal("employee approved", status)
	}
	if b.imageStatus(photo) != 404 || a.imageStatus(photo) != 200 || admin.imageStatus(photo) != 200 {
		t.Fatal("pending product image visibility is wrong")
	}

	// Admin sees the queue with author identity and rejects with a reason.
	status, body = admin.do("GET", "/admin/products?approvalStatus=PENDING", nil)
	if status != 200 || len(items(body)) != 1 {
		t.Fatalf("queue: %d %v", status, body)
	}
	seller := items(body)[0].(map[string]any)["seller"].(map[string]any)
	if seller["realName"] != "테스트 직원 A" || seller["department"] != "TEST_DEPT" {
		t.Fatalf("admin must see author identity: %v", seller)
	}
	if status, body := admin.do("POST", "/admin/products/"+id+"/reject", map[string]any{"reason": "  "}); status != 400 {
		t.Fatalf("blank reason: %d %v", status, body)
	}
	if status, body := admin.do("POST", "/admin/products/"+id+"/reject", map[string]any{"reason": "가격을 다시 확인해 주세요."}); status != 200 || data(body)["approvalStatus"] != "REJECTED" {
		t.Fatalf("reject: %d %v", status, body)
	}
	if status, body := admin.do("POST", "/admin/products/"+id+"/approve", map[string]any{}); status != 409 || code(body) != "INVALID_STATE" {
		t.Fatalf("review outside PENDING: %d %v", status, body)
	}

	// Seller sees the reason, fixes and resubmits; other users cannot.
	status, body = a.do("GET", "/me/products", nil)
	mine := items(body)[0].(map[string]any)
	if status != 200 || mine["approvalStatus"] != "REJECTED" || mine["rejectionReason"] != "가격을 다시 확인해 주세요." {
		t.Fatalf("my products: %d %v", status, body)
	}
	if status, body := b.do("GET", "/me/products", nil); status != 200 || len(items(body)) != 0 {
		t.Fatal("other user's products leaked", body)
	}
	fixed := valid()
	fixed["unitPrice"] = 9000
	if status, _ := b.do("POST", "/products/"+id+"/submit-review", fixed); status != 404 {
		t.Fatal("non-owner resubmitted", status)
	}
	if status, body := a.do("POST", "/products/"+id+"/submit-review", fixed); status != 200 || data(body)["approvalStatus"] != "PENDING" || data(body)["unitPrice"].(float64) != 9000 {
		t.Fatalf("resubmit: %d %v", status, body)
	}
	if status, body := a.do("POST", "/products/"+id+"/submit-review", fixed); status != 409 {
		t.Fatalf("resubmit while pending: %d %v", status, body)
	}
	// The photo was removed in the fix, so it is an unlinked upload again: owner only.
	if a.imageStatus(photo) != 200 || admin.imageStatus(photo) != 404 {
		t.Fatal("unlinked photo visibility is wrong")
	}

	if status, body := admin.do("POST", "/admin/products/"+id+"/approve", map[string]any{}); status != 200 || data(body)["approvalStatus"] != "APPROVED" {
		t.Fatalf("approve: %d %v", status, body)
	}
	status, body = admin.do("GET", "/admin/products/"+id, nil)
	reviews := data(body)["reviews"].([]any)
	if status != 200 || len(reviews) != 2 || reviews[0].(map[string]any)["decision"] != "REJECTED" || reviews[1].(map[string]any)["decision"] != "APPROVED" {
		t.Fatalf("review history: %d %v", status, body)
	}

	// Approved product photos become visible to every employee.
	second := a.uploadID(pngBytes(t))
	if b.imageStatus(second) != 404 {
		t.Fatal("unlinked photo visible to others")
	}

	// Persistence across restart: stop and reopen the same data directory.
	e.stop()
	e2 := start(t, e.dir)
	a2 := e2.login("dev-employee-a")
	status, body = a2.do("GET", "/me/products", nil)
	if status != 200 || len(items(body)) != 1 || items(body)[0].(map[string]any)["approvalStatus"] != "APPROVED" {
		t.Fatalf("after restart: %d %v", status, body)
	}
	if a2.imageStatus(second) != 200 {
		t.Fatal("uploaded file lost after restart")
	}
}

func TestApprovedProductPhotosArePublicToEmployees(t *testing.T) {
	e := start(t, t.TempDir())
	a, b, admin := e.login("dev-employee-a"), e.login("dev-employee-b"), e.login("dev-admin")
	photo := a.uploadID(pngBytes(t))
	_, body := a.do("POST", "/products", valid(photo))
	id := data(body)["id"].(string)
	admin.do("POST", "/admin/products/"+id+"/approve", map[string]any{})
	if b.imageStatus(photo) != 200 {
		t.Fatal("approved product photo hidden")
	}
}

func TestProductInputLimits(t *testing.T) {
	e := start(t, t.TempDir())
	a := e.login("dev-employee-a")
	status, body := a.do("GET", "/product-limits", nil)
	if status != 200 {
		t.Fatal(status, body)
	}
	limits := data(body)
	maxPrice := int64(limits["maxUnitPrice"].(float64))
	if maxPrice < 100_000_000 {
		t.Fatalf("must support at least 1억원: %d", maxPrice)
	}
	for _, accepted := range []map[string]any{
		{"unitPrice": 0},
		{"unitPrice": 100_000_000},
		{"unitPrice": maxPrice, "quantity": int64(limits["maxQuantity"].(float64))},
		{"title": strings.Repeat("가", int(limits["maxTitleLength"].(float64)))},
	} {
		p := valid()
		for k, v := range accepted {
			p[k] = v
		}
		if status, body := a.do("POST", "/products", p); status != 201 {
			t.Fatalf("%v rejected: %d %v", accepted, status, body)
		}
	}
	for name, rejected := range map[string]map[string]any{
		"negative price":   {"unitPrice": -1},
		"fractional price": {"unitPrice": 1.5},
		"over max price":   {"unitPrice": maxPrice + 1},
		"zero quantity":    {"quantity": 0},
		"over quantity":    {"quantity": int64(limits["maxQuantity"].(float64)) + 1},
		"blank title":      {"title": "   "},
		"long title":       {"title": strings.Repeat("가", int(limits["maxTitleLength"].(float64))+1)},
		"blank body":       {"description": ""},
		"missing price":    {"unitPrice": nil},
		"too many images":  {"imageIds": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}},
		"duplicate images": {"imageIds": []string{"x", "x"}},
	} {
		p := valid()
		for k, v := range rejected {
			if v == nil {
				delete(p, k)
			} else {
				p[k] = v
			}
		}
		if status, body := a.do("POST", "/products", p); status != 400 || code(body) != "VALIDATION_ERROR" {
			t.Errorf("%s accepted: %d %v", name, status, body)
		}
	}
	status, body = a.do("GET", "/me/products?limit=100", nil)
	if status != 200 || len(items(body)) != 4 {
		t.Fatalf("only valid products stored: %v", body)
	}
}

func TestImageUploadSafety(t *testing.T) {
	e := start(t, t.TempDir())
	a, b := e.login("dev-employee-a"), e.login("dev-employee-b")

	status, body := a.upload("../../evil.jpg", jpegWithExif(t))
	if status != 201 {
		t.Fatalf("upload: %d %v", status, body)
	}
	id := data(body)["id"].(string)
	if data(body)["url"] != "/api/v1/images/"+id {
		t.Fatalf("url: %v", body)
	}
	entries, _ := os.ReadDir(e.cfg.UploadDir)
	if len(entries) != 1 || strings.Contains(entries[0].Name(), "evil") || strings.Contains(entries[0].Name(), "홍길동") {
		t.Fatalf("stored file name must be server generated: %v", entries)
	}
	if _, err := os.Stat(filepath.Join(e.dir, "evil.jpg")); err == nil {
		t.Fatal("path traversal wrote outside upload dir")
	}
	res := a.raw("GET", "/images/"+id, nil, "")
	stored, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" || bytes.Contains(stored, []byte("Exif")) || bytes.Contains(stored, []byte("홍길동")) {
		t.Fatalf("stored image must be re-encoded without EXIF: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "private") || !strings.Contains(cc, "no-cache") {
		t.Fatalf("image must be revalidated on every view so another account never gets a cached copy: %q", cc)
	}
	// Revalidation still checks permission: the owner gets 304, others 404.
	r304, _ := http.NewRequest("GET", e.server.URL+"/api/v1/images/"+id, nil)
	r304.Header.Set("If-None-Match", res.Header.Get("ETag"))
	if res, _ := a.http.Do(r304); res.StatusCode != 304 {
		t.Fatalf("owner revalidation: %d", res.StatusCode)
	}
	rOther, _ := http.NewRequest("GET", e.server.URL+"/api/v1/images/"+id, nil)
	rOther.Header.Set("If-None-Match", res.Header.Get("ETag"))
	if res, _ := b.http.Do(rOther); res.StatusCode != 404 {
		t.Fatalf("other user's revalidation must not succeed: %d", res.StatusCode)
	}

	for name, content := range map[string][]byte{
		"html polyglot": []byte("<html><script>alert(1)</script></html>"),
		"svg":           []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"truncated":     jpegWithExif(t)[:40],
		"empty":         {},
	} {
		if status, body := a.upload("a.jpg", content); status != 400 || code(body) != "VALIDATION_ERROR" {
			t.Errorf("%s accepted: %d %v", name, status, body)
		}
	}
	big := bytes.Repeat([]byte{0xFF}, int(product.MaxImageBytes)+10)
	if status, _ := a.upload("big.jpg", big); status != 400 {
		t.Error("oversized upload accepted", status)
	}

	// Someone else's image cannot be attached or viewed.
	if b.imageStatus(id) != 404 {
		t.Fatal("other user saw unlinked upload")
	}
	if status, body := b.do("POST", "/products", valid(id)); status != 400 {
		t.Fatalf("attached someone else's image: %d %v", status, body)
	}
	if b.imageStatus("does-not-exist") != 404 || b.imageStatus("..%2F..%2Fmarket.db") != 404 {
		t.Fatal("unknown image must be 404")
	}
	if status, _ := b.do("GET", "/me/products", nil); status != 200 {
		t.Fatal(status)
	}
	// An image already used by one product cannot be attached to another.
	_, body = a.do("POST", "/products", valid(id))
	if status, body := a.do("POST", "/products", valid(id)); status != 400 {
		t.Fatalf("image reused across products: %d %v", status, body)
	}
}

func TestParticipantsIdentityIsAdminOnly(t *testing.T) {
	e := start(t, t.TempDir())
	a, admin := e.login("dev-employee-a"), e.login("dev-admin")
	_, body := a.do("POST", "/products", valid())
	id := data(body)["id"].(string)
	admin.do("POST", "/admin/products/"+id+"/approve", map[string]any{})

	db, err := database.Open(context.Background(), e.cfg.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now()
	for kind, buyer := range map[string]string{"inquiry": "dev-employee-b", "application": "dev-admin"} {
		tx, _ := db.Begin()
		if _, err := database.EnsureParticipantTx(context.Background(), tx, id, buyer, kind, now); err != nil {
			t.Fatal(err)
		}
		tx.Commit()
	}

	if status, _ := a.do("GET", "/admin/products/"+id+"/participants", nil); status != 403 {
		t.Fatal("seller saw participant identities", status)
	}
	status, body := admin.do("GET", "/admin/products/"+id+"/participants", nil)
	if status != 200 || len(items(body)) != 2 {
		t.Fatalf("participants: %d %v", status, body)
	}
	labels := map[string]string{}
	for _, item := range items(body) {
		m := item.(map[string]any)
		labels[m["realName"].(string)] = m["label"].(string)
		if _, leaked := m["email"]; leaked {
			t.Fatal("email leaked")
		}
	}
	if labels["테스트 직원 B"] != "문의자 A" || labels["테스트 관리자"] != "익명 1" {
		t.Fatalf("labels: %v", labels)
	}
	if status, _ := admin.do("GET", "/admin/products/missing/participants", nil); status != 404 {
		t.Fatal("missing product", status)
	}
	if status, body := admin.do("GET", "/admin/products?approvalStatus=WRONG", nil); status != 400 {
		t.Fatal("invalid filter", status, body)
	}
}
