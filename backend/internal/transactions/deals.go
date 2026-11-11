package transactions

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

type identity struct {
	RealName   string `json:"realName"`
	Department string `json:"department"`
	JobLevel   string `json:"jobLevel"`
}

type dealProduct struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	UnitPrice         int64  `json:"unitPrice"`
	RemainingQuantity int64  `json:"remainingQuantity"`
	RecruitmentClosed bool   `json:"recruitmentClosed"`
	Hidden            bool   `json:"hidden"`
}

// dealView is one deal as seen by one of its two parties. It never carries a
// global user ID; the counterparty is an alias until the deal is agreed.
type dealView struct {
	ID                string      `json:"id"`
	ProductID         string      `json:"productId"`
	ConversationID    string      `json:"conversationId"`
	Status            string      `json:"status"`
	ViewerRole        string      `json:"viewerRole"`
	Product           dealProduct `json:"product"`
	RequestedQuantity int64       `json:"requestedQuantity"`
	AgreedUnitPrice   *int64      `json:"agreedUnitPrice"`
	AgreedQuantity    *int64      `json:"agreedQuantity"`
	TotalPrice        *int64      `json:"totalPrice"`
	RequestedAt       string      `json:"requestedAt"`
	QuantityUpdatedAt *string     `json:"quantityUpdatedAt"`
	AcceptedAt        *string     `json:"acceptedAt"`
	CompletedAt       *string     `json:"completedAt"`
	RejectedAt        *string     `json:"rejectedAt"`
	WithdrawnAt       *string     `json:"withdrawnAt"`
	Counterparty      struct {
		Label    string    `json:"label"`
		Identity *identity `json:"identity"`
	} `json:"counterparty"`
	Actions   []string `json:"actions"`
	createdAt string
}

const dealSelect = `SELECT d.id,pr.id,p.id,d.status,pr.title,pr.unit_price,pr.remaining_quantity,pr.recruitment_closed,pr.deleted_at IS NOT NULL,
 pr.seller_id,p.applicant_number,p.identity_revealed_at IS NOT NULL,
 d.requested_quantity,d.agreed_unit_price,d.agreed_quantity,d.requested_at,d.quantity_updated_at,d.accepted_at,d.completed_at,d.rejected_at,d.withdrawn_at,d.created_at,
 s.real_name,s.department,s.job_level,b.real_name,b.department,b.job_level
 FROM deals d JOIN participants p ON p.id=d.participant_id JOIN products pr ON pr.id=p.product_id
 JOIN users s ON s.id=pr.seller_id JOIN users b ON b.id=p.buyer_id`

type scanner interface{ Scan(...any) error }

func scanDeal(row scanner, viewerID string) (dealView, error) {
	var d dealView
	var sellerID string
	var applicant sql.NullInt64
	var revealed bool
	var price, quantity sql.NullInt64
	var updated, accepted, completed, rejected, withdrawn sql.NullString
	var sellerIdentity, buyerIdentity identity
	err := row.Scan(&d.ID, &d.ProductID, &d.ConversationID, &d.Status, &d.Product.Title, &d.Product.UnitPrice, &d.Product.RemainingQuantity, &d.Product.RecruitmentClosed, &d.Product.Hidden,
		&sellerID, &applicant, &revealed,
		&d.RequestedQuantity, &price, &quantity, &d.RequestedAt, &updated, &accepted, &completed, &rejected, &withdrawn, &d.createdAt,
		&sellerIdentity.RealName, &sellerIdentity.Department, &sellerIdentity.JobLevel, &buyerIdentity.RealName, &buyerIdentity.Department, &buyerIdentity.JobLevel)
	if err != nil {
		return d, err
	}
	d.Product.ID = d.ProductID
	opt := func(s sql.NullString) *string {
		if s.Valid {
			return &s.String
		}
		return nil
	}
	d.QuantityUpdatedAt, d.AcceptedAt, d.CompletedAt, d.RejectedAt, d.WithdrawnAt = opt(updated), opt(accepted), opt(completed), opt(rejected), opt(withdrawn)
	if price.Valid && quantity.Valid {
		total := price.Int64 * quantity.Int64
		d.AgreedUnitPrice, d.AgreedQuantity, d.TotalPrice = &price.Int64, &quantity.Int64, &total
	}
	counterpart := buyerIdentity
	if viewerID == sellerID {
		d.ViewerRole = "SELLER"
		d.Counterparty.Label = fmt.Sprintf("익명 %d", applicant.Int64)
	} else {
		d.ViewerRole = "BUYER"
		d.Counterparty.Label = "판매자"
		counterpart = sellerIdentity
	}
	// Identity is revealed only once the seller has accepted this buyer on this
	// product; it then stays revealed between the same two people there.
	if revealed {
		d.Counterparty.Identity = &counterpart
	}
	d.Actions = []string{}
	switch {
	case d.Status == "REQUESTED" && d.ViewerRole == "BUYER":
		d.Actions = []string{"updateQuantity", "withdraw"}
	case d.Status == "REQUESTED" && !d.Product.Hidden:
		d.Actions = []string{"accept", "reject"}
	case d.Status == "REQUESTED":
		d.Actions = []string{"reject"}
	case d.Status == "ACCEPTED" && d.ViewerRole == "SELLER":
		d.Actions = []string{"complete"}
	}
	return d, nil
}

