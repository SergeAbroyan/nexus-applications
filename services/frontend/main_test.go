package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	handler, err := newHandler("")
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	if response.Body.String() != "ok\n" {
		t.Fatalf("expected health response %q, got %q", "ok\n", response.Body.String())
	}
}

func TestIndex(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	handler, err := newHandler("")
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	if !strings.Contains(response.Body.String(), "Nexus Platform") {
		t.Fatal("expected the Nexus page title")
	}

	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("expected security headers to be present")
	}
}

func TestUnknownPath(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()

	handler, err := newHandler("")
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestAPIProxy(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/items" {
			t.Fatalf("expected path /api/items, got %s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"items":[]}`))
	}))
	defer api.Close()

	handler, err := newHandler(api.URL)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/items", nil))

	result, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != `{"items":[]}` {
		t.Fatalf("unexpected proxy response %q", result)
	}
}

func TestInvalidAPIURL(t *testing.T) {
	if _, err := newHandler("api:8080"); err == nil {
		t.Fatal("expected an invalid API_URL error")
	}
}
