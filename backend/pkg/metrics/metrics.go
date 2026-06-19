package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTP request metrics
var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests processed, labeled by method, path, status.",
	}, []string{"method", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "status"})
)

// Payment / webhook metrics
var (
	WebhooksReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhooks_received_total",
		Help: "Total webhooks received, labeled by provider and outcome.",
	}, []string{"provider", "outcome"}) // outcome: accepted | invalid_signature | malformed | duplicate | rejected

	WebhookProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webhook_processing_duration_seconds",
		Help:    "Time to process a webhook end-to-end.",
		Buckets: []float64{.005, .01, .05, .1, .5, 1, 5},
	}, []string{"provider", "event_type"})

	PaymentsCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payments_created_total",
		Help: "Total payment orders created.",
	}, []string{"mock"}) // mock: true|false

	PaymentsCapturedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payments_captured_total",
		Help: "Total payments successfully captured.",
	}, []string{"method"})

	PaymentsFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payments_failed_total",
		Help: "Total payments that failed.",
	}, []string{"reason"})

	WebhookDLQDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "webhook_dlq_depth",
		Help: "Current number of events parked in the webhook dead-letter queue.",
	})
)

// Auth / login metrics
var (
	LoginAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "login_attempts_total",
		Help: "Login attempts by outcome.",
	}, []string{"outcome"}) // success | invalid_credentials | locked | unverified
)

// HTTPMiddleware records request count + duration. Wraps any http.Handler.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r)
		dur := time.Since(start).Seconds()
		labels := prometheus.Labels{
			"method": r.Method,
			"status": strconv.Itoa(ww.status),
		}
		HTTPRequestsTotal.With(labels).Inc()
		HTTPRequestDuration.With(labels).Observe(dur)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush delegates to the wrapped writer so SSE / streaming handlers can
// still flush headers + body chunks through this middleware.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the underlying writer so http.ResponseController (used to
// disable per-request write deadlines for SSE) can reach it.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

// Handler returns an http.Handler for /metrics with optional bearer-token auth.
// Empty token → public (only for dev / behind VPN). In production, set
// METRICS_TOKEN so only the scraper can read.
func Handler(token string) http.Handler {
	base := promhttp.Handler()
	if token == "" {
		return base
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + token
		if auth != expected {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		base.ServeHTTP(w, r)
	})
}
