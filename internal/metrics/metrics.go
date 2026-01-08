package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ScanMetrics holds all Prometheus metrics for scan operations
type ScanMetrics struct {
	// Wallet scan metrics
	WalletScansTotal      *prometheus.CounterVec
	WalletScanDuration    *prometheus.HistogramVec
	WalletScanSuccessTotal *prometheus.CounterVec
	WalletScanErrorTotal  *prometheus.CounterVec

	// TLS scan metrics
	TLSScansTotal      *prometheus.CounterVec
	TLSScanDuration    *prometheus.HistogramVec
	TLSScanSuccessTotal *prometheus.CounterVec
	TLSScanErrorTotal  *prometheus.CounterVec
}

var (
	// Default instance - initialized on first use
	defaultMetrics *ScanMetrics
)

// Init initializes the default metrics instance
// This should be called once during application startup
func Init() *ScanMetrics {
	if defaultMetrics != nil {
		return defaultMetrics
	}

	defaultMetrics = &ScanMetrics{
		// Wallet scan metrics
		WalletScansTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cafe_discovery_wallet_scans_total",
				Help: "Total number of wallet scans performed",
			},
			[]string{"scan_type"}, // scan_type: wallet
		),
		WalletScanDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cafe_discovery_wallet_scan_duration_seconds",
				Help:    "Duration of wallet scans in seconds",
				Buckets: prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
			},
			[]string{"scan_type"}, // scan_type: wallet
		),
		WalletScanSuccessTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cafe_discovery_wallet_scan_success_total",
				Help: "Total number of successful wallet scans",
			},
			[]string{"scan_type", "result"}, // scan_type: wallet, result: success
		),
		WalletScanErrorTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cafe_discovery_wallet_scan_error_total",
				Help: "Total number of failed wallet scans",
			},
			[]string{"scan_type", "result"}, // scan_type: wallet, result: failure
		),

		// TLS scan metrics
		TLSScansTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cafe_discovery_tls_scans_total",
				Help: "Total number of TLS scans performed",
			},
			[]string{"scan_type"}, // scan_type: tls
		),
		TLSScanDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cafe_discovery_tls_scan_duration_seconds",
				Help:    "Duration of TLS scans in seconds",
				Buckets: prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
			},
			[]string{"scan_type"}, // scan_type: tls
		),
		TLSScanSuccessTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cafe_discovery_tls_scan_success_total",
				Help: "Total number of successful TLS scans",
			},
			[]string{"scan_type", "result"}, // scan_type: tls, result: success
		),
		TLSScanErrorTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cafe_discovery_tls_scan_error_total",
				Help: "Total number of failed TLS scans",
			},
			[]string{"scan_type", "result"}, // scan_type: tls, result: failure
		),
	}

	return defaultMetrics
}

// Get returns the default metrics instance, initializing it if necessary
func Get() *ScanMetrics {
	if defaultMetrics == nil {
		return Init()
	}
	return defaultMetrics
}

// RecordWalletScan records metrics for a wallet scan operation
// This is a convenience method that records all related metrics atomically
func (m *ScanMetrics) RecordWalletScan(duration time.Duration, success bool) {
	scanType := "wallet"
	
	// Increment total scans counter
	m.WalletScansTotal.WithLabelValues(scanType).Inc()
	
	// Record duration
	m.WalletScanDuration.WithLabelValues(scanType).Observe(duration.Seconds())
	
	// Record success or error
	if success {
		m.WalletScanSuccessTotal.WithLabelValues(scanType, "success").Inc()
	} else {
		m.WalletScanErrorTotal.WithLabelValues(scanType, "failure").Inc()
	}
}

// RecordTLSScan records metrics for a TLS scan operation
// This is a convenience method that records all related metrics atomically
func (m *ScanMetrics) RecordTLSScan(duration time.Duration, success bool) {
	scanType := "tls"
	
	// Increment total scans counter
	m.TLSScansTotal.WithLabelValues(scanType).Inc()
	
	// Record duration
	m.TLSScanDuration.WithLabelValues(scanType).Observe(duration.Seconds())
	
	// Record success or error
	if success {
		m.TLSScanSuccessTotal.WithLabelValues(scanType, "success").Inc()
	} else {
		m.TLSScanErrorTotal.WithLabelValues(scanType, "failure").Inc()
	}
}
