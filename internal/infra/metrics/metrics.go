package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var evictionSkippedPodTooYoungTotal = promauto.With(prometheus.DefaultRegisterer).NewCounterVec(
	prometheus.CounterOpts{
		Name: "preoomkiller_eviction_skipped_pod_too_young_total",
		Help: "Total number of evictions skipped because pod age was below minimum " +
			"(possible misconfiguration or too-frequent restarts).",
	},
	[]string{"namespace", "pod"},
)

// RecordEvictionSkippedPodTooYoung increments the counter when an eviction is skipped
// because the pod was younger than the configured minimum age.
func RecordEvictionSkippedPodTooYoung(namespace, pod string) {
	evictionSkippedPodTooYoungTotal.WithLabelValues(namespace, pod).Inc()
}
