// Package catalog exposes authenticated, anonymous product reads.
package catalog

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/Mrbaeksang/kch-relay/backend/internal/transactions"
	"github.com/go-chi/chi/v5"
)

type Image struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	SortOrder int    `json:"sortOrder"`
}
type Seller struct {
	Label string `json:"label"`
}
type Product struct {
	ID                string  `json:"id"`
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	UnitPrice         int64   `json:"unitPrice"`
	RemainingQuantity int64   `json:"remainingQuantity"`
	RecruitmentClosed bool    `json:"recruitmentClosed"`
	CanApply          bool    `json:"canApply"`
	Seller            Seller  `json:"seller"`
	Images            []Image `json:"images"`
	CreatedAt         string  `json:"createdAt"`
}
type List struct {
	Items      []Product `json:"items"`
	NextCursor *string   `json:"nextCursor"`
}

func Register(r chi.Router, d application.Dependencies) {
	s := service{d}
	r.Get("/products", httpx.Adapt(s.list, d.Meta))
	r.Get("/products/{productId}", httpx.Adapt(s.detail, d.Meta))
}

type service struct{ application.Dependencies }

const columns = `id,title,description,unit_price,remaining_quantity,recruitment_closed,created_at,seller_id,approval_status,deleted_at`

type scanner interface{ Scan(...any) error }

func scan(row scanner, viewer string) (Product, error) {
	var p Product
	var seller, status string
	var deleted sql.NullString
	err := row.Scan(&p.ID, &p.Title, &p.Description, &p.UnitPrice, &p.RemainingQuantity, &p.RecruitmentClosed, &p.CreatedAt, &seller, &status, &deleted)
	p.Seller = Seller{Label: "판매자"}
	p.Images = []Image{}
	// Pending-application duplicate rules remain in the deal command's policy.
	p.CanApply = status == "APPROVED" && !deleted.Valid && !p.RecruitmentClosed && p.RemainingQuantity > 0 && seller != viewer
	return p, err
}
func images(r *http.Request, tx *sql.Tx, p *Product) error {
	rows, err := tx.QueryContext(r.Context(), `SELECT id,sort_order FROM images WHERE product_id=? ORDER BY sort_order,id`, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var i Image
		if err := rows.Scan(&i.ID, &i.SortOrder); err != nil {
			return err
		}
		i.URL = "/api/v1/images/" + url.PathEscape(i.ID)
		p.Images = append(p.Images, i)
	}
	return rows.Err()
}
func (s service) list(r *http.Request) (httpx.Result, error) {
	page, err := httpx.ParsePage(r.URL.Query())
	if err != nil {
		return httpx.Result{}, err
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) > 200 {
		return httpx.Result{}, httpx.Invalid("검색어는 200자 이내로 입력해 주세요.")
	}
	actor, _ := httpx.CurrentUser(r.Context())
	result := List{Items: []Product{}}
	err = database.WithTx(r.Context(), s.DB, func(tx *sql.Tx) error {
		query := `SELECT ` + columns + ` FROM products WHERE approval_status='APPROVED' AND deleted_at IS NULL`
		args := []any{}
		if q != "" {
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
			query += ` AND (title LIKE ? ESCAPE '\' OR description LIKE ? ESCAPE '\')`
			args = append(args, "%"+escaped+"%", "%"+escaped+"%")
		}
		if page.After != nil {
			query += ` AND (created_at < ? OR (created_at = ? AND id < ?))`
			args = append(args, page.After.CreatedAt, page.After.CreatedAt, page.After.ID)
		}
		query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
		args = append(args, page.Limit+1)
		rows, err := tx.QueryContext(r.Context(), query, args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			p, err := scan(rows, actor.ID)
			if err != nil {
				rows.Close()
				return err
			}
			result.Items = append(result.Items, p)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(result.Items) > page.Limit {
			result.Items = result.Items[:page.Limit]
			last := result.Items[len(result.Items)-1]
			cursor := httpx.EncodeCursor(httpx.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
			result.NextCursor = &cursor
		}
		for i := range result.Items {
			result.Items[i].CanApply, err = transactions.CanApply(r.Context(), tx, result.Items[i].ID, actor.ID)
			if err != nil {
				return err
			}
			if err := images(r, tx, &result.Items[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return httpx.Result{Data: result}, err
}
func (s service) detail(r *http.Request) (httpx.Result, error) {
	actor, _ := httpx.CurrentUser(r.Context())
	var result Product
	err := database.WithTx(r.Context(), s.DB, func(tx *sql.Tx) error {
		var err error
		result, err = scan(tx.QueryRowContext(r.Context(), `SELECT `+columns+` FROM products WHERE id=? AND ((approval_status='APPROVED' AND deleted_at IS NULL) OR seller_id=?)`, chi.URLParam(r, "productId"), actor.ID), actor.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.Error(404, "NOT_FOUND", "상품을 찾을 수 없습니다.")
		}
		if err != nil {
			return err
		}
		result.CanApply, err = transactions.CanApply(r.Context(), tx, result.ID, actor.ID)
		if err != nil {
			return err
		}
		return images(r, tx, &result)
	})
	return httpx.Result{Data: result}, err
}
