package watchermetrics

import "github.com/prometheus/client_golang/prometheus"

type WatcherMetrics struct {
	someGauge *prometheus.GaugeVec
}

const (
	watcherSomething = "watcher_something"
)

func NewMetrics() *WatcherMetrics {
	metrics := &WatcherMetrics{
		someGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: watcherSomething,
			Help: "Does something",
		}, []string{"some_label"}),
	}

	return metrics
}

func (p *WatcherMetrics) UpdateSomething(someLabel string, someValue float64) {
	p.someGauge.With(prometheus.Labels{
		"some_label": someLabel,
	}).Set(someValue)
}
