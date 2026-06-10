package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/singaaka/darkside/internal/config"
	"github.com/singaaka/darkside/internal/db"
	"github.com/singaaka/darkside/internal/frontend"
	"github.com/singaaka/darkside/internal/server"
	"github.com/singaaka/darkside/internal/store"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	_ "modernc.org/sqlite"
)

const version = "0.1.0"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfgPath := os.Getenv("DARKSIDE_CONFIG")
	if cfgPath == "" {
		cfgPath = "/etc/darkside/config.toml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	slog.Info("loaded config",
		"domain", cfg.Domain,
		"external_url", cfg.ExternalURL,
		"data_dir", cfg.DataDir,
		"listen", cfg.Listen,
	)

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		slog.Error("mkdir data dir", "error", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(cfg.DataDir, "darkside.sqlite")
	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		slog.Error("open sqlite", "error", err, "path", dbPath)
		os.Exit(1)
	}
	defer sqlDB.Close()
	if err := db.Migrate(context.Background(), sqlDB); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}
	st := store.New(sqlDB)
	slog.Info("sqlite ready", "path", dbPath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := server.New(ctx, server.Options{
		Config:   cfg,
		Store:    st,
		Version:  version,
		Frontend: frontend.Handler(),
	})

	h := h2c.NewHandler(srv.Handler(), &http2.Server{})
	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", cfg.Listen)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
