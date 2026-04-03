package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openchip/openchip/api/internal/app"
	"github.com/openchip/openchip/api/internal/config"
	"github.com/openchip/openchip/api/internal/email"
	"github.com/openchip/openchip/api/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)}))
	slog.SetDefault(logger)

	if err := runMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		logger.Error("migration_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if cfg.Env == "development" {
		if err := runMigrations(cfg.DatabaseURL, cfg.SeedMigrationsPath); err != nil {
			logger.Error("seed_migration_failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	pool, err := connectPoolWithRetry(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("db_connect_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app.New(cfg, store.New(pool, cfg.QueryTimeout), email.New(cfg.ResendAPIKey, cfg.FromEmail, cfg.DisableEmail, logger), logger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("server_starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server_failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-sigCtx.Done()

	start := time.Now()
	logger.Info("shutting down", slog.Duration("grace_period", 30*time.Second))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown_failed", slog.String("error", err.Error()))
		pool.Close()
		os.Exit(1)
	}
	pool.Close()
	logger.Info("shutdown complete", slog.Duration("duration", time.Since(start)))
}

func runMigrations(databaseURL, migrationsPath string) error {
	m, err := migrate.New(migrationsPath, databaseURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func connectPoolWithRetry(ctx context.Context, cfg config.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	poolConfig.MaxConns = cfg.DBMaxConns
	poolConfig.MaxConnLifetime = cfg.DBConnMaxLife
	// pgxpool does not expose a direct max-idle-conns count; MaxConnIdleTime is the closest
	// operational control, while DB_MAX_IDLE_CONNS is retained for parity with deployment docs.
	poolConfig.MaxConnIdleTime = cfg.DBConnMaxLife
	poolConfig.HealthCheckPeriod = 30 * time.Second

	var lastErr error
	backoff := time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, cfg.QueryTimeout)
			lastErr = pool.Ping(pingCtx)
			cancel()
			if lastErr == nil {
				logger.Info("db_connected", slog.Int("attempt", attempt), slog.Int64("max_open_conns", int64(cfg.DBMaxConns)), slog.Int64("max_idle_conns", int64(cfg.DBMaxIdleConns)))
				return pool, nil
			}
			pool.Close()
		} else {
			lastErr = err
		}
		if attempt < 3 {
			logger.Warn("db_connect_retry", slog.Int("attempt", attempt), slog.Duration("backoff", backoff), slog.String("error", lastErr.Error()))
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return nil, fmt.Errorf("connect db after retries: %w", lastErr)
}

func parseLogLevel(level config.SlogLevel) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
