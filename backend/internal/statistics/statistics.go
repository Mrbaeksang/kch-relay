// Package statistics reports completed marketplace deal totals for HR administrators.
package statistics

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// maxSafeAmount is the largest integer a browser number keeps exact (Number.MAX_SAFE_INTEGER).
const maxSafeAmount = 1<<53 - 1

var errOutOfRange = httpx.Error(409, "AMOUNT_OUT_OF_RANGE", "합계 금액이 정확히 표시할 수 있는 범위를 넘었습니다. 기간을 나누어 조회해 주세요.")

type CompletedDeals struct {
	From                 string `json:"from"`
	To                   string `json:"to"`
	Timezone             string `json:"timezone"`
	Currency             string `json:"currency"`
	CompletedDealCount   int64  `json:"completedDealCount"`
	TotalCompletedAmount int64  `json:"totalCompletedAmount"`
}

func Register(r chi.Router, d application.Dependencies) {
	r.Get("/admin/statistics/completed-deals", httpx.Adapt(httpx.RequireAdmin(completedDeals(d.DB)), d.Meta))
}

// completedDeals sums agreed price × quantity of deals completed within the inclusive Korean date range.
func completedDeals(db *sql.DB) httpx.Handler {
	return func(r *http.Request) (httpx.Result, error) {
		from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
		start, end, err := httpx.ParseDateRange(from, to)
		if err != nil {
			return httpx.Result{}, err
		}
		out := CompletedDeals{From: from, To: to, Timezone: "Asia/Seoul", Currency: "KRW"}
		var total any
		err = db.QueryRowContext(r.Context(), `SELECT COUNT(*), COALESCE(SUM(agreed_unit_price*agreed_quantity),0) FROM deals WHERE status='COMPLETED' AND completed_at>=? AND completed_at<?`,
			database.Timestamp(start), database.Timestamp(end)).Scan(&out.CompletedDealCount, &total)
		if err != nil && strings.Contains(err.Error(), "integer overflow") {
			return httpx.Result{}, errOutOfRange
		}
		if err != nil {
			return httpx.Result{}, err
		}
		// SQLite turns an overflowing multiplication into REAL instead of failing, so anything but an exact integer is rejected.
		amount, exact := total.(int64)
		if !exact || amount > maxSafeAmount {
			return httpx.Result{}, errOutOfRange
		}
		out.TotalCompletedAmount = amount
		return httpx.Result{Data: out}, nil
	}
}
