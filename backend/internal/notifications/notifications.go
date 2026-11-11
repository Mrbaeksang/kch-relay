// Package notifications는 구매자 본인의 앱 내 알림함 조회와 읽음 처리를 제공한다.
// 알림 생성은 거래 거절 기능이 database.AppendRejectionNotificationTx로 같은 트랜잭션에서 담당한다.
package notifications

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Item은 알림 한 건의 공개 응답이다. 상품 제목·가격 스냅샷과 상대방 정보는 담지 않는다.
type Item struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	DealID    string  `json:"dealId"`
	ProductID string  `json:"productId"`
	CreatedAt string  `json:"createdAt"`
	ReadAt    *string `json:"readAt"`
}

type Page struct {
	Items      []Item  `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

// Module은 /me/notifications 경로를 등록한다. now가 nil이면 현재 시각을 사용한다.
func Module(now func() time.Time) application.Module {
	if now == nil {
		now = time.Now
	}
	return func(r chi.Router, deps application.Dependencies) {
		s := service{db: deps.DB, now: now}
		r.Get("/me/notifications", httpx.Adapt(s.list, deps.Meta))
		r.Post("/me/notifications/{notificationId}/read", httpx.Adapt(s.read, deps.Meta))
	}
}

type service struct {
	db  *sql.DB
	now func() time.Time
}

const columns = `n.id,n.type,n.deal_id,p.product_id,n.created_at,n.read_at
 FROM notifications n JOIN deals d ON d.id=n.deal_id JOIN participants p ON p.id=d.participant_id`

func (s service) list(r *http.Request) (httpx.Result, error) {
	actor, _ := httpx.CurrentUser(r.Context())
	page, err := httpx.ParsePage(r.URL.Query())
	if err != nil {
		return httpx.Result{}, err
	}
	query := "SELECT " + columns + " WHERE n.recipient_id=?"
	args := []any{actor.ID}
	if page.After != nil {
		query += " AND (n.created_at<? OR (n.created_at=? AND n.id<?))"
		args = append(args, page.After.CreatedAt, page.After.CreatedAt, page.After.ID)
	}
	query += " ORDER BY n.created_at DESC, n.id DESC LIMIT ?"
	args = append(args, page.Limit+1) // 한 건 더 읽어 다음 페이지 존재 여부를 판단한다
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return httpx.Result{}, err
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		it, err := scan(rows)
		if err != nil {
			return httpx.Result{}, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return httpx.Result{}, err
	}
	result := Page{Items: items}
	if len(items) > page.Limit {
		result.Items = items[:page.Limit]
		last := result.Items[page.Limit-1]
		next := httpx.EncodeCursor(httpx.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
		result.NextCursor = &next
	}
	return httpx.Result{Data: result}, nil
}

func (s service) read(r *http.Request) (httpx.Result, error) {
	actor, _ := httpx.CurrentUser(r.Context())
	id := chi.URLParam(r, "notificationId")
	// 이미 읽은 알림은 처음 읽은 시각을 유지한다. 수신자가 아니면 존재 여부도 알리지 않는다.
	res, err := s.db.ExecContext(r.Context(), `UPDATE notifications SET read_at=COALESCE(read_at,?) WHERE id=? AND recipient_id=?`, database.Timestamp(s.now()), id, actor.ID)
	if err != nil {
		return httpx.Result{}, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return httpx.Result{}, err
	} else if n == 0 {
		return httpx.Result{}, notFound()
	}
	it, err := s.find(r.Context(), id, actor.ID)
	if err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Data: it}, nil
}

func (s service) find(ctx context.Context, id, recipient string) (Item, error) {
	it, err := scan(s.db.QueryRowContext(ctx, "SELECT "+columns+" WHERE n.id=? AND n.recipient_id=?", id, recipient))
	if errors.Is(err, sql.ErrNoRows) {
		return it, notFound()
	}
	return it, err
}

type scanner interface{ Scan(...any) error }

func scan(row scanner) (Item, error) {
	var it Item
	var readAt sql.NullString
	if err := row.Scan(&it.ID, &it.Type, &it.DealID, &it.ProductID, &it.CreatedAt, &readAt); err != nil {
		return it, err
	}
	if readAt.Valid {
		it.ReadAt = &readAt.String
	}
	return it, nil
}

func notFound() error { return httpx.Error(404, "NOT_FOUND", "알림을 찾을 수 없습니다.") }
