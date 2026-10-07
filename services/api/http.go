package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type itemStore interface {
	Ping(context.Context) error
	ListItems(context.Context) ([]item, error)
	CreateItem(context.Context, string) (item, error)
}

type itemCache interface {
	Ping(context.Context) error
	GetItems(context.Context) ([]item, bool, error)
	SetItems(context.Context, []item) error
	DeleteItems(context.Context) error
}

type messageQueue interface {
	Publish(context.Context, string) error
}

type application struct {
	logger *slog.Logger
	store  itemStore
	cache  itemCache
	queue  messageQueue
}

func (app *application) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", app.health)
	mux.HandleFunc("GET /readyz", app.ready)
	mux.HandleFunc("GET /api/items", app.listItems)
	mux.HandleFunc("POST /api/items", app.createItem)
	mux.HandleFunc("POST /api/notify", app.publishNotification)
	return mux
}

func (app *application) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (app *application) ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()

	if err := app.store.Ping(ctx); err != nil {
		writeError(response, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	if err := app.cache.Ping(ctx); err != nil {
		writeError(response, http.StatusServiceUnavailable, "cache unavailable")
		return
	}

	writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
}

func (app *application) listItems(response http.ResponseWriter, request *http.Request) {
	items, found, err := app.cache.GetItems(request.Context())
	if err == nil && found {
		writeJSON(response, http.StatusOK, map[string]any{"items": items, "source": "cache"})
		return
	}
	if err != nil {
		app.logger.Warn("read item cache", "error", err)
	}

	items, err = app.store.ListItems(request.Context())
	if err != nil {
		app.logger.Error("list items", "error", err)
		writeError(response, http.StatusInternalServerError, "could not list items")
		return
	}

	if err := app.cache.SetItems(request.Context(), items); err != nil {
		app.logger.Warn("write item cache", "error", err)
	}

	writeJSON(response, http.StatusOK, map[string]any{"items": items, "source": "database"})
}

func (app *application) createItem(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "invalid JSON body")
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 200 {
		writeError(response, http.StatusBadRequest, "name must contain between 1 and 200 characters")
		return
	}

	created, err := app.store.CreateItem(request.Context(), input.Name)
	if err != nil {
		app.logger.Error("create item", "error", err)
		writeError(response, http.StatusInternalServerError, "could not create item")
		return
	}

	if err := app.cache.DeleteItems(request.Context()); err != nil {
		app.logger.Warn("invalidate item cache", "error", err)
	}

	writeJSON(response, http.StatusCreated, created)
}

func (app *application) publishNotification(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Message string `json:"message"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "invalid JSON body")
		return
	}

	input.Message = strings.TrimSpace(input.Message)
	if input.Message == "" || len(input.Message) > 500 {
		writeError(response, http.StatusBadRequest, "message must contain between 1 and 500 characters")
		return
	}

	if err := app.queue.Publish(request.Context(), input.Message); err != nil {
		app.logger.Error("publish notification", "error", err)
		writeError(response, http.StatusBadGateway, "could not publish notification")
		return
	}

	writeJSON(response, http.StatusAccepted, map[string]string{"status": "queued"})
}

func decodeJSON(request *http.Request, destination any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, request.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
