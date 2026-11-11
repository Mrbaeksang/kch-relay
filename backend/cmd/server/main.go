package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/catalog"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/messages"
	"github.com/Mrbaeksang/kch-relay/backend/internal/notifications"
	"github.com/Mrbaeksang/kch-relay/backend/internal/product"
	"github.com/Mrbaeksang/kch-relay/backend/internal/statistics"
	"github.com/Mrbaeksang/kch-relay/backend/internal/transactions"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	c, err := config.LoadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := application.New(ctx, c, statistics.Register, notifications.Module(nil), catalog.Register, product.Module, transactions.Module(func(ctx context.Context, tx *sql.Tx, productID, sellerID string, ids []string, _ time.Time) error {
		return product.AttachImagesTx(ctx, tx, productID, sellerID, ids)
	}), messages.Module())
	if err != nil {
		return err
	}
	defer app.Close()
	slog.Info("KCH Relay listening", "port", c.Port, "environment", c.AppEnv, "authentication", c.AuthMode)
	return app.Run(ctx)
}
