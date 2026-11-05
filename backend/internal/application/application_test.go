package application

import (
	"context"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
)

func TestStartupFailureAndGracefulCancellation(t *testing.T) {
	dir := t.TempDir()
	c := config.Config{DatabasePath: dir, UploadDir: filepath.Join(dir, "uploads"), WebDir: dir, Port: "0"}
	if app, err := New(context.Background(), c); err == nil {
		app.Close()
		t.Fatal("invalid database must fail startup")
	}
	c.DatabasePath = filepath.Join(dir, "data", "market.db")
	app, err := New(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestProductionDoesNotSeedDevelopmentAccounts(t *testing.T) {
	dir := t.TempDir()
	c := config.Config{AppEnv: "production", AuthMode: "disabled", DatabasePath: filepath.Join(dir, "market.db"), UploadDir: filepath.Join(dir, "uploads"), WebDir: dir, Port: "0"}
	app, err := New(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	var count int
	if err := app.db.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil || count != 0 {
		t.Fatalf("production seed=%d err=%v", count, err)
	}
}

func TestFeatureModulesInheritAuthentication(t *testing.T) {
	dir := t.TempDir()
	c := config.Config{AppEnv: "production", AuthMode: "disabled", DatabasePath: filepath.Join(dir, "market.db"), UploadDir: filepath.Join(dir, "uploads"), WebDir: dir, Port: "0"}
	app, err := New(context.Background(), c, func(r chi.Router, dependencies Dependencies) {
		r.Get("/example", httpx.Adapt(func(r *http.Request) (httpx.Result, error) {
			return httpx.Result{Data: map[string]bool{"private": true}}, nil
		}, dependencies.Meta))
	})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	w := httptest.NewRecorder()
	app.HTTP.Handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/example", nil))
	if w.Code != 401 {
		t.Fatalf("feature module bypassed global authentication: %d", w.Code)
	}
}