// loadDeal returns the deal only to its seller or buyer; anyone else gets 404.
func loadDeal(ctx context.Context, q queryer, dealID, viewerID string) (dealView, error) {
	d, err := scanDeal(q.QueryRowContext(ctx, dealSelect+" WHERE d.id=? AND (pr.seller_id=? OR p.buyer_id=?)", dealID, viewerID, viewerID), viewerID)
	if isNoRows(err) {
		return d, errDealNotFound
	}
	return d, err
}

func readQuantity(r *http.Request) (int64, error) {
	var in struct {
		Quantity *int64 `json:"quantity"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return 0, err
	}
	if in.Quantity == nil || *in.Quantity < 1 || *in.Quantity > MaxQuantity {
		return 0, httpx.Invalid(fmt.Sprintf("신청 수량은 1 이상 %d 이하로 입력해 주세요.", MaxQuantity))
	}
	return *in.Quantity, nil
}

func insufficient(remaining int64) error {
	return conflict("INSUFFICIENT_QUANTITY", "남은 수량이 부족합니다.", map[string]int64{"remainingQuantity": remaining})
}

// apply creates a waiting application. It reserves no stock.
func (s *Service) apply(r *http.Request) (httpx.Result, error) {
	quantity, err := readQuantity(r)
	if err != nil {
		return httpx.Result{}, err
	}
	ctx, productID, buyer := r.Context(), chi.URLParam(r, "productId"), actor(r).ID
	var out dealView
	err = s.write(ctx, func(tx *sql.Tx, now time.Time) error {
		var sellerID, approval string
		var hidden, closed bool
		var remaining int64
		err := tx.QueryRowContext(ctx, "SELECT seller_id,approval_status,deleted_at IS NOT NULL,recruitment_closed,remaining_quantity FROM products WHERE id=?", productID).Scan(&sellerID, &approval, &hidden, &closed, &remaining)
		if isNoRows(err) || err == nil && (approval != "APPROVED" || hidden) {
			return errProductNotFound
		}
		if err != nil {
			return err
		}
		switch {
		case sellerID == buyer:
			return httpx.Error(403, "FORBIDDEN", "본인 상품에는 신청할 수 없습니다.")
		case closed:
			return conflict("RECRUITMENT_CLOSED", "모집이 종료된 상품입니다.", nil)
		case quantity > remaining:
			return insufficient(remaining)
		}
		var active string
		err = tx.QueryRowContext(ctx, `SELECT d.id FROM deals d JOIN participants p ON p.id=d.participant_id WHERE p.product_id=? AND p.buyer_id=? AND d.status IN ('REQUESTED','ACCEPTED')`, productID, buyer).Scan(&active)
		if err == nil {
			return conflict("ACTIVE_DEAL_EXISTS", "이미 진행 중인 신청이나 거래가 있습니다.", map[string]string{"dealId": active})
		}
		if !isNoRows(err) {
			return err
		}
		participant, err := database.EnsureParticipantTx(ctx, tx, productID, buyer, "application", now)
		if err != nil {
			return err
		}
		id := httpx.NewID()
		if _, err := tx.ExecContext(ctx, `INSERT INTO deals(id,participant_id,status,requested_quantity,requested_at,created_at,updated_at) VALUES(?,?,'REQUESTED',?,?,?,?)`, id, participant.ID, quantity, ts(now), ts(now), ts(now)); err != nil {
			return err
		}
		out, err = loadDeal(ctx, tx, id, buyer)
		return err
	})
	return httpx.Result{Status: 201, Data: out}, err
}

var dealStatuses = map[string]bool{"REQUESTED": true, "WITHDRAWN": true, "REJECTED": true, "ACCEPTED": true, "COMPLETED": true}

func (s *Service) listDeals(r *http.Request, viewerID, where string, args ...any) (httpx.Result, error) {
	page, err := httpx.ParsePage(r.URL.Query())
	if err != nil {
		return httpx.Result{}, err
	}
	query := dealSelect + " WHERE " + where
	if status := r.URL.Query().Get("status"); status != "" {
		if !dealStatuses[status] {
			return httpx.Result{}, httpx.Error(400, "INVALID_ENUM_VALUE", "거래 상태 값이 올바르지 않습니다.")
		}
		query += " AND d.status=?"
		args = append(args, status)
	}
	if page.After != nil {
		query += " AND (d.created_at<? OR (d.created_at=? AND d.id<?))"
		args = append(args, page.After.CreatedAt, page.After.CreatedAt, page.After.ID)
	}
	query += " ORDER BY d.created_at DESC, d.id DESC LIMIT ?"
	args = append(args, page.Limit+1)
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return httpx.Result{}, err
	}
	defer rows.Close()
	items := []dealView{}
	for rows.Next() {
		d, err := scanDeal(rows, viewerID)
		if err != nil {
			return httpx.Result{}, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return httpx.Result{}, err
	}
	var next *string
	if len(items) > page.Limit {
		items = items[:page.Limit]
		last := items[len(items)-1]
		cursor := httpx.EncodeCursor(httpx.Cursor{CreatedAt: last.createdAt, ID: last.ID})
		next = &cursor
	}
	return httpx.Result{Data: map[string]any{"items": items, "nextCursor": next}}, nil
}

func (s *Service) myDeals(r *http.Request) (httpx.Result, error) {
	me := actor(r).ID
	return s.listDeals(r, me, "p.buyer_id=?", me)
}

func (s *Service) productDeals(r *http.Request) (httpx.Result, error) {
	me, productID := actor(r).ID, chi.URLParam(r, "productId")
	product, err := loadManagedTx(r.Context(), s.db, productID)
	if isNoRows(err) || err == nil && product.sellerID != me {
		return httpx.Result{}, errProductNotFound
	}
	if err != nil {
		return httpx.Result{}, err
	}
	result, err := s.listDeals(r, me, "pr.id=?", productID)
	if err == nil {
		// The seller's application list also carries the managed product state.
		result.Data.(map[string]any)["product"] = product
	}
	return result, err
}

func (s *Service) getDeal(r *http.Request) (httpx.Result, error) {
	d, err := loadDeal(r.Context(), s.db, chi.URLParam(r, "dealId"), actor(r).ID)
	return httpx.Result{Data: d}, err
}

// transition loads the deal for one of its parties, checks the acting role,
// applies change and returns the deal's latest state, all in one transaction.
func (s *Service) transition(r *http.Request, role string, change func(*sql.Tx, dealView, time.Time) error) (httpx.Result, error) {
	ctx, id, me := r.Context(), chi.URLParam(r, "dealId"), actor(r).ID
	var out dealView
	err := s.write(ctx, func(tx *sql.Tx, now time.Time) error {
		d, err := loadDeal(ctx, tx, id, me)
		if err != nil {
			return err
		}
		if d.ViewerRole != role {
			if role == "SELLER" {
				return httpx.Error(403, "FORBIDDEN", "판매자만 처리할 수 있습니다.")
			}
			return httpx.Error(403, "FORBIDDEN", "신청한 구매자만 처리할 수 있습니다.")
		}
		if err := change(tx, d, now); err != nil {
			return err
		}
		out, err = loadDeal(ctx, tx, id, me)
		return err
	})
	return httpx.Result{Data: out}, err
}

func invalidState(message string) error { return conflict("INVALID_STATE", message, nil) }

// moveTx performs one status change only if the deal is still in from.
func moveTx(ctx context.Context, tx *sql.Tx, dealID, from, set string, args ...any) error {
	res, err := tx.ExecContext(ctx, "UPDATE deals SET "+set+" WHERE id=? AND status=?", append(args, dealID, from)...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errStateChanged
	}
	return nil
}

func (s *Service) updateDealQuantity(r *http.Request) (httpx.Result, error) {
	quantity, err := readQuantity(r)
	if err != nil {
		return httpx.Result{}, err
	}
	return s.transition(r, "BUYER", func(tx *sql.Tx, d dealView, now time.Time) error {
		if d.Status != "REQUESTED" {
			return invalidState("대기 중인 신청만 수량을 바꿀 수 있습니다.")
		}
		if quantity == d.RequestedQuantity {
			return nil
		}
		if d.Product.Hidden {
			return invalidState("숨긴 상품입니다.")
		}
		if quantity > d.Product.RemainingQuantity {
			return insufficient(d.Product.RemainingQuantity)
		}
		return moveTx(r.Context(), tx, d.ID, "REQUESTED", "requested_quantity=?,quantity_updated_at=?,updated_at=?", quantity, ts(now), ts(now))
	})
}

func (s *Service) withdraw(r *http.Request) (httpx.Result, error) {
	return s.transition(r, "BUYER", func(tx *sql.Tx, d dealView, now time.Time) error {
		switch d.Status {
		case "WITHDRAWN":
			return nil
		case "REQUESTED":
			return moveTx(r.Context(), tx, d.ID, "REQUESTED", "status='WITHDRAWN',withdrawn_at=?,updated_at=?", ts(now), ts(now))
		}
		return invalidState("대기 중인 신청만 철회할 수 있습니다.")
	})
}

func (s *Service) reject(r *http.Request) (httpx.Result, error) {
	buyerOf := func(tx *sql.Tx, conversation string) (string, error) {
		var id string
		err := tx.QueryRowContext(r.Context(), "SELECT buyer_id FROM participants WHERE id=?", conversation).Scan(&id)
		return id, err
	}
	return s.transition(r, "SELLER", func(tx *sql.Tx, d dealView, now time.Time) error {
		switch d.Status {
		case "REJECTED":
			return nil
		case "REQUESTED":
			buyer, err := buyerOf(tx, d.ConversationID)
			if err != nil {
				return err
			}
			return rejectDealTx(r.Context(), tx, d.ID, buyer, now)
		}
		return invalidState("대기 중인 신청만 거절할 수 있습니다.")
	})
}

// accept agrees the deal: stock is deducted only if enough remains, the
// current unit price and the latest requested quantity are fixed on the deal,
// and both parties' identities are revealed — together or not at all.
func (s *Service) accept(r *http.Request) (httpx.Result, error) {
	ctx := r.Context()
	return s.transition(r, "SELLER", func(tx *sql.Tx, d dealView, now time.Time) error {
		switch {
		case d.Status == "ACCEPTED":
			return nil
		case d.Status != "REQUESTED":
			return invalidState("대기 중인 신청만 수락할 수 있습니다.")
		case d.Product.Hidden:
			return invalidState("숨긴 상품의 신청은 수락할 수 없습니다.")
		}
		res, err := tx.ExecContext(ctx, "UPDATE products SET remaining_quantity=remaining_quantity-?,updated_at=? WHERE id=? AND remaining_quantity>=?", d.RequestedQuantity, ts(now), d.ProductID, d.RequestedQuantity)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return insufficient(d.Product.RemainingQuantity)
		}
		if err := moveTx(ctx, tx, d.ID, "REQUESTED", "status='ACCEPTED',agreed_unit_price=(SELECT unit_price FROM products WHERE id=?),agreed_quantity=requested_quantity,accepted_at=?,updated_at=?", d.ProductID, ts(now), ts(now)); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "UPDATE participants SET identity_revealed_at=COALESCE(identity_revealed_at,?),updated_at=? WHERE id=?", ts(now), ts(now), d.ConversationID)
		return err
	})
}

func (s *Service) complete(r *http.Request) (httpx.Result, error) {
	return s.transition(r, "SELLER", func(tx *sql.Tx, d dealView, now time.Time) error {
		switch d.Status {
		case "COMPLETED":
			return nil
		case "ACCEPTED":
			return moveTx(r.Context(), tx, d.ID, "ACCEPTED", "status='COMPLETED',completed_at=?,updated_at=?", ts(now), ts(now))
		}
		return invalidState("성사된 거래만 완료할 수 있습니다.")
	})
}
