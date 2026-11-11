package transactions

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// managedProduct is the seller's own view of a product they manage.
type managedProduct struct {
	ID                      string   `json:"id"`
	Title                   string   `json:"title"`
	Description             string   `json:"description"`
	UnitPrice               int64    `json:"unitPrice"`
	RemainingQuantity       int64    `json:"remainingQuantity"`
	RecruitmentClosed       bool     `json:"recruitmentClosed"`
	ApprovalStatus          string   `json:"approvalStatus"`
	Hidden                  bool     `json:"hidden"`
	ImageIDs                []string `json:"imageIds"`
	PendingApplicationCount int64    `json:"pendingApplicationCount"`
	UpdatedAt               string   `json:"updatedAt"`
	sellerID                string
}

type reader interface {
	queryer
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadManagedTx(ctx context.Context, tx reader, productID string) (managedProduct, error) {
	var p managedProduct
	var deleted sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT id,seller_id,title,description,unit_price,remaining_quantity,recruitment_closed,approval_status,deleted_at,updated_at,
 (SELECT count(*) FROM deals d JOIN participants pa ON pa.id=d.participant_id WHERE pa.product_id=products.id AND d.status='REQUESTED')
 FROM products WHERE id=?`, productID).Scan(&p.ID, &p.sellerID, &p.Title, &p.Description, &p.UnitPrice, &p.RemainingQuantity, &p.RecruitmentClosed, &p.ApprovalStatus, &deleted, &p.UpdatedAt, &p.PendingApplicationCount)
	if err != nil {
		return p, err
	}
	p.Hidden = deleted.Valid
	rows, err := tx.QueryContext(ctx, "SELECT id FROM images WHERE product_id=? ORDER BY sort_order,id", productID)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	p.ImageIDs = []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return p, err
		}
		p.ImageIDs = append(p.ImageIDs, id)
	}
	return p, rows.Err()
}

// ownedTx loads a product the actor sells. Other users get 404 so a product
// ID alone reveals nothing. Hidden products accept no further changes.
func ownedTx(ctx context.Context, tx *sql.Tx, productID, sellerID string, allowHidden bool) (managedProduct, error) {
	p, err := loadManagedTx(ctx, tx, productID)
	if isNoRows(err) || err == nil && p.sellerID != sellerID {
		return p, errProductNotFound
	}
	if err == nil && p.Hidden && !allowHidden {
		return p, conflict("INVALID_STATE", "숨긴 상품은 변경할 수 없습니다.", nil)
	}
	return p, err
}

// manage runs a seller-only change and returns the product's latest state.
func (s *Service) manage(r *http.Request, change func(*sql.Tx, managedProduct, time.Time) error) (httpx.Result, error) {
	id, seller := chi.URLParam(r, "productId"), actor(r).ID
	var out managedProduct
	err := s.write(r.Context(), func(tx *sql.Tx, now time.Time) error {
		p, err := ownedTx(r.Context(), tx, id, seller, false)
		if err != nil {
			return err
		}
		if err := change(tx, p, now); err != nil {
			return err
		}
		out, err = loadManagedTx(r.Context(), tx, id)
		return err
	})
	return httpx.Result{Data: out}, err
}

type productEdit struct {
	Title                     *string   `json:"title"`
	Description               *string   `json:"description"`
	UnitPrice                 *int64    `json:"unitPrice"`
	ImageIDs                  *[]string `json:"imageIds"`
	RejectPendingApplications bool      `json:"rejectPendingApplications"`
}

func checkText(value *string, field string, max int) error {
	if value == nil {
		return nil
	}
	*value = strings.TrimSpace(*value)
	if *value == "" {
		return httpx.Invalid(field + "을 입력해 주세요.")
	}
	if utf8.RuneCountInString(*value) > max {
		return httpx.Invalid(fmt.Sprintf("%s은 %d자 이하로 입력해 주세요.", field, max))
	}
	return nil
}

func (in *productEdit) validate() error {
	if err := checkText(in.Title, "제목", MaxTitleLength); err != nil {
		return err
	}
	if err := checkText(in.Description, "내용", MaxDescriptionLength); err != nil {
		return err
	}
	if in.UnitPrice != nil && (*in.UnitPrice < 0 || *in.UnitPrice > MaxUnitPrice) {
		return httpx.Invalid(fmt.Sprintf("가격은 0원 이상 %d원 이하로 입력해 주세요.", MaxUnitPrice))
	}
	if in.ImageIDs != nil {
		for _, id := range *in.ImageIDs {
			if id == "" {
				return httpx.Invalid("사진 정보가 올바르지 않습니다.")
			}
		}
	}
	return nil
}

// updateProduct changes content, price or photos. Waiting applications were
// made for the old content, so the edit, their rejection and every applicant's
// notification commit together, or the edit is refused while any remain.
func (s *Service) updateProduct(r *http.Request) (httpx.Result, error) {
	var in productEdit
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return httpx.Result{}, err
	}
	if err := in.validate(); err != nil {
		return httpx.Result{}, err
	}
	return s.manage(r, func(tx *sql.Tx, p managedProduct, now time.Time) error {
		next := p
		if in.Title != nil {
			next.Title = *in.Title
		}
		if in.Description != nil {
			next.Description = *in.Description
		}
		if in.UnitPrice != nil {
			next.UnitPrice = *in.UnitPrice
		}
		photos := in.ImageIDs != nil && !slices.Equal(*in.ImageIDs, p.ImageIDs)
		if next.Title == p.Title && next.Description == p.Description && next.UnitPrice == p.UnitPrice && !photos {
			return nil
		}
		if photos && s.attach == nil {
			return httpx.Invalid("사진 변경은 아직 지원하지 않습니다.")
		}
		if p.PendingApplicationCount > 0 {
			if !in.RejectPendingApplications {
				return conflict("PENDING_APPLICATIONS_EXIST", "대기 중인 신청을 거절한 후 수정할 수 있습니다.", map[string]int64{"pendingCount": p.PendingApplicationCount})
			}
			if err := rejectPendingTx(r.Context(), tx, p.ID, now); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(r.Context(), "UPDATE products SET title=?,description=?,unit_price=?,updated_at=? WHERE id=?", next.Title, next.Description, next.UnitPrice, ts(now), p.ID); err != nil {
			return err
		}
		if photos {
			return s.attach(r.Context(), tx, p.ID, p.sellerID, *in.ImageIDs, now)
		}
		return nil
	})
}

// updateRemainingQuantity keeps every application: one larger than the new
// stock stays waiting and is checked again when the seller accepts it.
func (s *Service) updateRemainingQuantity(r *http.Request) (httpx.Result, error) {
	var in struct {
		RemainingQuantity *int64 `json:"remainingQuantity"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return httpx.Result{}, err
	}
	if in.RemainingQuantity == nil || *in.RemainingQuantity < 0 || *in.RemainingQuantity > MaxQuantity {
		return httpx.Result{}, httpx.Invalid(fmt.Sprintf("잔여 수량은 0 이상 %d 이하로 입력해 주세요.", MaxQuantity))
	}
	return s.manage(r, func(tx *sql.Tx, p managedProduct, now time.Time) error {
		_, err := tx.ExecContext(r.Context(), "UPDATE products SET remaining_quantity=?,updated_at=? WHERE id=?", *in.RemainingQuantity, ts(now), p.ID)
		return err
	})
}

