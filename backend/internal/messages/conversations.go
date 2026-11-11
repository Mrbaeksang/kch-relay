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

const previewLength = 100

type lastMessageView struct {
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	FromSelf  bool   `json:"fromSelf"`
}

type summaryView struct {
	ConversationID string           `json:"conversationId"`
	ViewerRole     string           `json:"viewerRole"`
	Product        productRef       `json:"product"`
	Counterparty   counterpartyView `json:"counterparty"`
	LastMessage    *lastMessageView `json:"lastMessage"`
	HasUnread      bool             `json:"hasUnread"`
}

type productRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func preview(body string) string {
	if r := []rune(body); len(r) > previewLength {
		return string(r[:previewLength])
	}
	return body
}

// myConversations lists the viewer's conversations as buyer and as seller,
// most recently active first. A seller only sees conversations that already
// carry a message, so opening an inquiry without writing leaves no trace.
func (s *Service) myConversations(r *http.Request) (httpx.Result, error) {
	user := actor(r).ID
	page, err := httpx.ParsePage(r.URL.Query())
	if err != nil {
		return httpx.Result{}, err
	}
	ctx := r.Context()
	const sortAt = "COALESCE(lm.created_at,p.created_at)"
	query := `SELECT p.id,p.product_id,pr.title,pr.seller_id,p.buyer_id,p.inquirer_number,p.applicant_number,p.identity_revealed_at IS NOT NULL,p.created_at,
 lm.body,lm.created_at,lm.sender_id,
 EXISTS(SELECT 1 FROM messages u WHERE u.participant_id=p.id AND u.recipient_id=? AND u.read_at IS NULL),` + sortAt + `
 FROM participants p JOIN products pr ON pr.id=p.product_id
 LEFT JOIN messages lm ON lm.id=(SELECT m.id FROM messages m WHERE m.participant_id=p.id ORDER BY m.created_at DESC,m.id DESC LIMIT 1)
 WHERE (p.buyer_id=? OR (pr.seller_id=? AND lm.id IS NOT NULL))`
	args := []any{user, user, user}
	if page.After != nil {
		query += " AND (" + sortAt + "<? OR (" + sortAt + "=? AND p.id<?))"
		args = append(args, page.After.CreatedAt, page.After.CreatedAt, page.After.ID)
	}
	query += " ORDER BY " + sortAt + " DESC,p.id DESC LIMIT ?"
	args = append(args, page.Limit+1)
	type row struct {
		c      conversation
		last   *lastMessageView
		unread bool
		sortAt string
	}
	var found []row
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return httpx.Result{}, err
	}
	for rows.Next() {
		var x row
		var body, created, sender sql.NullString
		if err := rows.Scan(&x.c.ID, &x.c.ProductID, &x.c.ProductTitle, &x.c.SellerID, &x.c.BuyerID, &x.c.Inquirer, &x.c.Applicant, &x.c.Revealed, &x.c.CreatedAt,
			&body, &created, &sender, &x.unread, &x.sortAt); err != nil {
			rows.Close()
			return httpx.Result{}, err
		}
		if body.Valid {
			x.last = &lastMessageView{Body: preview(body.String), CreatedAt: created.String, FromSelf: sender.String == user}
		}
		found = append(found, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return httpx.Result{}, err
	}
	var next *string
	if len(found) > page.Limit {
		found = found[:page.Limit]
		last := found[len(found)-1]
		cursor := httpx.EncodeCursor(httpx.Cursor{CreatedAt: last.sortAt, ID: last.c.ID})
		next = &cursor
	}
	// The database has one connection, so identities are read after the rows are closed.
	items := make([]summaryView, 0, len(found))
	for _, x := range found {
		other, err := counterparty(ctx, s.db, x.c, user)
		if err != nil {
			return httpx.Result{}, err
		}
		role := "BUYER"
		if x.c.viewerIsSeller(user) {
			role = "SELLER"
		}
		items = append(items, summaryView{ConversationID: x.c.ID, ViewerRole: role, Product: productRef{ID: x.c.ProductID, Title: x.c.ProductTitle},
			Counterparty: other, LastMessage: x.last, HasUnread: x.unread})
	}
	return httpx.Result{Data: map[string]any{"items": items, "nextCursor": next}}, nil
}

type identity struct {
	RealName   string `json:"realName"`
	Department string `json:"department"`
	JobLevel   string `json:"jobLevel"`
}

type counterpartyView struct {
	Label    string    `json:"label"`
	Identity *identity `json:"identity"`
}

// counterparty describes the other party for the viewer. The identity is
// filled only after the two of them were matched in an accepted deal.
func counterparty(ctx context.Context, q queryer, c conversation, viewerID string) (counterpartyView, error) {
	v := counterpartyView{Label: counterpartyLabel(c.viewerIsSeller(viewerID), c.Inquirer, c.Applicant)}
	if !c.Revealed {
		return v, nil
	}
	var id identity
	if err := q.QueryRowContext(ctx, "SELECT real_name,department,job_level FROM users WHERE id=?", otherParty(c, viewerID)).Scan(&id.RealName, &id.Department, &id.JobLevel); err != nil {
		return v, err
	}
	v.Identity = &id
	return v, nil
}
