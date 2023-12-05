package watchermetrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type WatcherMetrics struct {
	requestDurationGauge               prometheus.Gauge
	failedKCPRequestsTotalCounter      prometheus.Counter
	kcpRequestsTotalCounter            prometheus.Counter
	admissionRequestsErrorTotalCounter prometheus.Counter
	admissionRequestsTotalCounter      prometheus.Counter
}

const (
	requestDuration             = "watcher_request_duration"
	failedKCPRequestsTotal      = "watcher_failed_kcp_total"
	kcpRequestsTotal            = "watcher_kcp_requests_total"
	admissionRequestsErrorTotal = "watcher_admission_request_error_total"
	admissionRequestsTotal      = "watcher_admission_request_total"
)

func NewMetrics() *WatcherMetrics {
	metrics := &WatcherMetrics{
		requestDurationGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: requestDuration,
			Help: "Indicates average request handling duration",
		}),
		failedKCPRequestsTotalCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: failedKCPRequestsTotal,
			Help: "Indicates total failed requests to KCP count",
		}),
		kcpRequestsTotalCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: kcpRequestsTotal,
			Help: "Indicates total requests to KCP count",
		}),
		admissionRequestsErrorTotalCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: admissionRequestsErrorTotal,
			Help: "Indicates total admission requests parsing error count",
		}),
		admissionRequestsTotalCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: admissionRequestsTotal,
			Help: "Indicates total incoming admission requests count",
		}),
	}

	return metrics
}

func (w *WatcherMetrics) UpdateRequestDuration(duration time.Duration) {
	w.requestDurationGauge.Set(duration.Seconds())
}

func (w *WatcherMetrics) UpdateFailedKCPTotal() {
	w.failedKCPRequestsTotalCounter.Inc()
}

func (w *WatcherMetrics) UpdateKCPTotal() {
	w.kcpRequestsTotalCounter.Inc()
}

func (w *WatcherMetrics) UpdateAdmissionRequestsErrorTotal() {
	w.admissionRequestsErrorTotalCounter.Inc()
}

func (w *WatcherMetrics) UpdateAdmissionRequestsTotal() {
	w.admissionRequestsTotalCounter.Inc()
}
