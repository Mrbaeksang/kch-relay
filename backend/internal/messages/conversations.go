package messages

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

var (
	errProductNotFound      = httpx.Error(404, "NOT_FOUND", "상품을 찾을 수 없습니다.")
	errConversationNotFound = httpx.Error(404, "NOT_FOUND", "대화를 찾을 수 없습니다.")
	errProductUnavailable   = httpx.Error(409, "INVALID_STATE", "승인되어 공개 중인 상품에만 새로 문의할 수 있습니다.")
	errOwnProduct           = httpx.Invalid("본인 상품에는 문의할 수 없습니다.")
)

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// conversation is one participants row seen from its seller or its buyer.
type conversation struct {
	ID, ProductID, ProductTitle, SellerID, BuyerID string
	Inquirer, Applicant                            sql.NullInt64
	Revealed                                       bool
	CreatedAt                                      string
}

func (c conversation) viewerIsSeller(userID string) bool { return c.SellerID == userID }

// loadConversation returns the conversation only to its two parties; anyone
// else (administrators included) gets the same 404 as for a missing ID.
func loadConversation(ctx context.Context, q queryer, conversationID, userID string) (conversation, error) {
	var c conversation
	err := q.QueryRowContext(ctx, `SELECT p.id,p.product_id,pr.title,pr.seller_id,p.buyer_id,p.inquirer_number,p.applicant_number,p.identity_revealed_at IS NOT NULL,p.created_at
 FROM participants p JOIN products pr ON pr.id=p.product_id WHERE p.id=? AND (pr.seller_id=? OR p.buyer_id=?)`, conversationID, userID, userID).
		Scan(&c.ID, &c.ProductID, &c.ProductTitle, &c.SellerID, &c.BuyerID, &c.Inquirer, &c.Applicant, &c.Revealed, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, errConversationNotFound
	}
	return c, err
}

// startConversation reuses the buyer's participant on the product, so the
// conversation stays the same before and after an application. A new one is
// opened only on an approved, visible product.
func (s *Service) startConversation(r *http.Request) (httpx.Result, error) {
	user := actor(r).ID
	productID := chi.URLParam(r, "productId")
	var id string
	status := http.StatusOK
	err := s.write(r.Context(), func(tx *sql.Tx, now time.Time) error {
		err := tx.QueryRowContext(r.Context(), "SELECT id FROM participants WHERE product_id=? AND buyer_id=?", productID, user).Scan(&id)
		if err == nil {
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var sellerID, approval string
		var deleted sql.NullString
		err = tx.QueryRowContext(r.Context(), "SELECT seller_id,approval_status,deleted_at FROM products WHERE id=?", productID).Scan(&sellerID, &approval, &deleted)
		if errors.Is(err, sql.ErrNoRows) {
			return errProductNotFound
		}
		if err != nil {
			return err
		}
		if sellerID == user {
			return errOwnProduct
		}
		if approval != "APPROVED" || deleted.Valid {
			return errProductUnavailable
		}
		p, err := database.EnsureParticipantTx(r.Context(), tx, productID, user, "inquiry", now)
		id, status = p.ID, http.StatusCreated
		return err
	})
	if err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Status: status, Data: map[string]string{"conversationId": id}}, nil
}
