//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestStoreAgainstPostgreSQL(t *testing.T) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:18.6-alpine3.24",
		tcpostgres.WithDatabase("nexus"),
		tcpostgres.WithUsername("nexus"),
		tcpostgres.WithPassword("nexus"),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}

	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("read connection string: %v", err)
	}

	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	again, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open store a second time: %v", err)
	}
	again.Close()

	first, err := store.CreateItem(ctx, "first")
	if err != nil {
		t.Fatalf("create first item: %v", err)
	}
	second, err := store.CreateItem(ctx, "second")
	if err != nil {
		t.Fatalf("create second item: %v", err)
	}

	listed, err := store.ListItems(ctx)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(listed) != 2 || listed[0].ID != first.ID || listed[1].ID != second.ID {
		t.Fatalf("listed %+v, want %+v then %+v", listed, first, second)
	}
	if listed[0].Name != "first" || listed[0].CreatedAt.IsZero() {
		t.Fatalf("first item was stored as %+v", listed[0])
	}

	if _, err := store.CreateItem(ctx, ""); err == nil {
		t.Fatal("an empty name was accepted")
	}

	var version int
	var dirty bool
	if err := store.pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatalf("read migration version: %v", err)
	}
	if version != 1 || dirty {
		t.Fatalf("migration version = %d dirty = %v, want 1 and clean", version, dirty)
	}
}
