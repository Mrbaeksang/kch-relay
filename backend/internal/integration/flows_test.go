package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/catalog"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/notifications"
	"github.com/Mrbaeksang/kch-relay/backend/internal/product"
	"github.com/Mrbaeksang/kch-relay/backend/internal/statistics"
	"github.com/Mrbaeksang/kch-relay/backend/internal/transactions"
)

type client struct {
	t    *testing.T
	base string
	http *http.Client
	csrf string
}

func setup(t *testing.T) (*client, *client, *client) {
	t.Helper()
	dir := t.TempDir()
	c := config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: "http://relay.test", SessionTTL: 8 * time.Hour, DatabasePath: filepath.Join(dir, "market.db"), UploadDir: filepath.Join(dir, "uploads"), WebDir: dir}
	app, err := application.New(context.Background(), c, statistics.Register, notifications.Module(nil), catalog.Register, product.Module, transactions.Module(func(ctx context.Context, tx *sql.Tx, p, s string, ids []string, _ time.Time) error {
		return product.AttachImagesTx(ctx, tx, p, s, ids)
	}))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.HTTP.Handler)
	t.Cleanup(func() { server.Close(); app.Close() })
	login := func(key string) *client {
		jar, _ := cookiejar.New(nil)
		cl := &client{t: t, base: server.URL, http: &http.Client{Jar: jar}}
		data := cl.do(200, "POST", "/auth/development-login", map[string]string{"accountKey": key})
		cl.csrf = data["csrfToken"].(string)
		return cl
	}
	return login("dev-employee-a"), login("dev-employee-b"), login("dev-admin")
}
func (c *client) raw(method, path string, body io.Reader, contentType string) *http.Response {
	c.t.Helper()
	r, _ := http.NewRequest(method, c.base+"/api/v1"+path, body)
	r.Header.Set("Origin", "http://relay.test")
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	r.Header.Set("X-CSRF-Token", c.csrf)
	res, err := c.http.Do(r)
	if err != nil {
		c.t.Fatal(err)
	}
	return res
}
func (c *client) do(want int, method, path string, body any) map[string]any {
	c.t.Helper()
	var input io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		input = bytes.NewReader(b)
	}
	r := c.raw(method, path, input, "application/json")
	defer r.Body.Close()
	raw, _ := io.ReadAll(r.Body)
	if r.StatusCode != want {
		c.t.Fatalf("%s %s: %d want%d %s", method, path, r.StatusCode, want, raw)
	}
	if want >= 400 {
		return nil
	}
	var v struct{ Data map[string]any }
	if err := json.Unmarshal(raw, &v); err != nil {
		c.t.Fatal(err)
	}
	return v.Data
}
func (c *client) photo() string {
	c.t.Helper()
	var imageBytes, body bytes.Buffer
	png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	w := multipart.NewWriter(&body)
	part, _ := w.CreateFormFile("file", "fake.png")
	part.Write(imageBytes.Bytes())
	w.Close()
	res := c.raw("POST", "/images", &body, w.FormDataContentType())
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		c.t.Fatal(res.StatusCode, string(b))
	}
	var v struct{ Data struct{ ID string } }
	json.NewDecoder(res.Body).Decode(&v)
	return v.Data.ID
}
func TestRegistrationPhotoEditAndDealAgreement(t *testing.T) {
	a, b, admin := setup(t)
	first := a.photo()
	created := a.do(201, "POST", "/products", map[string]any{"title": "가상 상품", "description": "실제 직원 정보 없는 통합 테스트", "unitPrice": 10000000000, "quantity": 5, "imageIds": []string{first}})
	p := created["id"].(string)
	path := "/products/" + p
	b.do(404, "GET", path, nil)
	admin.do(200, "POST", "/admin/products/"+p+"/approve", map[string]any{})
	if b.do(200, "GET", path, nil)["canApply"] != true {
		t.Fatal("approved product cannot apply")
	}
	d := b.do(201, "POST", path+"/deals", map[string]int{"quantity": 2})
	id := d["id"].(string)
	if d["counterparty"].(map[string]any)["identity"] != nil {
		t.Fatal("identity before accept")
	}
	if b.do(200, "GET", path, nil)["canApply"] != false {
		t.Fatal("duplicate application allowed by catalog")
	}
	second := a.photo()
	edit := map[string]any{"imageIds": []string{second}}
	a.do(409, "PATCH", path, edit)
	edit["rejectPendingApplications"] = true
	a.do(200, "PATCH", path, edit)
	if b.do(200, "GET", "/deals/"+id, nil)["status"] != "REJECTED" {
		t.Fatal("photo edit did not reject")
	}
	imageResponse := b.raw("GET", "/images/"+second, nil, "")
	imageResponse.Body.Close()
	if imageResponse.StatusCode != 200 {
		t.Fatal("edited photo missing")
	}
	d = b.do(201, "POST", path+"/deals", map[string]int{"quantity": 2})
	id = d["id"].(string)
	a.do(200, "POST", "/deals/"+id+"/accept", nil)
	if b.do(200, "GET", "/deals/"+id, nil)["counterparty"].(map[string]any)["identity"] == nil {
		t.Fatal("no accepted identity")
	}
	a.do(200, "PATCH", path, map[string]any{"unitPrice": 9000000000})
	final := a.do(200, "POST", "/deals/"+id+"/complete", nil)
	if final["agreedUnitPrice"] != float64(10000000000) || final["agreedQuantity"] != float64(2) {
		t.Fatal("agreement changed", final)
	}
	a.do(200, "DELETE", path, nil)
	b.do(404, "GET", path, nil)
	if b.do(200, "GET", "/deals/"+id, nil)["status"] != "COMPLETED" {
		t.Fatal("hidden record lost")
	}
}

