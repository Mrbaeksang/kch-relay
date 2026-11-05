package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
)

func Timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

type DevelopmentAccount struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Role  string `json:"role"`
}

func DevelopmentAccounts() []DevelopmentAccount {
	return []DevelopmentAccount{{"dev-admin", "테스트 관리자", "HR_ADMIN"}, {"dev-employee-a", "테스트 직원 A", "MEMBER"}, {"dev-employee-b", "테스트 직원 B", "MEMBER"}}
}

// The application calls this only for explicitly enabled development authentication.
func EnsureDevelopmentUsers(ctx context.Context, db *sql.DB) error {
	return WithTx(ctx, db, func(tx *sql.Tx) error {
		for _, a := range DevelopmentAccounts() {
			_, err := tx.ExecContext(ctx, `INSERT INTO users(id,email,real_name,department,job_level,role) VALUES(?,?,?,'TEST_DEPT','TEST_LEVEL',?) ON CONFLICT(id) DO NOTHING`, a.Key, a.Key+"@example.invalid", a.Label, a.Role)
			if err != nil {
				return err
			}
			var email, role string
			if err := tx.QueryRowContext(ctx, "SELECT email,role FROM users WHERE id=?", a.Key).Scan(&email, &role); err != nil {
				return err
			}
			if email != a.Key+"@example.invalid" || role != a.Role {
				return fmt.Errorf("development account key collision")
			}
		}
		return nil
	})
}

type Participant struct {
	ID                              string
	InquirerNumber, ApplicantNumber sql.NullInt64
}

func EnsureParticipantTx(ctx context.Context, tx *sql.Tx, productID, buyerID, kind string, now time.Time) (Participant, error) {
	var p Participant
	if kind != "inquiry" && kind != "application" {
		return p, fmt.Errorf("unknown participation kind")
	}
	err := tx.QueryRowContext(ctx, `SELECT id,inquirer_number,applicant_number FROM participants WHERE product_id=? AND buyer_id=?`, productID, buyerID).Scan(&p.ID, &p.InquirerNumber, &p.ApplicantNumber)
	if err != nil && err != sql.ErrNoRows {
		return p, err
	}
	fresh := err == sql.ErrNoRows
	if !fresh && (kind == "application" && p.ApplicantNumber.Valid || kind == "inquiry") {
		return p, nil
	}
	var seller string
	var applicant, inquirer int64
	if err := tx.QueryRowContext(ctx, "SELECT seller_id,next_applicant_number,next_inquirer_number FROM products WHERE id=?", productID).Scan(&seller, &applicant, &inquirer); err != nil {
		return p, err
	}
	if seller == buyerID {
		return p, fmt.Errorf("seller cannot participate as buyer")
	}
	if kind == "application" {
		p.ApplicantNumber = sql.NullInt64{Int64: applicant, Valid: true}
		_, err = tx.ExecContext(ctx, "UPDATE products SET next_applicant_number=next_applicant_number+1 WHERE id=?", productID)
	} else {
		p.InquirerNumber = sql.NullInt64{Int64: inquirer, Valid: true}
		_, err = tx.ExecContext(ctx, "UPDATE products SET next_inquirer_number=next_inquirer_number+1 WHERE id=?", productID)
	}
	if err != nil {
		return p, err
	}
	if fresh {
		p.ID = httpx.NewID()
		_, err = tx.ExecContext(ctx, `INSERT INTO participants(id,product_id,buyer_id,inquirer_number,applicant_number,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, p.ID, productID, buyerID, p.InquirerNumber, p.ApplicantNumber, Timestamp(now), Timestamp(now))
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE participants SET applicant_number=?,updated_at=? WHERE id=?", p.ApplicantNumber, Timestamp(now), p.ID)
	}
	return p, err
}

func AppendRejectionNotificationTx(ctx context.Context, tx *sql.Tx, dealID, recipientID string, now time.Time) (string, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO notifications(id,recipient_id,deal_id,type,created_at)
 SELECT ?,p.buyer_id,d.id,'APPLICATION_REJECTED',? FROM deals d JOIN participants p ON p.id=d.participant_id
 WHERE d.id=? AND d.status='REJECTED' AND p.buyer_id=? ON CONFLICT(deal_id,type) DO NOTHING`, httpx.NewID(), Timestamp(now), dealID, recipientID)
	if err != nil {
		return "", err
	}
	var id string
	err = tx.QueryRowContext(ctx, "SELECT id FROM notifications WHERE deal_id=? AND recipient_id=?", dealID, recipientID).Scan(&id)
	return id, err
}
func UnreadMeta(ctx context.Context, db *sql.DB, userID string) (httpx.Meta, error) {
	var m httpx.Meta
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE recipient_id=? AND read_at IS NULL),EXISTS(SELECT 1 FROM notifications WHERE recipient_id=? AND read_at IS NULL)`, userID, userID).Scan(&m.HasUnreadMessages, &m.HasUnreadNotifications)
	return m, err
}
