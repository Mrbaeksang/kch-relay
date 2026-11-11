package product

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

type module struct {
	db         *sql.DB
	uploadDir  string
	now        func() time.Time
	uploadSlot chan struct{}
}

// Module registers product, photo and review routes on the authenticated API router.
func Module(r chi.Router, d application.Dependencies) {
	m := &module{db: d.DB, uploadDir: d.Config.UploadDir, now: time.Now, uploadSlot: make(chan struct{}, 1)}
	a := func(h httpx.Handler) http.HandlerFunc { return httpx.Adapt(h, d.Meta) }
	admin := func(h httpx.Handler) http.HandlerFunc { return a(httpx.RequireAdmin(h)) }

	r.Get("/product-limits", a(func(*http.Request) (httpx.Result, error) { return httpx.Result{Data: CurrentLimits()}, nil }))
	r.Post("/images", a(m.upload))
	r.Get("/images/{imageID}", m.serveImage)
	r.Post("/products", a(m.create))
	r.Get("/me/products", a(m.myProducts))
	r.Post("/products/{productID}/submit-review", a(m.resubmit))
	r.Get("/admin/products", admin(m.adminList))
	r.Get("/admin/products/{productID}", admin(m.adminDetail))
	r.Post("/admin/products/{productID}/approve", admin(m.approve))
	r.Post("/admin/products/{productID}/reject", admin(m.reject))
	r.Get("/admin/products/{productID}/participants", admin(m.participants))
}

var errNotFound = httpx.Error(404, "NOT_FOUND", "상품을 찾을 수 없습니다.")

type input struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	UnitPrice   json.Number `json:"unitPrice"`
	Quantity    json.Number `json:"quantity"`
	ImageIDs    []string    `json:"imageIds"`
}
type fields struct {
	title, description string
	unitPrice, qty     int64
	imageIDs           []string
}

func parseInteger(n json.Number, label string, min, max int64) (int64, error) {
	if n == "" {
		return 0, httpx.Invalid(label + "을(를) 입력해 주세요.")
	}
	v, err := strconv.ParseInt(string(n), 10, 64)
	if err != nil || v < min || v > max {
		return 0, httpx.Invalid(label + "은(는) " + strconv.FormatInt(min, 10) + " 이상 " + strconv.FormatInt(max, 10) + " 이하의 정수여야 합니다.")
	}
	return v, nil
}
func requiredText(s, label string, max int) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", httpx.Invalid(label + "을(를) 입력해 주세요.")
	}
	if utf8.RuneCountInString(s) > max {
		return "", httpx.Invalid(label + "은(는) " + strconv.Itoa(max) + "자 이하로 입력해 주세요.")
	}
	return s, nil
}

func readFields(r *http.Request) (fields, error) {
	var in input
	var f fields
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return f, err
	}
	var err error
	if f.title, err = requiredText(in.Title, "제목", MaxTitleLength); err != nil {
		return f, err
	}
	if f.description, err = requiredText(in.Description, "본문", MaxDescriptionLength); err != nil {
		return f, err
	}
	if f.unitPrice, err = parseInteger(in.UnitPrice, "단가", 0, MaxUnitPrice); err != nil {
		return f, err
	}
	if f.qty, err = parseInteger(in.Quantity, "수량", 1, MaxQuantity); err != nil {
		return f, err
	}
	f.imageIDs = in.ImageIDs
	if f.imageIDs == nil {
		f.imageIDs = []string{}
	}
	if len(f.imageIDs) > MaxImagesPerProduct {
		return f, httpx.Invalid("사진은 최대 " + strconv.Itoa(MaxImagesPerProduct) + "장까지 등록할 수 있습니다.")
	}
	return f, nil
}

