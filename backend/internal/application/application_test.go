package application

import (
	"context"
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
