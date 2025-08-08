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
	listenerEventsProcessed            = "watcher_listener_events_processed_total"
	listenerEventsDropped              = "watcher_listener_events_dropped_total"
	listenerEventConversion            = "watcher_listener_event_conversion_duration"
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
	eventsProcessedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerEventsProcessed,
		Help: "Total number of events successfully processed and forwarded",
	}, []string{serverNameLabel, "owner"})
	eventsDroppedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: listenerEventsDropped,
		Help: "Total number of events dropped due to various reasons",
	}, []string{serverNameLabel, "reason"})
	eventConversionDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    listenerEventConversion,
		Help:    "Time taken to convert and forward events",
		Buckets: prometheus.DefBuckets,
	}, []string{serverNameLabel})
)

func Init(metricsRegistry prometheus.Registerer) {
	metricsRegistry.MustRegister(httpRequestDurationGauge)
	metricsRegistry.MustRegister(httpRequestsCounter)
	metricsRegistry.MustRegister(httpRequestErrorsCounter)
	metricsRegistry.MustRegister(HTTPInflightRequestsGauge)
	metricsRegistry.MustRegister(httpRequestsExceedingSizeLimitCounter)
	metricsRegistry.MustRegister(httpFailedVerificationRequests)
	metricsRegistry.MustRegister(eventsProcessedCounter)
	metricsRegistry.MustRegister(eventsDroppedCounter)
	metricsRegistry.MustRegister(eventConversionDurationHistogram)
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

func RecordEventProcessed(owner string) {
	eventsProcessedCounter.WithLabelValues(listenerService, owner).Inc()
}

func RecordEventDropped(reason string) {
	eventsDroppedCounter.WithLabelValues(listenerService, reason).Inc()
}

func RecordEventConversionDuration(duration time.Duration) {
	eventConversionDurationHistogram.WithLabelValues(listenerService).Observe(duration.Seconds())
}

func recordHTTPRequests() {
	httpRequestsCounter.WithLabelValues(listenerService).Inc()
}
