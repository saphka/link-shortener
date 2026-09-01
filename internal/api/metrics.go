package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupMetricsEndpoint(mux *http.ServeMux, reg prometheus.Gatherer) {
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
}

func createMetricsMiddleware(reg prometheus.Registerer) MiddlewareFunc {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method", "path"},
	)

	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "A histogram of latencies for requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"code", "method", "path"},
	)

	reg.MustRegister(counter, duration)

	return func(next http.Handler) http.Handler {
		return promhttp.InstrumentHandlerDuration(
			duration,
			promhttp.InstrumentHandlerCounter(
				counter,
				next,
				promhttp.WithLabelFromRequest("path", requestPath),
			),
			promhttp.WithLabelFromRequest("path", requestPath),
		)
	}
}

func requestPath(req *http.Request) string {
	path := req.Pattern
	if path == "" {
		path = req.URL.Path
	}
	return path
}
