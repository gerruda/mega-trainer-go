// Package monitoring exposes the metrics registry and the HTTP server used for
// Prometheus scraping and Kubernetes-style health probes.
package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"mega-trainer-go/internal/config"
)

// Metrics is the single set of application metrics. It is created once in main
// and passed explicitly to its consumers, so metrics registration cannot panic
// on a second instantiation.
type Metrics struct {
	UpdatesProcessed prometheus.Counter
	UpdatesFailed    prometheus.Counter
	UpdateDuration   prometheus.Histogram
}

// NewMetrics creates and registers the application metrics on the default
// Prometheus registry, which is served by the /metrics endpoint.
func NewMetrics(namespace string) *Metrics {
	if namespace == "" {
		namespace = "mega_trainer"
	}

	metrics := &Metrics{
		UpdatesProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "telegram_updates_processed_total",
			Help:      "Total number of successfully processed Telegram updates.",
		}),
		UpdatesFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "telegram_updates_failed_total",
			Help:      "Total number of Telegram updates that could not be processed.",
		}),
		UpdateDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "telegram_update_duration_seconds",
			Help:      "Time spent processing a single Telegram update.",
			Buckets:   prometheus.DefBuckets,
		}),
	}

	prometheus.MustRegister(metrics.UpdatesProcessed, metrics.UpdatesFailed, metrics.UpdateDuration)
	return metrics
}

// ReadyCheck is a dependency probe used by the readiness endpoint.
type ReadyCheck func(ctx context.Context) error

// Server is a small HTTP server exposing /metrics, /healthz and /readyz.
type Server struct {
	srv    *http.Server
	log    zerolog.Logger
	checks []ReadyCheck
}

// NewServer builds the monitoring server. The listener is only opened on Start.
func NewServer(cfg config.MonitoringConfig, metrics *Metrics, log zerolog.Logger, checks ...ReadyCheck) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	s := &Server{log: log, checks: checks}
	mux.HandleFunc("/readyz", s.handleReady)

	s.srv = &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

// Start runs the server in the background. Errors are logged, because there is
// no caller that could react to them after startup.
func (s *Server) Start() {
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error().Err(err).Str("addr", s.srv.Addr).Msg("monitoring server stopped unexpectedly")
		}
	}()
	s.log.Info().Str("addr", s.srv.Addr).Msg("monitoring server started")
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown monitoring server: %w", err)
	}
	return nil
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	for _, check := range s.checks {
		if err := check(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  err.Error(),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
