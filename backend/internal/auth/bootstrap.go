package auth

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"strings"
	"time"
)

// BootstrapAdmin is an operator-only recovery action, never an HTTP endpoint.
// It only approves an explicitly identified, pending HR member if no active admin exists.
func BootstrapAdmin(ctx context.Context, db *sql.DB, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("an exact Google account email is required")
	}
	return database.WithTx(ctx, db, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role='HR_ADMIN' AND status='ACTIVE' AND membership_status='ACTIVE'`).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return errors.New("an active administrator already exists; use member management")
		}
		result, err := tx.ExecContext(ctx, `UPDATE users SET role='HR_ADMIN',membership_status='ACTIVE',reviewed_at=?,updated_at=? WHERE email=? AND membership_status='PENDING' AND status='ACTIVE' AND department_id IN (SELECT id FROM departments WHERE is_hr=1)`, database.Timestamp(time.Now()), database.Timestamp(time.Now()), email)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return errors.New("the exact account must first sign in and submit a pending HR application")
		}
		return nil
	})
}
