package product

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Identity is shown only on HR_ADMIN routes.
type Identity struct {
	RealName   string `json:"realName"`
	Department string `json:"department"`
	JobLevel   string `json:"jobLevel"`
}
type AdminProduct struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	UnitPrice         int64    `json:"unitPrice"`
	RemainingQuantity int64    `json:"remainingQuantity"`
	ApprovalStatus    string   `json:"approvalStatus"`
	Seller            Identity `json:"seller"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
}
type Review struct {
	Decision  string  `json:"decision"`
	Reason    *string `json:"reason"`
	Reviewer  string  `json:"reviewer"`
	CreatedAt string  `json:"createdAt"`
}
type AdminProductDetail struct {
	AdminProduct
	Description string   `json:"description"`
	Images      []Image  `json:"images"`
	Reviews     []Review `json:"reviews"`
}

const adminColumns = `p.id,p.title,p.unit_price,p.remaining_quantity,p.approval_status,u.real_name,u.department,u.job_level,p.created_at,p.updated_at`

func scanAdmin(s scanner, extra ...any) (AdminProduct, error) {
	var p AdminProduct
	err := s.Scan(append([]any{&p.ID, &p.Title, &p.UnitPrice, &p.RemainingQuantity, &p.ApprovalStatus, &p.Seller.RealName, &p.Seller.Department, &p.Seller.JobLevel, &p.CreatedAt, &p.UpdatedAt}, extra...)...)
	return p, err
}

// adminList returns the review queue oldest first so earlier requests are handled first.
func (m *module) adminList(r *http.Request) (httpx.Result, error) {
	page, err := httpx.ParsePage(r.URL.Query())
	if err != nil {
		return httpx.Result{}, err
	}
	q := `SELECT ` + adminColumns + ` FROM products p JOIN users u ON u.id=p.seller_id WHERE p.deleted_at IS NULL`
	args := []any{}
	switch status := r.URL.Query().Get("approvalStatus"); status {
	case "":
	case "PENDING", "APPROVED", "REJECTED":
		q += ` AND p.approval_status=?`
		args = append(args, status)
	default:
		return httpx.Result{}, httpx.Error(400, "INVALID_ENUM_VALUE", "approvalStatus는 PENDING, APPROVED, REJECTED 중 하나여야 합니다.")
	}
	if page.After != nil {
		q += ` AND (p.created_at>? OR (p.created_at=? AND p.id>?))`
		args = append(args, page.After.CreatedAt, page.After.CreatedAt, page.After.ID)
	}
	q += ` ORDER BY p.created_at,p.id LIMIT ?`
	args = append(args, page.Limit+1)
	rows, err := m.db.QueryContext(r.Context(), q, args...)
	if err != nil {
		return httpx.Result{}, err
	}
	defer rows.Close()
	list := []AdminProduct{}
	for rows.Next() {
		p, err := scanAdmin(rows)
		if err != nil {
			return httpx.Result{}, err
		}
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Data: next(list, page.Limit, func(p AdminProduct) httpx.Cursor { return httpx.Cursor{CreatedAt: p.CreatedAt, ID: p.ID} })}, nil
}

func (m *module) loadAdminDetail(r *http.Request, q querier, id string) (AdminProductDetail, error) {
	var d AdminProductDetail
	var err error
	d.AdminProduct, err = scanAdmin(q.QueryRowContext(r.Context(), `SELECT `+adminColumns+`,p.description FROM products p JOIN users u ON u.id=p.seller_id WHERE p.id=? AND p.deleted_at IS NULL`, id), &d.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return d, errNotFound
	}
	if err != nil {
		return d, err
	}
	if d.Images, err = productImagesTx(r.Context(), q, id); err != nil {
		return d, err
	}
	rows, err := q.QueryContext(r.Context(), `SELECT r.decision,r.reason,u.real_name,r.created_at FROM reviews r JOIN users u ON u.id=r.reviewer_id WHERE r.product_id=? ORDER BY r.created_at,r.id`, id)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	d.Reviews = []Review{}
	for rows.Next() {
		var v Review
		var reason sql.NullString
		if err := rows.Scan(&v.Decision, &reason, &v.Reviewer, &v.CreatedAt); err != nil {
			return d, err
		}
		if reason.Valid {
			v.Reason = &reason.String
		}
		d.Reviews = append(d.Reviews, v)
	}
	return d, rows.Err()
}

func (m *module) adminDetail(r *http.Request) (httpx.Result, error) {
	d, err := m.loadAdminDetail(r, m.db, chi.URLParam(r, "productID"))
	return httpx.Result{Data: d}, err
}

func (m *module) approve(r *http.Request) (httpx.Result, error) {
	var empty struct{}
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(r, &empty); err != nil {
			return httpx.Result{}, err
		}
	}
	return m.decide(r, "APPROVED", nil)
}

func (m *module) reject(r *http.Request) (httpx.Result, error) {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return httpx.Result{}, err
	}
	reason, err := requiredText(in.Reason, "반려 사유", MaxReasonLength)
	if err != nil {
		return httpx.Result{}, err
	}
	return m.decide(r, "REJECTED", &reason)
}

// decide changes PENDING to the decision and appends the review history atomically.
func (m *module) decide(r *http.Request, decision string, reason *string) (httpx.Result, error) {
	actor, _ := httpx.CurrentUser(r.Context())
	id := chi.URLParam(r, "productID")
	ts := database.Timestamp(m.now())
	var out AdminProductDetail
	err := database.WithTx(r.Context(), m.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(r.Context(), `UPDATE products SET approval_status=?,updated_at=? WHERE id=? AND approval_status='PENDING' AND deleted_at IS NULL`, decision, ts, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			var exists bool
			if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM products WHERE id=? AND deleted_at IS NULL)`, id).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return errNotFound
			}
			return httpx.Error(409, "INVALID_STATE", "심사 대기 중인 상품만 승인하거나 반려할 수 있습니다.")
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO reviews(id,product_id,reviewer_id,decision,reason,created_at) VALUES(?,?,?,?,?,?)`, httpx.NewID(), id, actor.ID, decision, reason, ts); err != nil {
			return err
		}
		out, err = m.loadAdminDetail(r, tx, id)
		return err
	})
	return httpx.Result{Data: out}, err
}

type Participant struct {
	Label           string `json:"label"`
	InquirerNumber  *int64 `json:"inquirerNumber"`
	ApplicantNumber *int64 `json:"applicantNumber"`
	Identity
	CreatedAt string `json:"createdAt"`
}

// inquirerLabel turns 1→A, 26→Z, 27→AA like spreadsheet columns.
func inquirerLabel(n int64) string {
	s := ""
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return "문의자 " + s
}

// participants lists who inquired or applied with identities, for HR_ADMIN only.
// Message contents are intentionally not exposed.
func (m *module) participants(r *http.Request) (httpx.Result, error) {
	id := chi.URLParam(r, "productID")
	var exists bool
	if err := m.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM products WHERE id=?)`, id).Scan(&exists); err != nil {
		return httpx.Result{}, err
	}
	if !exists {
		return httpx.Result{}, errNotFound
	}
	rows, err := m.db.QueryContext(r.Context(), `SELECT pa.inquirer_number,pa.applicant_number,u.real_name,u.department,u.job_level,pa.created_at FROM participants pa JOIN users u ON u.id=pa.buyer_id WHERE pa.product_id=? ORDER BY pa.created_at,pa.id`, id)
	if err != nil {
		return httpx.Result{}, err
	}
	defer rows.Close()
	list := []Participant{}
	for rows.Next() {
		var p Participant
		var inquirer, applicant sql.NullInt64
		if err := rows.Scan(&inquirer, &applicant, &p.RealName, &p.Department, &p.JobLevel, &p.CreatedAt); err != nil {
			return httpx.Result{}, err
		}
		if inquirer.Valid {
			p.InquirerNumber = &inquirer.Int64
			p.Label = inquirerLabel(inquirer.Int64)
		}
		if applicant.Valid {
			p.ApplicantNumber = &applicant.Int64
			p.Label = "익명 " + strconv.FormatInt(applicant.Int64, 10)
		}
		list = append(list, p)
	}
	return httpx.Result{Data: map[string]any{"items": list}}, rows.Err()
}
