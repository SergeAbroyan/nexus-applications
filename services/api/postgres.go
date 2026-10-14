package main

import (
	"context"
	"time"

	"github.com/SergeAbroyan/nexus-applications/services/observability"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

	ctx, span := postgresSpan(ctx, "CREATE")
	_, err = pool.Exec(ctx, createItemsTable)
	observability.RecordSpanError(span, err)
	span.End()
	if err != nil {
		pool.Close()
		return nil, err
	}

	return store, nil
}

func (store *postgresStore) Close() {
	store.pool.Close()
}

func (store *postgresStore) Ping(ctx context.Context) error {
	ctx, span := postgresSpan(ctx, "PING")
	defer span.End()

	err := store.pool.Ping(ctx)
	observability.RecordSpanError(span, err)
	return err
}

func (store *postgresStore) ListItems(ctx context.Context) ([]item, error) {
	ctx, span := postgresSpan(ctx, "SELECT")
	defer span.End()

	rows, err := store.pool.Query(ctx, `SELECT id, name, created_at FROM items ORDER BY id`)
	if err != nil {
		observability.RecordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	items := make([]item, 0)
	for rows.Next() {
		var current item
		if err := rows.Scan(&current.ID, &current.Name, &current.CreatedAt); err != nil {
			observability.RecordSpanError(span, err)
			return nil, err
		}
		items = append(items, current)
	}

	err = rows.Err()
	observability.RecordSpanError(span, err)
	return items, err
}

func (store *postgresStore) CreateItem(ctx context.Context, name string) (item, error) {
	ctx, span := postgresSpan(ctx, "INSERT")
	defer span.End()

	var created item
	err := store.pool.QueryRow(
		ctx,
		`INSERT INTO items (name) VALUES ($1) RETURNING id, name, created_at`,
		name,
	).Scan(&created.ID, &created.Name, &created.CreatedAt)
	observability.RecordSpanError(span, err)
	return created, err
}

func postgresSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return otel.Tracer("nexus-api/postgres").Start(
		ctx,
		"postgresql."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system.name", "postgresql"),
			attribute.String("db.operation.name", operation),
		),
	)
}
