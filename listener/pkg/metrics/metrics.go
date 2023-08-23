package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	listenerRequestDuration            = "watcher_listener_request_duration"
	listenerRequestRate                = "watcher_listener_request_rate"
	listenerRequestErrors              = "watcher_listener_request_error"
	listenerInflightRequests           = "watcher_listener_inflight_requests"
	listenerExceedingSizeLimitRequests = "watcher_listener_exceeding_size_limit_requests"
	listenerFailedVerificationRequests = "watcher_listener_failed_verification_requests"
	requestUriLabel                    = "request_uri_label"
)

var (
	Registry                     prometheus.Registerer
	httpRequestDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: listenerRequestDuration,
		Help: "Indicates the latency of each request in seconds",
	}, []string{requestUriLabel})
	httpRequestRateCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerRequestRate,
		Help: "Indicates the number of requests per second",
	}, []string{requestUriLabel})
	httpRequestErrorsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerRequestErrors,
		Help: "Indicates the number of failed requests per second",
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

func Initialize() {
	Registry.MustRegister(httpRequestDurationHistogram)
	Registry.MustRegister(httpRequestRateCounter)
	Registry.MustRegister(httpRequestErrorsCounter)
	Registry.MustRegister(httpInflightRequestsCounter)
	Registry.MustRegister(httpRequestsExceedingSizeLimitCounter)
	Registry.MustRegister(httpFailedVerificationRequests)
}

func UpdateMetrics(requestUri string, duration time.Duration) {
	recordHttpRequestDuration(requestUri, duration)
	recordHttpRequestRate(requestUri)
}

func recordHttpRequestDuration(requestUri string, duration time.Duration) {
	httpRequestDurationHistogram.WithLabelValues(requestUri).Observe(duration.Seconds())
}

func recordHttpRequestRate(requestUri string) {
	// TODO: THIS IS ONLY COUNT NOT COUNT PER SECOND
	httpRequestRateCounter.WithLabelValues(requestUri).Inc()
}

func RecordHttpRequestErrors(requestUri string) {
	// TODO: THIS IS ONLY COUNT NOT COUNT PER SECOND
	httpRequestErrorsCounter.WithLabelValues(requestUri).Inc()
}

func RecordHttpInflightRequests(requestUri string, increaseBy float64) {
	httpInflightRequestsCounter.WithLabelValues(requestUri).Add(increaseBy)
}

func RecordHttpRequestExceedingSizeLimit(requestUri string) {
	httpRequestsExceedingSizeLimitCounter.WithLabelValues(requestUri).Inc()
}

func RecordHttpFailedVerificationRequests(requestUri string) {
	httpFailedVerificationRequests.WithLabelValues(requestUri).Inc()
}
