package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Mrbaeksang/kch-relay/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpapi"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
)

type Application struct {
	HTTP *http.Server
	db   *sql.DB
}

type Dependencies struct {
	DB     *sql.DB
	Config config.Config
	Meta   httpx.MetaProvider
}
type Module func(chi.Router, Dependencies)

func New(ctx context.Context, c config.Config, modules ...Module) (*Application, error) {
	if c.AuthMode == "development" && c.AppEnv != "development" {
		return nil, fmt.Errorf("development authentication is forbidden outside development")
	}
	db, err := database.Open(ctx, c.DatabasePath)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(c.UploadDir, 0700); err != nil {
		db.Close()
		return nil, err
	}
	if c.AuthMode == "development" {
		if err := database.EnsureDevelopmentUsers(ctx, db); err != nil {
			db.Close()
			return nil, err
		}
	}
	authentication, err := auth.New(db, c, nil)
	if err != nil {
		db.Close()
		return nil, err
	}
	handler := httpapi.NewRouter(db, os.DirFS(c.WebDir), func(r chi.Router) {
		r.Route("/api/v1", func(api chi.Router) {
			api.Use(authentication.Authenticate)
			authentication.Register(api)
			api.Group(func(protected chi.Router) {
				protected.Use(httpx.RequireUserMiddleware)
				deps := Dependencies{DB: db, Config: c, Meta: func(ctx context.Context, user string) (httpx.Meta, error) { return database.UnreadMeta(ctx, db, user) }}
				for _, register := range modules {
					register(protected, deps)
				}
			})
		})
	})
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
