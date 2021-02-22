package collector

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iLert/ilert-kube-agent/pkg/storage"
)

// Collector definition
type Collector struct {
	storage               *storage.Storage
	incidentsCreatedCount *prometheus.Desc
}

// NewCollector definition
func NewCollector(storage *storage.Storage) *Collector {
	return &Collector{
		storage: storage,
		incidentsCreatedCount: prometheus.NewDesc(
			"ilert_incidents_created_count",
			"The total Bigquery job runs for uptime monitor log table",
			[]string{}, nil,
		),
	}
}

// Describe gets prometheus metrics description
func (collector *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.incidentsCreatedCount
}

// Collect gets prometheus metrics collection
func (collector *Collector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(collector.incidentsCreatedCount, prometheus.CounterValue, collector.storage.GetIncidentsCreatedCount())
}
