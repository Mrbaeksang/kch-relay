// Package messages lets a buyer ask about a product anonymously and exchange
// one-to-one messages with its seller, with explicit read receipts.
package messages

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Module registers the conversation and message routes.
func Module() application.Module {
	return func(r chi.Router, deps application.Dependencies) {
		newService(deps.DB, deps.Meta, time.Now).register(r)
	}
}

type Service struct {
	db   *sql.DB
	meta httpx.MetaProvider
	now  func() time.Time
}

func newService(db *sql.DB, meta httpx.MetaProvider, now func() time.Time) *Service {
	return &Service{db: db, meta: meta, now: now}
}

func (s *Service) register(r chi.Router) {
	h := func(fn httpx.Handler) http.HandlerFunc { return httpx.Adapt(fn, s.meta) }
	r.Post("/products/{productId}/conversations", h(s.startConversation))
	r.Get("/conversations/{conversationId}/messages", h(s.listMessages))
	r.Post("/conversations/{conversationId}/messages", h(s.sendMessage))
}

// write runs fn in one transaction with the request time.
func (s *Service) write(ctx context.Context, fn func(*sql.Tx, time.Time) error) error {
	now := s.now().UTC()
	return database.WithTx(ctx, s.db, func(tx *sql.Tx) error { return fn(tx, now) })
}

func actor(r *http.Request) httpx.Actor { a, _ := httpx.CurrentUser(r.Context()); return a }
