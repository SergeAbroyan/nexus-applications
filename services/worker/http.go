package main

import (
	"encoding/json"
	"net/http"

	"github.com/SergeAbroyan/nexus-applications/services/observability"
)

func (worker *worker) handler() http.Handler {
	metrics := observability.New("worker")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(response http.ResponseWriter, _ *http.Request) {
		if !worker.ready.Load() {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
			return
		}
		writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.Handle("GET /metrics", metrics.Handler())
	return observability.TraceHTTPHandler("nexus-worker", metrics.Middleware(mux))
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