func TestHiddenProductRejectionReachesNotificationInbox(t *testing.T) {
	a, b, admin := setup(t)
	p := a.do(201, "POST", "/products", map[string]any{"title": "가상 나눔", "description": "가상 통합 테스트", "unitPrice": 0, "quantity": 1})["id"].(string)
	admin.do(200, "POST", "/admin/products/"+p+"/approve", map[string]any{})
	d := b.do(201, "POST", "/products/"+p+"/deals", map[string]int{"quantity": 1})["id"].(string)
	a.do(200, "DELETE", "/products/"+p, nil)
	inbox := b.do(200, "GET", "/me/notifications", nil)["items"].([]any)
	if len(inbox) != 1 {
		t.Fatal(inbox)
	}
	n := inbox[0].(map[string]any)
	if n["dealId"] != d || n["readAt"] != nil {
		t.Fatal(n)
	}
	read := b.do(200, "POST", "/me/notifications/"+n["id"].(string)+"/read", nil)
	if read["readAt"] == nil {
		t.Fatal("read not saved")
	}
	record := b.do(200, "GET", "/deals/"+d, nil)
	if record["status"] != "REJECTED" || record["product"].(map[string]any)["hidden"] != true {
		t.Fatal(record)
	}
}

func TestSellerCompletionReachesKoreanPeriodStatistics(t *testing.T) {
	a, b, admin := setup(t)
	p := a.do(201, "POST", "/products", map[string]any{"title": "통계용 가상 상품", "description": "통합 테스트", "unitPrice": 150000000, "quantity": 3})["id"].(string)
	admin.do(200, "POST", "/admin/products/"+p+"/approve", map[string]any{})
	d := b.do(201, "POST", "/products/"+p+"/deals", map[string]int{"quantity": 2})["id"].(string)
	a.do(200, "POST", "/deals/"+d+"/accept", nil)
	a.do(200, "PATCH", "/products/"+p, map[string]int{"unitPrice": 0})
	complete := a.do(200, "POST", "/deals/"+d+"/complete", nil)
	stamp, err := time.Parse(time.RFC3339Nano, complete["completedAt"].(string))
	if err != nil {
		t.Fatal(err)
	}
	day := stamp.In(time.FixedZone("Asia/Seoul", 9*60*60)).Format("2006-01-02")
	a.do(200, "DELETE", "/products/"+p, nil)
	path := "/admin/statistics/completed-deals?from=" + day + "&to=" + day
	b.do(403, "GET", path, nil)
	totals := admin.do(200, "GET", path, nil)
	if totals["completedDealCount"] != float64(1) || totals["totalCompletedAmount"] != float64(300000000) {
		t.Fatal(totals)
	}
}
