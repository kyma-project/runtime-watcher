package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	listenerRequestDuration            = "watcher_listener_request_duration"
	listenerRequests                   = "watcher_listener_requests_total"
	listenerRequestErrors              = "watcher_listener_request_errors_total"
	listenerInflightRequests           = "watcher_listener_inflight_requests_total"
	listenerExceedingSizeLimitRequests = "watcher_listener_exceeding_size_limit_requests_total"
	listenerFailedVerificationRequests = "watcher_listener_failed_verification_requests_total"
	requestURILabel                    = "request_uri_label"
	listenerService                    = "listener_server"
	serverNameLabel                    = "server_name"
)

var (
	registry                     prometheus.Registerer //nolint:gochecknoglobals//nolint:gochecknoglobals
	httpRequestDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: listenerRequestDuration,
		Help: "Indicates the latency of each request in seconds",
	}, []string{serverNameLabel}) //nolint:gochecknoglobals
	httpRequestsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerRequests,
		Help: "Indicates the number of requests",
	}, []string{serverNameLabel}) //nolint:gochecknoglobals
	httpRequestErrorsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerRequestErrors,
		Help: "Indicates the number of failed requests",
	}, []string{serverNameLabel}) //nolint:gochecknoglobals
	httpInflightRequestsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerInflightRequests,
		Help: "Indicates the number of inflight requests",
	}, []string{serverNameLabel}) //nolint:gochecknoglobals
	httpRequestsExceedingSizeLimitCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerExceedingSizeLimitRequests,
		Help: "Indicates the number of requests exceeding size limit",
	}, []string{serverNameLabel}) //nolint:gochecknoglobals
	httpFailedVerificationRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerFailedVerificationRequests,
		Help: "Indicates the number of requests that failed verification",
	}, []string{serverNameLabel, requestURILabel}) //nolint:gochecknoglobals
)

//nolint:gochecknoinits
func init() {
	registry.MustRegister(httpRequestDurationHistogram)
	registry.MustRegister(httpRequestsCounter)
	registry.MustRegister(httpRequestErrorsCounter)
	registry.MustRegister(httpInflightRequestsCounter)
	registry.MustRegister(httpRequestsExceedingSizeLimitCounter)
	registry.MustRegister(httpFailedVerificationRequests)
}

func UpdateMetrics(duration time.Duration) {
	recordHTTPRequestDuration(duration)
	recordHTTPRequests()
}

func recordHTTPRequestDuration(duration time.Duration) {
	httpRequestDurationHistogram.WithLabelValues(listenerService).Observe(duration.Seconds())
}

func recordHTTPRequests() {
	httpRequestsCounter.WithLabelValues(listenerService).Inc()
}

func RecordHTTPRequestErrors() {
	httpRequestErrorsCounter.WithLabelValues(listenerService).Inc()
}

func RecordHTTPInflightRequests(increaseBy float64) {
	httpInflightRequestsCounter.WithLabelValues(listenerService).Add(increaseBy)
}

func RecordHTTPRequestExceedingSizeLimit() {
	httpRequestsExceedingSizeLimitCounter.WithLabelValues(listenerService).Inc()
}

func RecordHttpFailedVerificationRequests(requestUri string) {
	httpFailedVerificationRequests.WithLabelValues(listenerService, requestUri).Inc()
}
