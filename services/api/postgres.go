package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const createItemsTable = `
CREATE TABLE IF NOT EXISTS items (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

type item struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type postgresStore struct {
	pool *pgxpool.Pool
}

func newPostgresStore(ctx context.Context, databaseURL string) (*postgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	store := &postgresStore{pool: pool}
	if err := store.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	if _, err := pool.Exec(ctx, createItemsTable); err != nil {
		pool.Close()
		return nil, err
	}

	return store, nil
}

func (store *postgresStore) Close() {
	store.pool.Close()
}

func (store *postgresStore) Ping(ctx context.Context) error {
	return store.pool.Ping(ctx)
}

func (store *postgresStore) ListItems(ctx context.Context) ([]item, error) {
	rows, err := store.pool.Query(ctx, `SELECT id, name, created_at FROM items ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]item, 0)
	for rows.Next() {
		var current item
		if err := rows.Scan(&current.ID, &current.Name, &current.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, current)
	}

	return items, rows.Err()
}

func (store *postgresStore) CreateItem(ctx context.Context, name string) (item, error) {
	var created item
	err := store.pool.QueryRow(
		ctx,
		`INSERT INTO items (name) VALUES ($1) RETURNING id, name, created_at`,
		name,
	).Scan(&created.ID, &created.Name, &created.CreatedAt)
	return created, err
}
