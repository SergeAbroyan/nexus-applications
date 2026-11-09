package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/SergeAbroyan/nexus-applications/services/api/internal/items"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/notifications"
)

func TestResponsesMatchTheOpenAPIDocument(t *testing.T) {
	ctx := context.Background()
	document, err := openapi3.NewLoader().LoadFromData(OpenAPI)
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}
	if err := document.Validate(ctx); err != nil {
		t.Fatalf("invalid OpenAPI document: %v", err)
	}
	router, err := gorillamux.NewRouter(document)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	cases := []struct {
		name    string
		method  string
		path    string
		body    string
		prepare func(*testServer)
		status  int
	}{
		{name: "list items", method: http.MethodGet, path: "/api/items", status: http.StatusOK},
		{name: "create an item", method: http.MethodPost, path: "/api/items", body: `{"name":"new item"}`, status: http.StatusCreated},
		{name: "reject an invalid item", method: http.MethodPost, path: "/api/items", body: `{"name":"x"}`, status: http.StatusBadRequest,
			prepare: func(server *testServer) { server.items.createErr = items.ErrInvalidName }},
		{name: "refuse writes on a replica", method: http.MethodPost, path: "/api/items", body: `{"name":"x"}`, status: http.StatusServiceUnavailable,
			prepare: func(server *testServer) { server.items.createErr = items.ErrReadOnly }},
		{name: "fail to create", method: http.MethodPost, path: "/api/items", body: `{"name":"x"}`, status: http.StatusInternalServerError,
			prepare: func(server *testServer) { server.items.createErr = errors.New("database down") }},
		{name: "queue a notification", method: http.MethodPost, path: "/api/notify", body: `{"message":"hello"}`, status: http.StatusAccepted},
		{name: "reject an invalid notification", method: http.MethodPost, path: "/api/notify", body: `{"message":"x"}`, status: http.StatusBadRequest,
			prepare: func(server *testServer) { server.notifications.err = notifications.ErrInvalidMessage }},
		{name: "fail to queue", method: http.MethodPost, path: "/api/notify", body: `{"message":"x"}`, status: http.StatusBadGateway,
			prepare: func(server *testServer) { server.notifications.err = errors.New("queue down") }},
		{name: "report the version", method: http.MethodGet, path: "/api/version", status: http.StatusOK},
		{name: "serve this document", method: http.MethodGet, path: "/api/openapi.yaml", status: http.StatusOK},
	}

	covered := map[string]bool{}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newTestServer()
			if testCase.prepare != nil {
				testCase.prepare(server)
			}

			request := httptest.NewRequest(testCase.method, "http://nexus.test"+testCase.path, strings.NewReader(testCase.body))
			if testCase.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			route, parameters, err := router.FindRoute(request)
			if err != nil {
				t.Fatalf("%s %s is not documented: %v", testCase.method, testCase.path, err)
			}
			covered[route.Operation.OperationID] = true

			requestInput := &openapi3filter.RequestValidationInput{Request: request, PathParams: parameters, Route: route}
			if err := openapi3filter.ValidateRequest(ctx, requestInput); err != nil {
				t.Fatalf("request does not match the document: %v", err)
			}
			request.Body = io.NopCloser(strings.NewReader(testCase.body))

			recorder := httptest.NewRecorder()
			server.handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.status {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.status)
			}

			responseInput := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: requestInput,
				Status:                 recorder.Code,
				Header:                 recorder.Header(),
				Body:                   io.NopCloser(bytes.NewReader(recorder.Body.Bytes())),
			}
			if err := openapi3filter.ValidateResponse(ctx, responseInput); err != nil {
				t.Fatalf("response does not match the document: %v", err)
			}
		})
	}

	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			if !covered[operation.OperationID] {
				t.Errorf("%s %s is documented but not tested", method, path)
			}
		}
	}
}
