// Package transactions lets sellers manage their products and lets buyers and
// sellers move a deal from application to completion.
package transactions

import (
	"context"
	"database/sql"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/go-chi/chi/v5"
)

// ImageAttacher is the photo contract supplied by the product registration
// feature (AttachImagesTx): inside the caller's transaction it validates the
// seller's image IDs and replaces the product's linked images. It never commits.
type ImageAttacher func(ctx context.Context, tx *sql.Tx, productID, sellerID string, imageIDs []string, now time.Time) error

// Module registers the product management and deal routes. attach may be nil
// until the photo supplier is connected; photo changes are then refused.
func Module(attach ImageAttacher) application.Module {
	return func(r chi.Router, deps application.Dependencies) {
		s := &Service{db: deps.DB, meta: deps.Meta, now: time.Now, attach: attach}
		s.register(r)
	}
}
