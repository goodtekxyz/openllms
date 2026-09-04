package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goodtekxyz/openllms/internal/config"
	"github.com/goodtekxyz/openllms/internal/db"
	"github.com/goodtekxyz/openllms/internal/health"
	"github.com/goodtekxyz/openllms/internal/httpserver"
	"github.com/goodtekxyz/openllms/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx := context.Background()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	var st *store.Store
	if db.IsSQLite(cfg.DatabaseURL) {
		sqldb, err := db.ConnectSQLite(ctx, cfg.DatabaseURL)
		if err != nil {
			slog.Error("sqlite connect", "err", err)
			os.Exit(1)
		}
		defer sqldb.Close()
		st = store.NewSQLiteWithCacheTTL(sqldb, cfg.AuthCacheTTL, cfg.RouteCacheTTL)
		slog.Info("database backend", "kind", "sqlite", "url", cfg.DatabaseURL)
	} else {
		pool, err := db.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			slog.Error("db connect", "err", err)
			os.Exit(1)
		}
		defer pool.Close()
		st = store.NewWithCacheTTL(pool, cfg.AuthCacheTTL, cfg.RouteCacheTTL)
		slog.Info("database backend", "kind", "postgres")
	}

	secret, err := openSecrets(ctx, &cfg)
	if err != nil {
		slog.Error("secrets", "err", err)
		os.Exit(1)
	}

	srv := httpserver.New(st, cfg, secret)
	refreshCtx, refreshCancel := context.WithCancel(context.Background())
	defer refreshCancel()
	srv.StartQuotaRefresh(refreshCtx, 5*time.Minute)
	prober := &health.Prober{Store: st, Secrets: secret, HTTP: &http.Client{Timeout: 8 * time.Second}}
	if secret != nil {
		prober.Start(refreshCtx, 2*time.Minute)
	}

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("llms-gateway listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
