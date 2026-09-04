package metrics

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal          *prometheus.CounterVec
	httpRequestDurationSeconds *prometheus.HistogramVec
	httpMetricsOnce            sync.Once
)

func initHTTPMetrics() {
	httpMetricsOnce.Do(func() {
		httpRequestsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "status", "path"},
		)
		httpRequestDurationSeconds = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "status", "path"},
		)
		registerCollectors(httpRequestsTotal, httpRequestDurationSeconds)
	})
}

// HTTPMiddleware records Prometheus metrics compatible with the Grafana API dashboard (http_requests_total, http_request_duration_seconds_bucket).
func HTTPMiddleware() fiber.Handler {
	initHTTPMetrics()
	return func(c fiber.Ctx) error {
		start := time.Now()
		// fasthttp reuses request buffers; clone before Next() so Prometheus labels stay stable.
		method := sanitizeLabelValue(canonicalHTTPMethod(cloneFiberString(c.Method())))
		err := c.Next()

		if c.Path() == "/metrics" {
			return err
		}

		path := routePath(c)
		status := sanitizeLabelValue(strconv.Itoa(c.Response().StatusCode()))

		httpRequestsTotal.WithLabelValues(method, status, path).Inc()
		httpRequestDurationSeconds.WithLabelValues(method, status, path).Observe(time.Since(start).Seconds())
		return err
	}
}

// cloneFiberString copies a fasthttp/Fiber string that may share the request buffer.
func cloneFiberString(s string) string {
	return strings.Clone(s)
}

// canonicalHTTPMethod maps the request method to a stable label (RFC 7231 methods only; unknown → OTHER).
// Always returns a safe string (literal or newly allocated), never a fasthttp buffer view.
func canonicalHTTPMethod(m string) string {
	s := strings.ToUpper(strings.TrimSpace(m))
	switch s {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
		// Return a fresh copy: ToUpper may return the input unchanged for already-upper ASCII,
		// and that input may still be a fasthttp buffer view.
		return strings.Clone(s)
	case "POS":
		// Seen with some proxies/clients + fasthttp (truncated POST).
		return "POST"
	case "GETT":
		// Seen with fasthttp buffer reuse / truncated method reads.
		return "GET"
	default:
		if s == "" {
			return "UNKNOWN"
		}
		return "OTHER"
	}
}

func routePath(c fiber.Ctx) string {
	if r := c.Route(); r != nil && r.Path != "" {
		// Route templates are normally static, but clone defensively.
		return sanitizeLabelValue(cloneFiberString(r.Path))
	}
	return "_unmatched"
}

const maxLabelValueLen = 128

func sanitizeLabelValue(v string) string {
	v = strings.ToValidUTF8(strings.TrimSpace(v), "_")
	if v == "" {
		return "_empty"
	}
	if len(v) > maxLabelValueLen {
		v = v[:maxLabelValueLen]
	}
	// Ensure Prometheus never holds a fasthttp buffer view (ToValidUTF8/TrimSpace may share input).
	return strings.Clone(v)
}
