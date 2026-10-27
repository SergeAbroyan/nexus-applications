package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxNameLength = 200
	cacheKey      = "nexus:items"
)

var (
	ErrInvalidName = fmt.Errorf("name must contain between 1 and %d characters", MaxNameLength)
	ErrReadOnly    = errors.New("items are read-only while the database is a standby replica")
)

type Item struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Source string

const (
	FromCache    Source = "cache"
	FromDatabase Source = "database"
)

type Store interface {
	ListItems(ctx context.Context) ([]Item, error)
	CreateItem(ctx context.Context, name string) (Item, error)
}

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type Service struct {
	store  Store
	cache  Cache
	ttl    time.Duration
	logger *slog.Logger
}

func NewService(store Store, cache Cache, ttl time.Duration, logger *slog.Logger) *Service {
	return &Service{store: store, cache: cache, ttl: ttl, logger: logger}
}

func (service *Service) List(ctx context.Context) ([]Item, Source, error) {
	if cached, found := service.readCache(ctx); found {
		return cached, FromCache, nil
	}

	stored, err := service.store.ListItems(ctx)
	if err != nil {
		return nil, "", err
	}
	service.writeCache(ctx, stored)
	return stored, FromDatabase, nil
}

func (service *Service) Create(ctx context.Context, name string) (Item, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLength {
		return Item{}, ErrInvalidName
	}

	created, err := service.store.CreateItem(ctx, name)
	if err != nil {
		return Item{}, err
	}
	if err := service.cache.Delete(ctx, cacheKey); err != nil {
		service.logger.WarnContext(ctx, "invalidate item cache", "error", err)
	}
	return created, nil
}

func (service *Service) readCache(ctx context.Context) ([]Item, bool) {
	value, found, err := service.cache.Get(ctx, cacheKey)
	if err != nil {
		service.logger.WarnContext(ctx, "read item cache", "error", err)
		return nil, false
	}
	if !found {
		return nil, false
	}

	var cached []Item
	if err := json.Unmarshal(value, &cached); err != nil {
		service.logger.WarnContext(ctx, "decode item cache", "error", err)
		return nil, false
	}
	return cached, true
}

func (service *Service) writeCache(ctx context.Context, stored []Item) {
	value, err := json.Marshal(stored)
	if err == nil {
		err = service.cache.Set(ctx, cacheKey, value, service.ttl)
	}
	if err != nil {
		service.logger.WarnContext(ctx, "write item cache", "error", err)
	}
}
