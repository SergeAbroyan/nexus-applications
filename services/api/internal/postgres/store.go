package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/SergeAbroyan/nexus-applications/services/api/internal/items"
)

const readOnlyTransaction = "25006"

//go:embed migrations/*.sql
var migrations embed.FS

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

	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	database := stdlib.OpenDBFromPool(store.pool)
	driver, err := migratepgx.WithInstance(database, &migratepgx.Config{})
	if err != nil {
		return errors.Join(fmt.Errorf("prepare migrations: %w", err), database.Close())
	}
	migrator, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return errors.Join(fmt.Errorf("prepare migrations: %w", err), driver.Close())
	}

	applied := migrator.Up()
	sourceErr, databaseErr := migrator.Close()
	if applied != nil && !errors.Is(applied, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", applied)
	}
	return errors.Join(sourceErr, databaseErr)
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
