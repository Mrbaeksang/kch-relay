package statistics_test

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
	"github.com/Mrbaeksang/kch-relay/backend/internal/statistics"
	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
	"github.com/go-chi/chi/v5"
)

const origin = "http://localhost:5173"

type server struct {
	t   *testing.T
	f   *testkit.Fixture
	url string
}

// newServer assembles the authenticated module group the same way application.New does, on a fixture database.
func newServer(t *testing.T) *server {
	t.Helper()
	f := testkit.New(t)
	c := config.Config{AppEnv: "development", AuthMode: "development", AppOrigin: origin, SessionTTL: time.Hour}
	service, err := auth.New(f.DB, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	deps := application.Dependencies{DB: f.DB, Config: c, Meta: func(ctx context.Context, user string) (httpx.Meta, error) {
		return database.UnreadMeta(ctx, f.DB, user)
	}}
	handler := httpapi.NewRouter(f.DB, fstest.MapFS{}, func(r chi.Router) {
		r.Route("/api/v1", func(api chi.Router) {
			api.Use(service.Authenticate)
			service.Register(api)
			api.Group(func(protected chi.Router) {
				protected.Use(httpx.RequireUserMiddleware)
				statistics.Register(protected, deps)
			})
		})
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return &server{t: t, f: f, url: ts.URL + "/api/v1"}
}

// client returns a browser-like client logged in as a development account, or anonymous for "".
func (s *server) client(account string) *http.Client {
	s.t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	if account == "" {
		return c
	}
	r, _ := http.NewRequest("POST", s.url+"/auth/development-login", bytes.NewBufferString(`{"accountKey":"`+account+`"}`))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	response, err := c.Do(r)
	if err != nil {
		s.t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		s.t.Fatalf("login %s=%d", account, response.StatusCode)
	}
	return c
}

// get calls the statistics API and decodes numbers exactly so large totals are not rounded by the test itself.
func (s *server) get(c *http.Client, query string) (int, map[string]any) {
	s.t.Helper()
	r, _ := http.NewRequest("GET", s.url+"/admin/statistics/completed-deals"+query, nil)
	r.Header.Set("Origin", origin)
	response, err := c.Do(r)
	if err != nil {
		s.t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var body map[string]any
	if err := decoder.Decode(&body); err != nil {
		s.t.Fatalf("invalid response %s", raw)
	}
	return response.StatusCode, body
}

// dealAt records a deal in the given status whose transition happened at the given Korean wall-clock time.
func (s *server) dealAt(participant, status, kst string, quantity, price int64) string {
	s.t.Helper()
	at, err := time.ParseInLocation("2006-01-02 15:04:05.000", kst, time.FixedZone("KST", 9*60*60))
	if err != nil {
		s.t.Fatal(err)
	}
	s.f.Now = at
	return s.f.Deal(participant, status, quantity, price)
}

func (s *server) buyer() (product, participant string) {
	s.t.Helper()
	product = s.f.Product("dev-employee-b", "APPROVED", 15000, 100)
	return product, s.f.Participant(product, "dev-employee-a", "application").ID
}

func data(t *testing.T, status int, body map[string]any) map[string]any {
	t.Helper()
	if status != 200 {
		t.Fatalf("status=%d body=%v", status, body)
	}
	return body["data"].(map[string]any)
}

func expectTotals(t *testing.T, got map[string]any, count, amount string) {
	t.Helper()
	if got["completedDealCount"].(json.Number).String() != count || got["totalCompletedAmount"].(json.Number).String() != amount {
		t.Fatalf("count=%v amount=%v, want %s / %s", got["completedDealCount"], got["totalCompletedAmount"], count, amount)
	}
}

func TestCountsAndSumsOnlyCompletedDealsAtAgreedValues(t *testing.T) {
	s := newServer(t)
	admin := s.client("dev-admin")
	status, body := s.get(admin, "?from=2026-11-01&to=2026-11-30")
	expectTotals(t, data(t, status, body), "0", "0")

	_, participant := s.buyer()
	for _, status := range []string{"REQUESTED", "REJECTED", "WITHDRAWN", "ACCEPTED"} {
		s.dealAt(participant, status, "2026-11-10 12:00:00.000", 2, 15000)
	}
	s.dealAt(participant, "COMPLETED", "2026-11-10 12:00:00.000", 2, 15000)
	s.dealAt(participant, "COMPLETED", "2026-11-11 09:30:00.000", 3, 7000)
	s.dealAt(participant, "COMPLETED", "2026-11-12 18:00:00.000", 1, 0)

	status, body = s.get(admin, "?from=2026-11-01&to=2026-11-30")
	got := data(t, status, body)
	expectTotals(t, got, "3", "51000")
	for field, want := range map[string]string{"from": "2026-11-01", "to": "2026-11-30", "timezone": "Asia/Seoul", "currency": "KRW"} {
		if got[field] != want {
			t.Fatalf("%s=%v, want %s", field, got[field], want)
		}
	}
	if _, ok := body["meta"].(map[string]any)["hasUnreadNotifications"]; !ok {
		t.Fatalf("authenticated response must carry unread meta: %v", body["meta"])
	}
}

func TestKoreanDateBoundariesIncludeWholeEndDay(t *testing.T) {
	s := newServer(t)
	admin := s.client("dev-admin")
	_, participant := s.buyer()
	// 금액을 2의 거듭제곱으로 두어 합계만 보고도 어느 거래가 포함됐는지 알 수 있게 한다.
	for kst, price := range map[string]int64{
		"2026-10-31 23:59:59.999": 1,   // 11월 직전
		"2026-11-01 00:00:00.000": 2,   // 11월 시작
		"2026-11-30 23:59:59.999": 4,   // 11월 마지막 시각
		"2026-12-01 00:00:00.000": 8,   // 12월 시작
		"2026-12-31 23:59:59.999": 16,  // 연말 마지막 시각
		"2027-01-01 00:00:00.000": 32,  // 새해 시작
		"2027-01-01 23:59:59.999": 64,  // 새해 첫날 마지막 시각
		"2027-01-02 00:00:00.000": 128, // 다음 날 시작
	} {
		s.dealAt(participant, "COMPLETED", kst, 1, price)
	}
	for query, want := range map[string][2]string{
		"?from=2026-11-01&to=2026-11-30": {"2", "6"},
		"?from=2026-10-31&to=2026-10-31": {"1", "1"},
		"?from=2026-12-01&to=2026-12-31": {"2", "24"},
		"?from=2026-12-31&to=2027-01-01": {"3", "112"},
		"?from=2027-01-01&to=2027-01-01": {"2", "96"},
		"?from=2026-10-01&to=2027-12-31": {"8", "255"},
		"?from=2027-01-03&to=2027-01-31": {"0", "0"},
	} {
		status, body := s.get(admin, query)
		got := data(t, status, body)
		if got["completedDealCount"].(json.Number).String() != want[0] || got["totalCompletedAmount"].(json.Number).String() != want[1] {
			t.Errorf("%s count=%v amount=%v, want %v", query, got["completedDealCount"], got["totalCompletedAmount"], want)
		}
	}
}

func TestRejectsMissingReversedOrImpossibleDates(t *testing.T) {
	s := newServer(t)
	admin := s.client("dev-admin")
	for _, query := range []string{"", "?from=2026-11-01", "?to=2026-11-30", "?from=2026-11-30&to=2026-11-01", "?from=2026-02-30&to=2026-03-01", "?from=2026-11-1&to=2026-11-30", "?from=2026-11-01T00:00&to=2026-11-30", "?from=2026-11-01&to=2026-13-01"} {
		status, body := s.get(admin, query)
		if status != 400 || body["error"].(map[string]any)["code"] != "VALIDATION_ERROR" {
			t.Errorf("%q status=%d body=%v", query, status, body)
		}
	}
}

func TestProductPriceChangeAndHidingDoNotRewriteCompletedAmounts(t *testing.T) {
	s := newServer(t)
	admin := s.client("dev-admin")
	product, participant := s.buyer()
	s.dealAt(participant, "COMPLETED", "2026-11-10 12:00:00.000", 2, 15000)
	if _, err := s.f.DB.Exec("UPDATE products SET unit_price=99000, deleted_at=? WHERE id=?", database.Timestamp(time.Now()), product); err != nil {
		t.Fatal(err)
	}
	status, body := s.get(admin, "?from=2026-11-01&to=2026-11-30")
	expectTotals(t, data(t, status, body), "1", "30000")
}

func TestLargeAmountsStayExactOrFailLoudly(t *testing.T) {
	const maxSafe = 1<<53 - 1 // 브라우저 숫자가 정확히 표현하는 최대 정수
	t.Run("1억원 이상 단가와 여러 수량", func(t *testing.T) {
		s := newServer(t)
		_, participant := s.buyer()
		s.dealAt(participant, "COMPLETED", "2026-11-10 12:00:00.000", 3, 150_000_000)
		s.dealAt(participant, "COMPLETED", "2026-11-11 12:00:00.000", 7, 123_456_789)
		status, body := s.get(s.client("dev-admin"), "?from=2026-11-01&to=2026-11-30")
		expectTotals(t, data(t, status, body), "2", "1314197523")
	})
	t.Run("정확히 표현 가능한 최대 합계", func(t *testing.T) {
		s := newServer(t)
		_, participant := s.buyer()
		s.dealAt(participant, "COMPLETED", "2026-11-10 12:00:00.000", 1, maxSafe-1)
		s.dealAt(participant, "COMPLETED", "2026-11-10 13:00:00.000", 1, 1)
		status, body := s.get(s.client("dev-admin"), "?from=2026-11-01&to=2026-11-30")
		expectTotals(t, data(t, status, body), "2", "9007199254740991")
	})
	for name, deals := range map[string][][2]int64{
		"합계가 표현 범위를 1원 초과":  {{1, maxSafe}, {1, 1}},
		"단가×수량이 64비트 정수 초과": {{4, 1 << 62}},
		"합계가 64비트 정수 초과":    {{1, 1 << 62}, {1, 1 << 62}},
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t)
			_, participant := s.buyer()
			for _, d := range deals {
				s.dealAt(participant, "COMPLETED", "2026-11-10 12:00:00.000", d[0], d[1])
			}
			status, body := s.get(s.client("dev-admin"), "?from=2026-11-01&to=2026-11-30")
			failure, _ := body["error"].(map[string]any)
			if status != 409 || failure["code"] != "AMOUNT_OUT_OF_RANGE" {
				t.Fatalf("status=%d body=%v", status, body)
			}
		})
	}
}

func TestOnlyHRAdminCanViewCompletedDealStatistics(t *testing.T) {
	s := newServer(t)
	for _, tc := range []struct {
		account string
		status  int
		code    string
	}{{"", 401, "UNAUTHENTICATED"}, {"dev-employee-a", 403, "FORBIDDEN"}, {"dev-admin", 200, ""}} {
		status, body := s.get(s.client(tc.account), "?from=2026-11-01&to=2026-11-30")
		if status != tc.status {
			t.Fatalf("%q status=%d body=%v", tc.account, status, body)
		}
		if tc.code != "" && body["error"].(map[string]any)["code"] != tc.code {
			t.Fatalf("%q error=%v", tc.account, body["error"])
		}
	}
}
