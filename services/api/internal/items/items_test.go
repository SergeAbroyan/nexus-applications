package items

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type memoryStore struct {
	items []Item
	reads int
}

func (store *memoryStore) ListItems(context.Context) ([]Item, error) {
	store.reads++
	return store.items, nil
}

func (store *memoryStore) CreateItem(_ context.Context, name string) (Item, error) {
	created := Item{ID: int64(len(store.items) + 1), Name: name, CreatedAt: time.Now().UTC()}
	store.items = append(store.items, created)
	return created, nil
}

type memoryCache struct {
	values map[string][]byte
	err    error
}

func (cache *memoryCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	value, found := cache.values[key]
	return value, found, cache.err
}

func (cache *memoryCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	cache.values[key] = value
	return cache.err
}

func (cache *memoryCache) Delete(_ context.Context, key string) error {
	delete(cache.values, key)
	return cache.err
}

func newService() (*Service, *memoryStore, *memoryCache) {
	store := &memoryStore{}
	cache := &memoryCache{values: map[string][]byte{}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(store, cache, time.Minute, logger), store, cache
}

func TestListReadsTheDatabaseOnceAndThenTheCache(t *testing.T) {
	service, store, _ := newService()
	ctx := context.Background()
	if _, err := service.Create(ctx, "first"); err != nil {
		t.Fatal(err)
	}

	for _, want := range []Source{FromDatabase, FromCache} {
		list, source, err := service.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if source != want || len(list) != 1 || list[0].Name != "first" {
			t.Fatalf("List() = %v from %s, want one item from %s", list, source, want)
		}
	}
	if store.reads != 1 {
		t.Fatalf("database reads = %d, want 1", store.reads)
	}
}

func TestCreateInvalidatesTheCache(t *testing.T) {
	service, _, cache := newService()
	ctx := context.Background()
	if _, _, err := service.List(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Create(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	if _, found := cache.values[cacheKey]; found {
		t.Fatal("expected the item list to be removed from the cache")
	}
}

func TestListFallsBackToTheDatabaseWhenTheCacheFails(t *testing.T) {
	service, _, cache := newService()
	cache.err = errors.New("cache down")

	_, source, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if source != FromDatabase {
		t.Fatalf("source = %s, want %s", source, FromDatabase)
	}
}

func TestCreateValidatesTheName(t *testing.T) {
	service, _, _ := newService()
	ctx := context.Background()

	for _, name := range []string{"", "   ", strings.Repeat("a", MaxNameLength+1)} {
		if _, err := service.Create(ctx, name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Create(%q) error = %v, want ErrInvalidName", name, err)
		}
	}

	created, err := service.Create(ctx, "  "+strings.Repeat("é", MaxNameLength)+"  ")
	if err != nil {
		t.Fatalf("a name of %d multi-byte characters must be accepted: %v", MaxNameLength, err)
	}
	if strings.HasPrefix(created.Name, " ") {
		t.Fatal("expected the name to be trimmed")
	}
}
