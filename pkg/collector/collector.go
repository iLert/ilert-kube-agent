package collector

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iLert/ilert-kube-agent/pkg/storage"
)

// Collector definition
type Collector struct {
	storage            *storage.Storage
	alertsCreatedCount *prometheus.Desc
}

// NewCollector definition
func NewCollector(storage *storage.Storage) *Collector {
	return &Collector{
		storage: storage,
		alertsCreatedCount: prometheus.NewDesc(
			"ilert_alerts_created_count",
			"The total Bigquery job runs for uptime monitor log table",
			[]string{}, nil,
		),
	}
}

// Describe gets prometheus metrics description
func (collector *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.alertsCreatedCount
}

// Collect gets prometheus metrics collection
func (collector *Collector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(collector.alertsCreatedCount, prometheus.CounterValue, collector.storage.GetAlertsCreatedCount())
}