func (s *Service) setRecruitment(closed bool) httpx.Handler {
	return func(r *http.Request) (httpx.Result, error) {
		return s.manage(r, func(tx *sql.Tx, p managedProduct, now time.Time) error {
			if p.RecruitmentClosed == closed {
				return nil
			}
			_, err := tx.ExecContext(r.Context(), "UPDATE products SET recruitment_closed=?,updated_at=? WHERE id=?", closed, ts(now), p.ID)
			return err
		})
	}
}

func (s *Service) closeRecruitment(r *http.Request) (httpx.Result, error) {
	return s.setRecruitment(true)(r)
}
func (s *Service) reopenRecruitment(r *http.Request) (httpx.Result, error) {
	return s.setRecruitment(false)(r)
}

// hideProduct removes the product from the market but keeps all records.
// Waiting applications can no longer be accepted, so they are rejected and
// their applicants notified in the same transaction.
func (s *Service) hideProduct(r *http.Request) (httpx.Result, error) {
	id, seller := chi.URLParam(r, "productId"), actor(r).ID
	err := s.write(r.Context(), func(tx *sql.Tx, now time.Time) error {
		p, err := ownedTx(r.Context(), tx, id, seller, true)
		if err != nil || p.Hidden {
			return err
		}
		if err := rejectPendingTx(r.Context(), tx, id, now); err != nil {
			return err
		}
		_, err = tx.ExecContext(r.Context(), "UPDATE products SET deleted_at=?,updated_at=? WHERE id=?", ts(now), ts(now), id)
		return err
	})
	if err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Data: map[string]bool{"deleted": true}}, nil
}
