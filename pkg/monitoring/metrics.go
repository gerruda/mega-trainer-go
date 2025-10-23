package monitoring

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	MessagesProcessed prometheus.Counter
	MessagesFailed    prometheus.Counter
	CommandsProcessed *prometheus.CounterVec
	FSMSessions       prometheus.Gauge
	DatabaseQueries   *prometheus.CounterVec
	ResponseTime      prometheus.Histogram
}

func NewMetrics(serviceName string) *Metrics {
	labels := prometheus.Labels{"service": serviceName}

	return &Metrics{
		MessagesProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "messages_processed_total",
			Help:        "Total number of processed messages",
			ConstLabels: labels,
		}),

		MessagesFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "messages_failed_total",
			Help:        "Total number of failed messages",
			ConstLabels: labels,
		}),

		CommandsProcessed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "commands_processed_total",
			Help:        "Total number of processed commands",
			ConstLabels: labels,
		}, []string{"command"}),

		FSMSessions: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "fsm_sessions_active",
			Help:        "Number of active FSM sessions",
			ConstLabels: labels,
		}),

		DatabaseQueries: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "database_queries_total",
			Help:        "Total number of database queries",
			ConstLabels: labels,
		}, []string{"type", "status"}),

		ResponseTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "response_time_seconds",
			Help:        "Response time in seconds",
			ConstLabels: labels,
			Buckets:     prometheus.DefBuckets,
		}),
	}
}

func (m *Metrics) StartServer(port int) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		http.ListenAndServe(":"+strconv.Itoa(port), nil)
	}()
}

// Middleware для измерения времени
func (m *Metrics) MeasureRequest(start time.Time) {
	duration := time.Since(start).Seconds()
	m.ResponseTime.Observe(duration)
}
