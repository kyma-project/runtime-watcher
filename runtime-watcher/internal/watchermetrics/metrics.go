package watchermetrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type WatcherMetrics struct {
	requestDurationGauge               prometheus.Gauge
	admissionRequestsErrorTotalCounter prometheus.Counter
	admissionRequestsTotalCounter      prometheus.Counter
	kcpRequestsTotalCounter            prometheus.Counter
	failedKCPRequestsTotalCounter      *prometheus.CounterVec
}

const (
	RequestDuration                          = "watcher_request_duration"
	FailedKCPRequestsTotal                   = "watcher_failed_kcp_total"
	KcpRequestsTotal                         = "watcher_kcp_requests_total"
	AdmissionRequestsErrorTotal              = "watcher_admission_request_error_total"
	AdmissionRequestsTotal                   = "watcher_admission_request_total"
	kcpErrReasonLabel                        = "error_reason"
	ReasonSubresource           KcpErrReason = "invalid-subresource"
	ReasonOwner                 KcpErrReason = "unknown-owner"
	ReasonKcpAddress            KcpErrReason = "missing-address-or-contract"
	ReasonRequest               KcpErrReason = "request-setup"
	ReasonResponse              KcpErrReason = "failed-request"
)

type KcpErrReason string

func NewMetrics() *WatcherMetrics {
	metrics := &WatcherMetrics{
		requestDurationGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: RequestDuration,
			Help: "Indicates average request handling duration",
		}),
		admissionRequestsErrorTotalCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: AdmissionRequestsErrorTotal,
			Help: "Indicates total admission requests parsing error count",
		}),
		admissionRequestsTotalCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: AdmissionRequestsTotal,
			Help: "Indicates total incoming admission requests count",
		}),
		kcpRequestsTotalCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: KcpRequestsTotal,
			Help: "Indicates total requests to KCP count",
		}),
		failedKCPRequestsTotalCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: FailedKCPRequestsTotal,
			Help: "Indicates total failed requests to KCP count",
		}, []string{kcpErrReasonLabel}),
	}
	return metrics
}

func (w *WatcherMetrics) RegisterAll() {
	prometheus.MustRegister(w.requestDurationGauge)
	prometheus.MustRegister(w.admissionRequestsErrorTotalCounter)
	prometheus.MustRegister(w.admissionRequestsTotalCounter)
	prometheus.MustRegister(w.kcpRequestsTotalCounter)
	prometheus.MustRegister(w.failedKCPRequestsTotalCounter)
}

func (w *WatcherMetrics) UpdateRequestDuration(duration time.Duration) {
	w.requestDurationGauge.Set(duration.Seconds())
}

func (w *WatcherMetrics) UpdateFailedKCPTotal(reason KcpErrReason) {
	w.failedKCPRequestsTotalCounter.With(prometheus.Labels{
		kcpErrReasonLabel: string(reason),
	}).Inc()
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
