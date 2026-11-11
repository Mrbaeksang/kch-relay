package messages

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Input limits owned by this feature (docs/decisions/anonymous-messages.md).
const (
	MaxMessageLength        = 2000 // characters (Unicode code points)
	MaxIdempotencyKeyLength = 128
)

var errKeyReused = httpx.Error(409, "IDEMPOTENCY_KEY_REUSED", "같은 요청 키로 다른 내용을 보낼 수 없습니다. 새로고침 후 다시 보내 주세요.")

type senderView struct {
	Kind     string    `json:"kind"` // SELF or COUNTERPARTY
	Label    string    `json:"label"`
	Identity *identity `json:"identity"`
}

type messageView struct {
	ID        string     `json:"id"`
	Body      string     `json:"body"`
	CreatedAt string     `json:"createdAt"`
	ReadAt    *string    `json:"readAt"`
	Sender    senderView `json:"sender"`
}

func validKey(key string) bool {
	if key == "" || len(key) > MaxIdempotencyKeyLength {
		return false
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 0x21 || key[i] > 0x7e {
			return false
		}
	}
	return true
}

func validateBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return httpx.Invalid("쪽지 내용을 입력해 주세요.")
	}
	if utf8.RuneCountInString(body) > MaxMessageLength {
		return &httpx.AppError{Status: 400, Code: "VALIDATION_ERROR", Message: "쪽지는 2,000자까지 보낼 수 있습니다.", Details: map[string]int{"maxLength": MaxMessageLength}}
	}
	return nil
}

// nextCreatedAt keeps message times strictly increasing within a conversation,
// so "read through this message" never covers a message that arrived later in
// the same millisecond.
func nextCreatedAt(ctx context.Context, tx *sql.Tx, conversationID string, now time.Time) (string, error) {
	var last sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT MAX(created_at) FROM messages WHERE participant_id=?", conversationID).Scan(&last); err != nil {
		return "", err
	}
	at := now.UTC().Truncate(time.Millisecond)
	if last.Valid {
		prev, err := time.Parse(time.RFC3339Nano, last.String)
		if err != nil {
			return "", err
		}
		if !at.After(prev) {
			at = prev.Add(time.Millisecond)
		}
	}
	return database.Timestamp(at), nil
}

func otherParty(c conversation, user string) string {
	if c.viewerIsSeller(user) {
		return c.BuyerID
	}
	return c.SellerID
}

