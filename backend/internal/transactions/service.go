package transactions

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Input limits owned by this feature (docs/decisions/market-transactions.md).
// Other screens read them from API errors instead of hard-coding them.
const (
	MaxTitleLength       = 100
	MaxDescriptionLength = 5000
	MaxUnitPrice         = int64(1_000_000_000)
	MaxQuantity          = int64(1_000_000)
)

type Service struct {
	db     *sql.DB
	meta   httpx.MetaProvider
	now    func() time.Time
	attach ImageAttacher
}

func (s *Service) register(r chi.Router) {
	h := func(fn httpx.Handler) http.HandlerFunc { return httpx.Adapt(fn, s.meta) }
	r.Patch("/products/{productId}", h(s.updateProduct))
	r.Patch("/products/{productId}/quantity", h(s.updateRemainingQuantity))
	r.Post("/products/{productId}/close", h(s.closeRecruitment))
	r.Post("/products/{productId}/reopen", h(s.reopenRecruitment))
	r.Delete("/products/{productId}", h(s.hideProduct))
	r.Post("/products/{productId}/deals", h(s.apply))
	r.Get("/products/{productId}/deals", h(s.productDeals))
	r.Get("/me/deals", h(s.myDeals))
	r.Get("/deals/{dealId}", h(s.getDeal))
	r.Patch("/deals/{dealId}/quantity", h(s.updateDealQuantity))
	r.Post("/deals/{dealId}/withdraw", h(s.withdraw))
	r.Post("/deals/{dealId}/reject", h(s.reject))
	r.Post("/deals/{dealId}/accept", h(s.accept))
	r.Post("/deals/{dealId}/complete", h(s.complete))
}

var (
	errProductNotFound = httpx.Error(404, "NOT_FOUND", "상품을 찾을 수 없습니다.")
	errDealNotFound    = httpx.Error(404, "NOT_FOUND", "거래를 찾을 수 없습니다.")
	errStateChanged    = httpx.Error(409, "INVALID_STATE", "상태가 바뀌었습니다. 새로고침 후 다시 확인해 주세요.")
)

func conflict(code, message string, details any) error {
	return &httpx.AppError{Status: 409, Code: code, Message: message, Details: details}
}

func actor(r *http.Request) httpx.Actor { a, _ := httpx.CurrentUser(r.Context()); return a }

// write runs fn in one transaction. The shared database uses a single
// connection, so transactions are serialized; every state change is still a
// conditional UPDATE so a stale read can never oversell or double-apply.
func (s *Service) write(ctx context.Context, fn func(*sql.Tx, time.Time) error) error {
	now := s.now().UTC()
	return database.WithTx(ctx, s.db, func(tx *sql.Tx) error { return fn(tx, now) })
}

func ts(t time.Time) string { return database.Timestamp(t) }

// rejectDealTx rejects one waiting application and notifies its applicant in
// the caller's transaction, so a notification failure undoes the rejection.
func rejectDealTx(ctx context.Context, tx *sql.Tx, dealID, buyerID string, now time.Time) error {
	res, err := tx.ExecContext(ctx, `UPDATE deals SET status='REJECTED',rejected_at=?,updated_at=? WHERE id=? AND status='REQUESTED'`, ts(now), ts(now), dealID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errStateChanged
	}
	_, err = database.AppendRejectionNotificationTx(ctx, tx, dealID, buyerID, now)
	return err
}

func rejectPendingTx(ctx context.Context, tx *sql.Tx, productID string, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT d.id,p.buyer_id FROM deals d JOIN participants p ON p.id=d.participant_id WHERE p.product_id=? AND d.status='REQUESTED'`, productID)
	if err != nil {
		return err
	}
	var pending [][2]string
	for rows.Next() {
		var deal, buyer string
		if err := rows.Scan(&deal, &buyer); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, [2]string{deal, buyer})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range pending {
		if err := rejectDealTx(ctx, tx, p[0], p[1], now); err != nil {
			return err
		}
	}
	return nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// CanApply reports whether user may submit a new application for the product
// now. Product screens use it for canApply instead of re-deriving the policy.
func CanApply(ctx context.Context, q queryer, productID, userID string) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM products pr WHERE pr.id=? AND pr.approval_status='APPROVED' AND pr.deleted_at IS NULL
 AND pr.recruitment_closed=0 AND pr.remaining_quantity>0 AND pr.seller_id<>?
 AND NOT EXISTS(SELECT 1 FROM deals d JOIN participants p ON p.id=d.participant_id WHERE p.product_id=pr.id AND p.buyer_id=? AND d.status IN ('REQUESTED','ACCEPTED')))`, productID, userID, userID).Scan(&ok)
	return ok, err
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
