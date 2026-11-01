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

	"github.com/ReanSn0w/justkb/internal/config"
	"github.com/ReanSn0w/justkb/internal/database"
	"github.com/ReanSn0w/justkb/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	if err := os.MkdirAll(cfg.StoragePath, 0o750); err != nil {
		logger.Error("create storage directory", "error", err)
		os.Exit(1)
	}
	connectContext, cancelConnect := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := database.Open(connectContext, cfg.DatabaseURL)
	cancelConnect()
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	handler := httpapi.New(httpapi.Dependencies{
		Logger:            logger,
		StoragePath:       cfg.StoragePath,
		Database:          db,
		DB:                db,
		DatabaseURL:       cfg.DatabaseURL,
		CookieSecure:      cfg.CookieSecure,
		AttachmentMaxSize: cfg.AttachmentMaxSize,
	})
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("http server started", "address", cfg.ListenAddress)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	case <-shutdownContext.Done():
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			_ = server.Close()
			os.Exit(1)
		}
		logger.Info("http server stopped")
	}
}
