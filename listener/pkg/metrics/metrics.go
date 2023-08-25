//nolint:gochecknoglobals
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	listenerRequestDuration            = "watcher_listener_request_duration"
	listenerRequests                   = "watcher_listener_requests_total"
	listenerRequestErrors              = "watcher_listener_request_errors_total"
	listenerInflightRequests           = "watcher_listener_inflight_requests"
	listenerExceedingSizeLimitRequests = "watcher_listener_exceeding_size_limit_requests_total"
	listenerFailedVerificationRequests = "watcher_listener_failed_verification_requests_total"
	requestURILabel                    = "request_uri_label"
	listenerService                    = "listener"
	serverNameLabel                    = "server_name"
)

var (
	httpRequestDurationGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: listenerRequestDuration,
		Help: "Indicates the latency of each request in seconds",
	}, []string{serverNameLabel})
	httpRequestsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerRequests,
		Help: "Indicates the number of requests",
	}, []string{serverNameLabel})
	httpRequestErrorsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerRequestErrors,
		Help: "Indicates the number of failed requests",
	}, []string{serverNameLabel})
	HTTPInflightRequestsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: listenerInflightRequests,
		Help: "Indicates the number of inflight requests",
	}, []string{serverNameLabel})
	httpRequestsExceedingSizeLimitCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerExceedingSizeLimitRequests,
		Help: "Indicates the number of requests exceeding size limit",
	}, []string{serverNameLabel})
	httpFailedVerificationRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerFailedVerificationRequests,
		Help: "Indicates the number of requests that failed verification",
	}, []string{serverNameLabel, requestURILabel})
)

func Init(metricsRegistry prometheus.Registerer) {
	metricsRegistry.MustRegister(httpRequestDurationGauge)
	metricsRegistry.MustRegister(httpRequestsCounter)
	metricsRegistry.MustRegister(httpRequestErrorsCounter)
	metricsRegistry.MustRegister(HTTPInflightRequestsGauge)
	metricsRegistry.MustRegister(httpRequestsExceedingSizeLimitCounter)
	metricsRegistry.MustRegister(httpFailedVerificationRequests)
}

func UpdateHTTPRequestMetrics(duration time.Duration) {
	recordHTTPRequestDuration(duration)
	recordHTTPRequests()
}

func RecordHTTPRequestErrors() {
	httpRequestErrorsCounter.WithLabelValues(listenerService).Inc()
}

func RecordHTTPInflightRequests(increaseBy float64) {
	HTTPInflightRequestsGauge.WithLabelValues(listenerService).Add(increaseBy)
}

func RecordHTTPRequestExceedingSizeLimit() {
	httpRequestsExceedingSizeLimitCounter.WithLabelValues(listenerService).Inc()
}

func RecordHTTPFailedVerificationRequests(requestURI string) {
	httpFailedVerificationRequests.WithLabelValues(listenerService, requestURI).Inc()
}

func recordHTTPRequestDuration(duration time.Duration) {
	httpRequestDurationGauge.WithLabelValues(listenerService).Set(duration.Seconds())
}

func recordHTTPRequests() {
	httpRequestsCounter.WithLabelValues(listenerService).Inc()
}
