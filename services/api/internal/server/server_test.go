package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SergeAbroyan/nexus-applications/services/api/internal/items"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/notifications"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/telemetry"
)

type fakeItems struct {
	createErr error
}

func (fake *fakeItems) List(context.Context) ([]items.Item, items.Source, error) {
	return []items.Item{{ID: 1, Name: "first item"}}, items.FromCache, nil
}

func (fake *fakeItems) Create(_ context.Context, name string) (items.Item, error) {
	if fake.createErr != nil {
		return items.Item{}, fake.createErr
	}
	return items.Item{ID: 2, Name: name}, nil
}

type fakeNotifications struct {
	err      error
	messages []string
}

func (fake *fakeNotifications) Request(_ context.Context, message string) error {
	fake.messages = append(fake.messages, message)
	return fake.err
}

type testServer struct {
	handler       http.Handler
	items         *fakeItems
	notifications *fakeNotifications
	databaseErr   error
}

func newTestServer() *testServer {
	server := &testServer{items: &fakeItems{}, notifications: &fakeNotifications{}}
	server.handler = New(Dependencies{
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics:       telemetry.NewMetrics("test"),
		Items:         server.items,
		Notifications: server.notifications,
		Readiness: []Check{{Name: "database", Ping: func(context.Context) error {
			return server.databaseErr
		}}},
	})
	return server
}

func (server *testServer) do(method, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.handler.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
	return response
}

func expect(t *testing.T, response *httptest.ResponseRecorder, status int, bodyContains string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body %q", response.Code, status, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), bodyContains) {
		t.Fatalf("body %q does not contain %q", response.Body.String(), bodyContains)
	}
}

func TestServesTheWebPageWithSecurityHeaders(t *testing.T) {
	server := newTestServer()

	page := server.do(http.MethodGet, "/", "")
	expect(t, page, http.StatusOK, "Nexus Platform")
	if policy := page.Header().Get("Content-Security-Policy"); !strings.Contains(policy, "default-src 'self'") || strings.Contains(policy, "unsafe-inline") {
		t.Fatalf("unexpected Content-Security-Policy %q", policy)
	}
	for header, want := range map[string]string{
		"Cross-Origin-Embedder-Policy": "require-corp",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
		"X-Frame-Options":              "DENY",
	} {
		if got := page.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
	if page.Header().Get("Permissions-Policy") == "" {
		t.Fatal("Permissions-Policy is missing")
	}
	if cache := server.do(http.MethodGet, "/api/items", "").Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("API Cache-Control = %q, want no-store", cache)
	}

	script := server.do(http.MethodGet, "/app.js", "")
	expect(t, script, http.StatusOK, `"/api/items"`)
	if contentType := script.Header().Get("Content-Type"); contentType != "text/javascript; charset=utf-8" {
		t.Fatalf("script content type = %q", contentType)
	}

	expect(t, server.do(http.MethodGet, "/app.css", ""), http.StatusOK, "color-scheme")
	expect(t, server.do(http.MethodGet, "/missing", ""), http.StatusNotFound, "")
}

func TestProbes(t *testing.T) {
	server := newTestServer()
	expect(t, server.do(http.MethodGet, "/healthz", ""), http.StatusOK, `"ok"`)
	expect(t, server.do(http.MethodGet, "/readyz", ""), http.StatusOK, `"ready"`)

	server.databaseErr = errors.New("connection refused")
	expect(t, server.do(http.MethodGet, "/readyz", ""), http.StatusServiceUnavailable, "database unavailable")
	expect(t, server.do(http.MethodGet, "/healthz", ""), http.StatusOK, `"ok"`)
}

func TestItems(t *testing.T) {
	server := newTestServer()
	expect(t, server.do(http.MethodGet, "/api/items", ""), http.StatusOK, `"source":"cache"`)
	expect(t, server.do(http.MethodPost, "/api/items", `{"name":"second item"}`), http.StatusCreated, "second item")
	expect(t, server.do(http.MethodPost, "/api/items", `{"name":"x","extra":true}`), http.StatusBadRequest, "invalid JSON body")
	expect(t, server.do(http.MethodPost, "/api/items", `{"name":"x"}{}`), http.StatusBadRequest, "invalid JSON body")
	expect(t, server.do(http.MethodDelete, "/api/items", ""), http.StatusMethodNotAllowed, "")

	server.items.createErr = items.ErrInvalidName
	expect(t, server.do(http.MethodPost, "/api/items", `{"name":""}`), http.StatusBadRequest, "between 1 and 200")

	server.items.createErr = items.ErrReadOnly
	expect(t, server.do(http.MethodPost, "/api/items", `{"name":"x"}`), http.StatusServiceUnavailable, "read-only")

	server.items.createErr = errors.New("disk full")
	expect(t, server.do(http.MethodPost, "/api/items", `{"name":"x"}`), http.StatusInternalServerError, "could not create item")
}

func TestNotifications(t *testing.T) {
	server := newTestServer()
	expect(t, server.do(http.MethodPost, "/api/notify", `{"message":"hello worker"}`), http.StatusAccepted, "queued")
	if len(server.notifications.messages) != 1 || server.notifications.messages[0] != "hello worker" {
		t.Fatalf("messages = %v", server.notifications.messages)
	}

	server.notifications.err = notifications.ErrInvalidMessage
	expect(t, server.do(http.MethodPost, "/api/notify", `{"message":""}`), http.StatusBadRequest, "between 1 and 500")

	server.notifications.err = errors.New("queue unavailable")
	expect(t, server.do(http.MethodPost, "/api/notify", `{"message":"hello"}`), http.StatusBadGateway, "could not publish")
}

func TestMetricsCountApplicationRequestsOnly(t *testing.T) {
	server := newTestServer()
	server.do(http.MethodGet, "/api/items", "")
	server.do(http.MethodGet, "/healthz", "")

	metrics := server.do(http.MethodGet, "/metrics", "")
	expect(t, metrics, http.StatusOK, `nexus_http_requests_total{code="200",method="get",service="test"} 1`)
	expect(t, metrics, http.StatusOK, "go_goroutines")
}