// SellerProduct is the seller's own view. It never contains identities.
type SellerProduct struct {
	ID                string  `json:"id"`
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	UnitPrice         int64   `json:"unitPrice"`
	RemainingQuantity int64   `json:"remainingQuantity"`
	ApprovalStatus    string  `json:"approvalStatus"`
	RejectionReason   *string `json:"rejectionReason"`
	RecruitmentClosed bool    `json:"recruitmentClosed"`
	Images            []Image `json:"images"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

const sellerColumns = `p.id,p.title,p.description,p.unit_price,p.remaining_quantity,p.approval_status,
 CASE WHEN p.approval_status='REJECTED' THEN (SELECT reason FROM reviews r WHERE r.product_id=p.id ORDER BY r.created_at DESC,r.id DESC LIMIT 1) END,
 p.recruitment_closed,p.created_at,p.updated_at`

type scanner interface{ Scan(...any) error }

func scanSeller(s scanner) (SellerProduct, error) {
	var p SellerProduct
	var reason sql.NullString
	err := s.Scan(&p.ID, &p.Title, &p.Description, &p.UnitPrice, &p.RemainingQuantity, &p.ApprovalStatus, &reason, &p.RecruitmentClosed, &p.CreatedAt, &p.UpdatedAt)
	if reason.Valid {
		p.RejectionReason = &reason.String
	}
	return p, err
}

type querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadSellerProduct(ctx context.Context, q querier, id string) (SellerProduct, error) {
	p, err := scanSeller(q.QueryRowContext(ctx, `SELECT `+sellerColumns+` FROM products p WHERE p.id=?`, id))
	if err != nil {
		return p, err
	}
	p.Images, err = productImagesTx(ctx, q, id)
	return p, err
}

func (m *module) create(r *http.Request) (httpx.Result, error) {
	actor, _ := httpx.CurrentUser(r.Context())
	f, err := readFields(r)
	if err != nil {
		return httpx.Result{}, err
	}
	id, ts := httpx.NewID(), database.Timestamp(m.now())
	var out SellerProduct
	err = database.WithTx(r.Context(), m.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO products(id,seller_id,title,description,unit_price,remaining_quantity,approval_status,created_at,updated_at) VALUES(?,?,?,?,?,?,'PENDING',?,?)`, id, actor.ID, f.title, f.description, f.unitPrice, f.qty, ts, ts); err != nil {
			return err
		}
		if err := AttachImagesTx(r.Context(), tx, id, actor.ID, f.imageIDs); err != nil {
			return err
		}
		out, err = loadSellerProduct(r.Context(), tx, id)
		return err
	})
	if err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Status: 201, Data: out}, nil
}

// resubmit lets the seller fix a rejected product and request review again in one step.
func (m *module) resubmit(r *http.Request) (httpx.Result, error) {
	actor, _ := httpx.CurrentUser(r.Context())
	id := chi.URLParam(r, "productID")
	f, err := readFields(r)
	if err != nil {
		return httpx.Result{}, err
	}
	var out SellerProduct
	err = database.WithTx(r.Context(), m.db, func(tx *sql.Tx) error {
		var seller, status string
		var deleted sql.NullString
		err := tx.QueryRowContext(r.Context(), `SELECT seller_id,approval_status,deleted_at FROM products WHERE id=?`, id).Scan(&seller, &status, &deleted)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (seller != actor.ID || deleted.Valid)) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if status != "REJECTED" {
			return httpx.Error(409, "INVALID_STATE", "반려된 상품만 다시 심사를 요청할 수 있습니다.")
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE products SET title=?,description=?,unit_price=?,remaining_quantity=?,approval_status='PENDING',updated_at=? WHERE id=? AND approval_status='REJECTED'`, f.title, f.description, f.unitPrice, f.qty, database.Timestamp(m.now()), id); err != nil {
			return err
		}
		if err := AttachImagesTx(r.Context(), tx, id, actor.ID, f.imageIDs); err != nil {
			return err
		}
		out, err = loadSellerProduct(r.Context(), tx, id)
		return err
	})
	if err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Data: out}, nil
}

type pageResult[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

func next[T any](rows []T, limit int, cursor func(T) httpx.Cursor) pageResult[T] {
	out := pageResult[T]{Items: rows}
	if len(rows) > limit {
		out.Items = rows[:limit]
		c := httpx.EncodeCursor(cursor(rows[limit-1]))
		out.NextCursor = &c
	}
	return out
}

func (m *module) myProducts(r *http.Request) (httpx.Result, error) {
	actor, _ := httpx.CurrentUser(r.Context())
	page, err := httpx.ParsePage(r.URL.Query())
	if err != nil {
		return httpx.Result{}, err
	}
	q := `SELECT ` + sellerColumns + ` FROM products p WHERE p.seller_id=? AND p.deleted_at IS NULL`
	args := []any{actor.ID}
	if page.After != nil {
		q += ` AND (p.created_at<? OR (p.created_at=? AND p.id<?))`
		args = append(args, page.After.CreatedAt, page.After.CreatedAt, page.After.ID)
	}
	q += ` ORDER BY p.created_at DESC,p.id DESC LIMIT ?`
	args = append(args, page.Limit+1)
	rows, err := m.db.QueryContext(r.Context(), q, args...)
	if err != nil {
		return httpx.Result{}, err
	}
	list := []SellerProduct{}
	for rows.Next() {
		p, err := scanSeller(rows)
		if err != nil {
			rows.Close()
			return httpx.Result{}, err
		}
		list = append(list, p)
	}
	rows.Close() // the single SQLite connection must be free before loading images
	if err := rows.Err(); err != nil {
		return httpx.Result{}, err
	}
	for i := range list {
		if list[i].Images, err = productImagesTx(r.Context(), m.db, list[i].ID); err != nil {
			return httpx.Result{}, err
		}
	}
	return httpx.Result{Data: next(list, page.Limit, func(p SellerProduct) httpx.Cursor { return httpx.Cursor{CreatedAt: p.CreatedAt, ID: p.ID} })}, nil
}
