package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type PostgresEnv struct {
	Container testcontainers.Container
	Pool      *pgxpool.Pool
	DSN       string
}

func StartPostgres(t *testing.T) *PostgresEnv {
	t.Helper()

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "openchip",
			"POSTGRES_USER":     "openchip",
			"POSTGRES_PASSWORD": "openchip",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}

	dsn := fmt.Sprintf("postgres://openchip:openchip@%s:%s/openchip?sslmode=disable", host, port.Port())
	pool := connectPool(t, dsn)
	t.Cleanup(pool.Close)

	runMigrations(t, pool)

	return &PostgresEnv{
		Container: container,
		Pool:      pool,
		DSN:       dsn,
	}
}

func connectPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	var lastErr error
	for range 40 {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			lastErr = pool.Ping(pingCtx)
			cancel()
			if lastErr == nil {
				return pool
			}
			pool.Close()
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("connect postgres: %v", lastErr)
	return nil
}

func runMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "db", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		statements := strings.Split(string(sqlBytes), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := pool.Exec(context.Background(), stmt); err != nil {
				t.Fatalf("run migration %s statement %q: %v", path, stmt, err)
			}
		}
	}
}
