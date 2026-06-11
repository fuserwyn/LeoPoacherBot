package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Database query latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	DatabaseErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "database_errors_total",
			Help: "Total number of database errors",
		},
		[]string{"operation", "error_type"},
	)

	BotUpdatesReceived = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "bot_updates_received_total",
			Help: "Total number of Telegram updates received",
		},
	)

	BotErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bot_errors_total",
			Help: "Total number of bot errors",
		},
		[]string{"error_type"},
	)

	ActiveUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_users",
			Help: "Current number of active users",
		},
	)

	PaymentRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_requests_total",
			Help: "Total number of payment requests",
		},
		[]string{"status"},
	)
)