// sendMessage stores a message from one party to the other. The server picks
// the recipient from the conversation; a retried request with the same
// Idempotency-Key returns the first message instead of storing a second one.
func (s *Service) sendMessage(r *http.Request) (httpx.Result, error) {
	user := actor(r).ID
	key := r.Header.Get("Idempotency-Key")
	if !validKey(key) {
		return httpx.Result{}, httpx.Invalid("Idempotency-Key 헤더는 1~128자의 영문·숫자·기호여야 합니다.")
	}
	var in struct {
		Body string `json:"body"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return httpx.Result{}, err
	}
	if err := validateBody(in.Body); err != nil {
		return httpx.Result{}, err
	}
	out := messageView{Sender: senderView{Kind: "SELF", Label: selfLabel}}
	status := http.StatusCreated
	err := s.write(r.Context(), func(tx *sql.Tx, now time.Time) error {
		ctx := r.Context()
		c, err := loadConversation(ctx, tx, chi.URLParam(r, "conversationId"), user)
		if err != nil {
			return err
		}
		var readAt sql.NullString
		err = tx.QueryRowContext(ctx, "SELECT id,body,created_at,read_at FROM messages WHERE participant_id=? AND sender_id=? AND idempotency_key=?", c.ID, user, key).
			Scan(&out.ID, &out.Body, &out.CreatedAt, &readAt)
		if err == nil {
			if out.Body != in.Body {
				return errKeyReused
			}
			status = http.StatusOK
			if readAt.Valid {
				out.ReadAt = &readAt.String
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		created, err := nextCreatedAt(ctx, tx, c.ID, now)
		if err != nil {
			return err
		}
		out.ID, out.Body, out.CreatedAt = httpx.NewID(), in.Body, created
		_, err = tx.ExecContext(ctx, "INSERT INTO messages(id,participant_id,sender_id,recipient_id,body,created_at,idempotency_key) VALUES(?,?,?,?,?,?,?)",
			out.ID, c.ID, user, otherParty(c, user), in.Body, created, key)
		return err
	})
	if err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Status: status, Data: out}, nil
}

// listMessages returns one page, newest first. Reading never marks anything
// read; the screen calls the read endpoint after showing the messages.
func (s *Service) listMessages(r *http.Request) (httpx.Result, error) {
	user := actor(r).ID
	page, err := httpx.ParsePage(r.URL.Query())
	if err != nil {
		return httpx.Result{}, err
	}
	ctx := r.Context()
	c, err := loadConversation(ctx, s.db, chi.URLParam(r, "conversationId"), user)
	if err != nil {
		return httpx.Result{}, err
	}
	other, err := counterparty(ctx, s.db, c, user)
	if err != nil {
		return httpx.Result{}, err
	}
	query := "SELECT id,sender_id,body,created_at,read_at FROM messages WHERE participant_id=?"
	args := []any{c.ID}
	if page.After != nil {
		query += " AND (created_at<? OR (created_at=? AND id<?))"
		args = append(args, page.After.CreatedAt, page.After.CreatedAt, page.After.ID)
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, page.Limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return httpx.Result{}, err
	}
	defer rows.Close()
	items := []messageView{}
	for rows.Next() {
		var m messageView
		var sender string
		var readAt sql.NullString
		if err := rows.Scan(&m.ID, &sender, &m.Body, &m.CreatedAt, &readAt); err != nil {
			return httpx.Result{}, err
		}
		if readAt.Valid {
			m.ReadAt = &readAt.String
		}
		if sender == user {
			m.Sender = senderView{Kind: "SELF", Label: selfLabel}
		} else {
			m.Sender = senderView{Kind: "COUNTERPARTY", Label: other.Label, Identity: other.Identity}
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return httpx.Result{}, err
	}
	var next *string
	if len(items) > page.Limit {
		items = items[:page.Limit]
		last := items[len(items)-1]
		cursor := httpx.EncodeCursor(httpx.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
		next = &cursor
	}
	return httpx.Result{Data: map[string]any{"items": items, "nextCursor": next}}, nil
}

var errForeignMessage = httpx.Invalid("읽음 기준 쪽지가 이 대화에 없습니다.")

// markRead marks the viewer's received messages read up to and including the
// given message (by time, then ID). Messages that arrive later stay unread
// even when the request races with them.
func (s *Service) markRead(r *http.Request) (httpx.Result, error) {
	user := actor(r).ID
	var in struct {
		ThroughMessageID string `json:"throughMessageId"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return httpx.Result{}, err
	}
	var updated int64
	err := s.write(r.Context(), func(tx *sql.Tx, now time.Time) error {
		ctx := r.Context()
		c, err := loadConversation(ctx, tx, chi.URLParam(r, "conversationId"), user)
		if err != nil {
			return err
		}
		var throughAt string
		err = tx.QueryRowContext(ctx, "SELECT created_at FROM messages WHERE id=? AND participant_id=?", in.ThroughMessageID, c.ID).Scan(&throughAt)
		if errors.Is(err, sql.ErrNoRows) {
			return errForeignMessage
		}
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE messages SET read_at=? WHERE participant_id=? AND recipient_id=? AND read_at IS NULL
 AND (created_at<? OR (created_at=? AND id<=?))`, database.Timestamp(now), c.ID, user, throughAt, throughAt, in.ThroughMessageID)
		if err != nil {
			return err
		}
		updated, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Data: map[string]int64{"updatedCount": updated}}, nil
}
