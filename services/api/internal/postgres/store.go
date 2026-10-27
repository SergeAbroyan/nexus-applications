package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeAbroyan/nexus-applications/services/api/internal/items"
)

const (
	schema = `
CREATE TABLE IF NOT EXISTS items (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`
	readOnlyTransaction = "25006"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}

	store := &Store{pool: pool}
	if err := store.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (store *Store) migrate(ctx context.Context) error {
	var replica bool
	if err := store.pool.QueryRow(ctx, `SELECT pg_is_in_recovery()`).Scan(&replica); err != nil {
		return fmt.Errorf("check whether the database is a replica: %w", err)
	}
	if replica {
		return nil
	}

	if _, err := store.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

func (store *Store) Ping(ctx context.Context) error {
	return store.pool.Ping(ctx)
}

func (store *Store) Close() {
	store.pool.Close()
}

func (store *Store) ListItems(ctx context.Context) ([]items.Item, error) {
	rows, err := store.pool.Query(ctx, `SELECT id, name, created_at FROM items ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[items.Item])
}

func (store *Store) CreateItem(ctx context.Context, name string) (items.Item, error) {
	var created items.Item
	err := store.pool.QueryRow(
		ctx,
		`INSERT INTO items (name) VALUES ($1) RETURNING id, name, created_at`,
		name,
	).Scan(&created.ID, &created.Name, &created.CreatedAt)

	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == readOnlyTransaction {
		return items.Item{}, items.ErrReadOnly
	}
	return created, err
}
