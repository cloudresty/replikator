package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MirroredAssetsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "replikator_mirrored_assets_total",
			Help: "Total number of mirrored assets by state (active, stale, failed)",
		},
		[]string{"state", "namespace"},
	)

	SyncErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "replikator_sync_errors_total",
			Help: "Total number of sync errors by error type",
		},
		[]string{"type"},
	)

	SourcesTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "replikator_sources_total",
			Help: "Total number of sources by state (enabled, disabled, error)",
		},
		[]string{"state", "namespace"},
	)

	ReflectionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "replikator_reflection_duration_seconds",
			Help:    "Duration of reflection operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		},
		[]string{"source_namespace", "source_name", "mirror_namespace"},
	)

	LastSyncTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "replikator_last_sync_timestamp",
			Help: "Unix timestamp of last successful sync for each source",
		},
		[]string{"source_namespace", "source_name"},
	)

	AutoMirrorsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "replikator_auto_mirrors_total",
			Help: "Total number of auto-created mirrors by namespace",
		},
		[]string{"namespace"},
	)

	ManualMirrorsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "replikator_manual_mirrors_total",
			Help: "Total number of manually created mirrors by namespace",
		},
		[]string{"namespace"},
	)

	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "replikator_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "replikator_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache"},
	)

	WatchSessionRestarts = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "replikator_watch_session_restarts_total",
			Help: "Total number of watch session restarts due to timeout",
		},
	)

	ReconciliationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "replikator_reconciliation_duration_seconds",
			Help:    "Duration of reconciliation operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 12),
		},
		[]string{"controller", "result"},
	)

	ActiveWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "replikator_active_workers",
			Help: "Number of currently active worker goroutines",
		},
	)
)

func RecordReflectionSuccess(sourceNamespace, sourceName, mirrorNamespace string, duration time.Duration) {
	ReflectionDuration.WithLabelValues(sourceNamespace, sourceName, mirrorNamespace).Observe(duration.Seconds())
	LastSyncTimestamp.WithLabelValues(sourceNamespace, sourceName).Set(float64(time.Now().Unix()))
	MirroredAssetsTotal.WithLabelValues("active", mirrorNamespace).Inc()
	SyncErrorsTotal.WithLabelValues("none").Inc()
}

func RecordReflectionError(sourceNamespace, sourceName, mirrorNamespace string, errType string) {
	SyncErrorsTotal.WithLabelValues(errType).Inc()
	MirroredAssetsTotal.WithLabelValues("failed", mirrorNamespace).Inc()
}

func RecordSourceEnabled(namespace string) {
	SourcesTotal.WithLabelValues("enabled", namespace).Inc()
}

func RecordSourceDisabled(namespace string) {
	SourcesTotal.WithLabelValues("disabled", namespace).Inc()
}

func RecordAutoMirrorCreated(namespace string) {
	AutoMirrorsTotal.WithLabelValues(namespace).Inc()
}

func RecordManualMirrorCreated(namespace string) {
	ManualMirrorsTotal.WithLabelValues(namespace).Inc()
}

func RecordCacheHit(cacheName string) {
	CacheHits.WithLabelValues(cacheName).Inc()
}

func RecordCacheMiss(cacheName string) {
	CacheMisses.WithLabelValues(cacheName).Inc()
}

func RecordWatchSessionRestart() {
	WatchSessionRestarts.Inc()
}

func RecordReconciliationDuration(controller, result string, duration time.Duration) {
	ReconciliationDuration.WithLabelValues(controller, result).Observe(duration.Seconds())
}
