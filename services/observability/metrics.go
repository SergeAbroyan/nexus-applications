package observability

import (
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type Metrics struct {
	service  string
	started  time.Time
	requests atomic.Uint64
	inFlight atomic.Int64
}

func New(service string) *Metrics {
	return &Metrics{service: service, started: time.Now()}
}

func (metrics *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/metrics" {
			next.ServeHTTP(response, request)
			return
		}

		metrics.requests.Add(1)
		metrics.inFlight.Add(1)
		defer metrics.inFlight.Add(-1)
		next.ServeHTTP(response, request)
	})
}

func (metrics *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		service := strconv.Quote(metrics.service)

		_, _ = fmt.Fprintf(response, "# HELP nexus_application_info Static Nexus service information.\n")
		_, _ = fmt.Fprintf(response, "# TYPE nexus_application_info gauge\n")
		_, _ = fmt.Fprintf(response, "nexus_application_info{service=%s} 1\n", service)
		_, _ = fmt.Fprintf(response, "# HELP nexus_process_uptime_seconds Seconds since the service started.\n")
		_, _ = fmt.Fprintf(response, "# TYPE nexus_process_uptime_seconds gauge\n")
		_, _ = fmt.Fprintf(response, "nexus_process_uptime_seconds{service=%s} %.0f\n", service, time.Since(metrics.started).Seconds())
		_, _ = fmt.Fprintf(response, "# HELP nexus_http_requests_total HTTP requests received by the service.\n")
		_, _ = fmt.Fprintf(response, "# TYPE nexus_http_requests_total counter\n")
		_, _ = fmt.Fprintf(response, "nexus_http_requests_total{service=%s} %d\n", service, metrics.requests.Load())
		_, _ = fmt.Fprintf(response, "# HELP nexus_http_requests_in_flight HTTP requests currently being served.\n")
		_, _ = fmt.Fprintf(response, "# TYPE nexus_http_requests_in_flight gauge\n")
		_, _ = fmt.Fprintf(response, "nexus_http_requests_in_flight{service=%s} %d\n", service, metrics.inFlight.Load())
	})
}
