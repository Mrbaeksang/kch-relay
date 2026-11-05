package application

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpapi"
)

type Application struct {
	HTTP *http.Server
	db   *sql.DB
}

func New(ctx context.Context, c config.Config) (*Application, error) {
	db, err := database.Open(ctx, c.DatabasePath)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(c.UploadDir, 0700); err != nil {
		db.Close()
		return nil, err
	}
	handler := httpapi.NewRouter(db, os.DirFS(c.WebDir))
	return &Application{db: db, HTTP: &http.Server{Addr: ":" + c.Port, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}}, nil
}

func (a *Application) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", a.HTTP.Addr)
	if err != nil {
		return err
	}
	result := make(chan error, 1)
	go func() { result <- a.HTTP.Serve(listener) }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.HTTP.Shutdown(shutdown); err != nil {
			a.HTTP.Close()
			return err
		}
		return nil
	}
}

func (a *Application) Close() error { a.HTTP.Close(); return a.db.Close() }
