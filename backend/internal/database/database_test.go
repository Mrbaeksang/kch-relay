package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrationAndDataSurviveReopening(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "market.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO users (id,google_subject,email,real_name,department,job_level) VALUES (?,?,?,?,?,?)`, "fake-user", "fake-subject", "person@example.invalid", "가상 직원", "TEST_DEPT", "TEST_LEVEL")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name string
	if err := db.QueryRowContext(ctx, "SELECT real_name FROM users WHERE id = ?", "fake-user").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "가상 직원" {
		t.Fatalf("unexpected name %q", name)
	}
	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys=%d err=%v", foreignKeys, err)
	}
}
