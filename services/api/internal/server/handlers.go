package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/SergeAbroyan/nexus-applications/services/api/internal/items"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/notifications"
)

const maxRequestBytes = 64 << 10

type handlers struct {
	version       string
	logger        *slog.Logger
	items         ItemService
	notifications NotificationService
	readiness     []Check
}

func (handlers *handlers) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (handlers *handlers) showVersion(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"version": handlers.version})
}

func (handlers *handlers) ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()

	for _, check := range handlers.readiness {
		if err := check.Ping(ctx); err != nil {
			handlers.logger.WarnContext(ctx, "readiness check failed", "dependency", check.Name, "error", err)
			writeError(response, http.StatusServiceUnavailable, check.Name+" unavailable")
			return
		}
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
}

func (handlers *handlers) listItems(response http.ResponseWriter, request *http.Request) {
	list, source, err := handlers.items.List(request.Context())
	if err != nil {
		handlers.logger.ErrorContext(request.Context(), "list items", "error", err)
		writeError(response, http.StatusInternalServerError, "could not list items")
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": list, "source": source})
}

func (handlers *handlers) createItem(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "invalid JSON body")
		return
	}

	created, err := handlers.items.Create(request.Context(), input.Name)
	switch {
	case errors.Is(err, items.ErrInvalidName):
		writeError(response, http.StatusBadRequest, err.Error())
	case errors.Is(err, items.ErrReadOnly):
		writeError(response, http.StatusServiceUnavailable, err.Error())
	case err != nil:
		handlers.logger.ErrorContext(request.Context(), "create item", "error", err)
		writeError(response, http.StatusInternalServerError, "could not create item")
	default:
		writeJSON(response, http.StatusCreated, created)
	}
}

func (handlers *handlers) requestNotification(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Message string `json:"message"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, http.StatusBadRequest, "invalid JSON body")
		return
	}

	err := handlers.notifications.Request(request.Context(), input.Message)
	switch {
	case errors.Is(err, notifications.ErrInvalidMessage):
		writeError(response, http.StatusBadRequest, err.Error())
	case err != nil:
		handlers.logger.ErrorContext(request.Context(), "publish notification", "error", err)
		writeError(response, http.StatusBadGateway, "could not publish notification")
	default:
		writeJSON(response, http.StatusAccepted, map[string]string{"status": "queued"})
	}
}

func decodeJSON(response http.ResponseWriter, request *http.Request, destination any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
