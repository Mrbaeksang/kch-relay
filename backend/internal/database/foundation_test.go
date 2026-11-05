package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Mrbaeksang/kch-relay/backend/migrations"
	"github.com/pressly/goose/v3"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func TestDevelopmentSeedAndRollback(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := EnsureDevelopmentUsers(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDevelopmentUsers(ctx, db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM users WHERE google_subject IS NULL").Scan(&n); err != nil || n != 3 {
		t.Fatalf("seed=%d err=%v", n, err)
	}
	sentinel := errors.New("stop")
	err = WithTx(ctx, db, func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE users SET status='SUSPENDED' WHERE id=?", "dev-employee-a")
		if err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	var status string
	if err := db.QueryRow("SELECT status FROM users WHERE id=?", "dev-employee-a").Scan(&status); err != nil || status != "ACTIVE" {
		t.Fatalf("rollback status=%s err=%v", status, err)
	}
}

func TestApplicationHasNoAgreedValuesBeforeAcceptance(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := EnsureDevelopmentUsers(ctx, db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO products(id,seller_id,title,description,unit_price,remaining_quantity,created_at,updated_at) VALUES('product','dev-admin','상품','설명',10000,5,'now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	err = WithTx(ctx, db, func(tx *sql.Tx) error {
		p, err := EnsureParticipantTx(ctx, tx, "product", "dev-employee-a", "application", time.Now())
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO deals(id,participant_id,status,requested_quantity,requested_at,created_at,updated_at) VALUES('deal',?,'REQUESTED',2,'now','now','now')`, p.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeKeepsExistingUsers(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	content, err := migrations.Files.ReadFile("00001_users.sql")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, old, fstest.MapFS{"00001_users.sql": &fstest.MapFile{Data: content}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`INSERT INTO users(id,google_subject,email,real_name,department,job_level) VALUES('existing','google-subject','person@example.invalid','기존 직원','DEPT','LEVEL')`); err != nil {
		t.Fatal(err)
	}
	old.Close()
	upgraded, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	var name, subject string
	if err := upgraded.QueryRow("SELECT real_name,google_subject FROM users WHERE id='existing'").Scan(&name, &subject); err != nil || name != "기존 직원" || subject != "google-subject" {
		t.Fatalf("name=%s subject=%s err=%v", name, subject, err)
	}
	if err := EnsureDevelopmentUsers(ctx, upgraded); err != nil {
		t.Fatal(err)
	}
}
