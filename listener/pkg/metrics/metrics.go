package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	listenerRequestDuration            = "watcher_listener_request_duration"
	listenerRequests                   = "watcher_listener_requests"
	listenerRequestErrors              = "watcher_listener_request_error"
	listenerInflightRequests           = "watcher_listener_inflight_requests"
	listenerExceedingSizeLimitRequests = "watcher_listener_exceeding_size_limit_requests"
	listenerFailedVerificationRequests = "watcher_listener_failed_verification_requests"
	requestUriLabel                    = "request_uri_label"
	listenerService                    = "listener_server"
)

var (
	Registry                     prometheus.Registerer
	httpRequestDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: listenerRequestDuration,
		Help: "Indicates the latency of each request in seconds",
	}, []string{requestUriLabel})
	httpRequestsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerRequests,
		Help: "Indicates the number of requests",
	}, []string{requestUriLabel})
	httpRequestErrorsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerRequestErrors,
		Help: "Indicates the number of failed requests",
	}, []string{requestUriLabel})
	httpInflightRequestsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerInflightRequests,
		Help: "Indicates the number of inflight requests",
	}, []string{requestUriLabel})
	httpRequestsExceedingSizeLimitCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerExceedingSizeLimitRequests,
		Help: "Indicates the number of requests exceeding size limit",
	}, []string{requestUriLabel})
	httpFailedVerificationRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerFailedVerificationRequests,
		Help: "Indicates the number of requests that failed verification",
	}, []string{requestUriLabel})
)

func init() {
	Registry.MustRegister(httpRequestDurationHistogram)
	Registry.MustRegister(httpRequestsCounter)
	Registry.MustRegister(httpRequestErrorsCounter)
	Registry.MustRegister(httpInflightRequestsCounter)
	Registry.MustRegister(httpRequestsExceedingSizeLimitCounter)
	Registry.MustRegister(httpFailedVerificationRequests)
}

func UpdateMetrics(duration time.Duration) {
	recordHttpRequestDuration(duration)
	recordHttpRequests()
}

func recordHttpRequestDuration(duration time.Duration) {
	httpRequestDurationHistogram.WithLabelValues(listenerService).Observe(duration.Seconds())
}

func recordHttpRequests() {
	httpRequestsCounter.WithLabelValues(listenerService).Inc()
}

func RecordHttpRequestErrors() {
	httpRequestErrorsCounter.WithLabelValues(listenerService).Inc()
}

func RecordHttpInflightRequests(increaseBy float64) {
	httpInflightRequestsCounter.WithLabelValues(listenerService).Add(increaseBy)
}

func RecordHttpRequestExceedingSizeLimit() {
	httpRequestsExceedingSizeLimitCounter.WithLabelValues(listenerService).Inc()
}

func RecordHttpFailedVerificationRequests(requestUri string) {
	httpFailedVerificationRequests.WithLabelValues(listenerService, requestUri).Inc()
}
