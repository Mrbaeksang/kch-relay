package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Mrbaeksang/kch-relay/backend/migrations"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	for _, statement := range []string{"PRAGMA foreign_keys = ON", "PRAGMA busy_timeout = 5000"} {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			database.Close()
			return nil, err
		}
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, database, migrations.Files)
	if err == nil {
		_, err = provider.Up(ctx)
	}
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return database, nil
}
