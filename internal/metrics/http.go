package metrics

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal          *prometheus.CounterVec
	httpRequestDurationSeconds *prometheus.HistogramVec
)

func initHTTPMetrics() {
	if httpRequestsTotal != nil {
		return
	}
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "status", "path"},
	)
	httpRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "status", "path"},
	)
}

// HTTPMiddleware records Prometheus metrics compatible with the Grafana API dashboard (http_requests_total, http_request_duration_seconds_bucket).
func HTTPMiddleware() fiber.Handler {
	initHTTPMetrics()
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		path := routePath(c)
		status := strconv.Itoa(c.Response().StatusCode())
		method := canonicalHTTPMethod(c.Method())

		httpRequestsTotal.WithLabelValues(method, status, path).Inc()
		httpRequestDurationSeconds.WithLabelValues(method, status, path).Observe(time.Since(start).Seconds())
		return err
	}
}

// canonicalHTTPMethod maps the request method to a stable label (RFC 7231 methods only; unknown → OTHER).
func canonicalHTTPMethod(m string) string {
	s := strings.ToUpper(strings.TrimSpace(m))
	switch s {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
		return s
	case "POS":
		// Seen with some proxies/clients + fasthttp (truncated POST).
		return "POST"
	case "GETT":
		return "GET"
	default:
		if s == "" {
			return "UNKNOWN"
		}
		return "OTHER"
	}
}

func routePath(c *fiber.Ctx) string {
	if r := c.Route(); r != nil && r.Path != "" {
		return r.Path
	}
	p := c.Path()
	if p == "" {
		return "/"
	}
	return p
}
