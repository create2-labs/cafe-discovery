package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registry = prometheus.NewRegistry()

// Handler serves GET /metrics from the dedicated registry (no default go/process collectors).
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func registerCollectors(collectors ...prometheus.Collector) {
	registry.MustRegister(collectors...)
}
