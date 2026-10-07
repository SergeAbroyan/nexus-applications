package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	items []item
}

func (store *fakeStore) Ping(context.Context) error {
	return nil
}

func (store *fakeStore) ListItems(context.Context) ([]item, error) {
	return store.items, nil
}

func (store *fakeStore) CreateItem(_ context.Context, name string) (item, error) {
	created := item{ID: int64(len(store.items) + 1), Name: name, CreatedAt: time.Now().UTC()}
	store.items = append(store.items, created)
	return created, nil
}

type fakeCache struct {
	items []item
	found bool
}

func (cache *fakeCache) Ping(context.Context) error {
	return nil
}

func (cache *fakeCache) GetItems(context.Context) ([]item, bool, error) {
	return cache.items, cache.found, nil
}

func (cache *fakeCache) SetItems(_ context.Context, items []item) error {
	cache.items = items
	cache.found = true
	return nil
}

func (cache *fakeCache) DeleteItems(context.Context) error {
	cache.items = nil
	cache.found = false
	return nil
}

type fakeQueue struct {
	messages []string
}

func (queue *fakeQueue) Publish(_ context.Context, message string) error {
	queue.messages = append(queue.messages, message)
	return nil
}

func testApplication() (*application, *fakeStore, *fakeQueue) {
	store := &fakeStore{}
	queue := &fakeQueue{}
	return &application{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:  store,
		cache:  &fakeCache{},
		queue:  queue,
	}, store, queue
}

func TestCreateAndListItems(t *testing.T) {
	app, _, _ := testApplication()

	createRequest := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(`{"name":"first item"}`))
	createResponse := httptest.NewRecorder()
	app.handler().ServeHTTP(createResponse, createRequest)

	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createResponse.Code)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	listResponse := httptest.NewRecorder()
	app.handler().ServeHTTP(listResponse, listRequest)

	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listResponse.Code)
	}
	if !strings.Contains(listResponse.Body.String(), "first item") {
		t.Fatal("expected the created item in the response")
	}
}

func TestRejectsInvalidItem(t *testing.T) {
	app, _, _ := testApplication()
	request := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(`{"name":""}`))
	response := httptest.NewRecorder()

	app.handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestPublishesNotification(t *testing.T) {
	app, _, queue := testApplication()
	request := httptest.NewRequest(http.MethodPost, "/api/notify", strings.NewReader(`{"message":"hello worker"}`))
	response := httptest.NewRecorder()

	app.handler().ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, response.Code)
	}
	if len(queue.messages) != 1 || queue.messages[0] != "hello worker" {
		t.Fatal("expected one queued notification")
	}
}
