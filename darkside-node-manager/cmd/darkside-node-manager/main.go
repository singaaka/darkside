package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/singaaka/darkside-node-manager/internal/ansible"
	"github.com/singaaka/darkside-node-manager/internal/db"
	"github.com/singaaka/darkside-node-manager/internal/frontend"
	"github.com/singaaka/darkside-node-manager/internal/queue"
	"github.com/singaaka/darkside-node-manager/internal/server"
	"github.com/singaaka/darkside-node-manager/internal/store"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	_ "modernc.org/sqlite"
)

const defaultPort = "7373"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Verify ansible is installed.
	if ver, err := ansible.Check(); err != nil {
		slog.Error("ansible-playbook not found",
			"error", err,
			"hint", "Install ansible: https://docs.ansible.com/ansible/latest/installation_guide/index.html")
		os.Exit(1)
	} else {
		slog.Info("ansible ready", "version", ver)
	}

	// Open SQLite database alongside the binary.
	execPath, err := os.Executable()
	if err != nil {
		execPath = "."
	}
	dbPath := filepath.Join(filepath.Dir(execPath), "darkside-node-manager.db")
	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		slog.Error("open sqlite", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := db.Migrate(ctx, sqlDB); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}
	slog.Info("database ready", "path", dbPath)

	st := store.New(sqlDB)

	// Find playbooks directory: next to binary, or embedded.
	execDir := filepath.Dir(execPath)
	playbookDir := filepath.Join(execDir, "playbooks")
	if _, err := os.Stat(playbookDir); os.IsNotExist(err) {
		// Fall back to project source when running with `go run`.
		_, callerFile, _, _ := runtime.Caller(0)
		playbookDir = filepath.Join(filepath.Dir(callerFile), "..", "..", "playbooks")
	}

	runner := &ansible.Runner{PlaybookDir: playbookDir}
	q := queue.New(st)

	srv := server.New(server.Options{
		Store:       st,
		Queue:       q,
		Runner:      runner,
		PlaybookDir: playbookDir,
		Frontend:    frontend.Handler(),
	})
	server.RegisterJobHandlers(srv)

	// Start queue worker in background.
	go q.Run(ctx)

	port := os.Getenv("DARKSIDE_NM_PORT")
	if port == "" {
		port = defaultPort
	}
	addr := ":" + port

	h := h2c.NewHandler(srv.Handler(), &http2.Server{})
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("darkside node manager listening", "addr", fmt.Sprintf("http://localhost%s", addr))
	slog.Info("open your browser to get started")

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutCtx)
}
