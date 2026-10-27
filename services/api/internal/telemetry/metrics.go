package telemetry

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	inFlight prometheus.Gauge
}

func NewMetrics(service string) *Metrics {
	labels := prometheus.Labels{"service": service}
	metrics := &Metrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "nexus_http_requests_total",
			Help:        "HTTP requests served by the application routes.",
			ConstLabels: labels,
		}, []string{"code", "method"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "nexus_http_requests_in_flight",
			Help:        "HTTP requests currently being served by the application routes.",
			ConstLabels: labels,
		}),
	}

	metrics.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		metrics.requests,
		metrics.inFlight,
	)
	return metrics
}

func (metrics *Metrics) Instrument(next http.Handler) http.Handler {
	return promhttp.InstrumentHandlerInFlight(metrics.inFlight, promhttp.InstrumentHandlerCounter(metrics.requests, next))
}

func (metrics *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{Registry: metrics.registry})
}
