package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Connect(ctx context.Context) (*DB, error) {
	dsn, err := dsn()
	if err != nil {
		return nil, err
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

func dsn() (string, error) {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url, nil
	}
	host, err := getenv("POSTGRES_HOST")
	if err != nil {
		return "", err
	}
	port, err := getenv("POSTGRES_PORT")
	if err != nil {
		return "", err
	}
	user, err := getenv("POSTGRES_USER")
	if err != nil {
		return "", err
	}
	pass, err := getenv("POSTGRES_PASSWORD")
	if err != nil {
		return "", err
	}
	name, err := getenv("POSTGRES_DB")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, pass, host, port, name), nil
}

func getenv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}
