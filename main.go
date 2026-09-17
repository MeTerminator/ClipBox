package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MeTerminator/ClipBox/internal/config"
	"github.com/MeTerminator/ClipBox/internal/server"
	"github.com/MeTerminator/ClipBox/internal/store"
	"github.com/MeTerminator/ClipBox/internal/upload"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := store.Open(ctx, cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		slog.Error("initialize database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	uploadManager, err := upload.NewManager(cfg.DataDir, cfg.UploadChunkSize, cfg.MaxUploadFileSize, cfg.UploadSessionTTL)
	if err != nil {
		slog.Error("initialize upload storage", "error", err)
		os.Exit(1)
	}
	go uploadManager.RunCleanup(ctx)

	application := server.NewWithDependencies(cfg, server.Dependencies{
		Clips:   database,
		Rooms:   database,
		Uploads: uploadManager,
	})
	go application.RunCleanup(ctx)
	httpServer := &http.Server{
		Addr:              cfg.Address,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	go func() {
		slog.Info("ClipBox listening", "address", cfg.Address)
		if serveErr := httpServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("serve HTTP", "error", serveErr)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown HTTP server", "error", err)
	}
}
