package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Mrbaeksang/kch-relay/backend/internal/application"
	"github.com/Mrbaeksang/kch-relay/backend/internal/catalog"
	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/product"
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
	app, err := application.New(ctx, c, catalog.Register, product.Module)
	if err != nil {
		return err
	}
	defer app.Close()
	slog.Info("KCH Relay listening", "port", c.Port, "environment", c.AppEnv, "authentication", c.AuthMode)
	return app.Run(ctx)
}
