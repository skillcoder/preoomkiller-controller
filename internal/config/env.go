package config

import "time"

// Env key constants. All controller configuration env vars use PREOOMKILLER_ prefix;
// duration values support explicit units (e.g. 5m, 40s, 2h).

// Path to kubeconfig file. If unset, KUBECONFIG is used as fallback.
const envKeyKubeConfig = "PREOOMKILLER_KUBECONFIG"

// Kubernetes API server URL. If unset, KUBERNETES_MASTER is used as fallback.
const envKeyKubeMaster = "PREOOMKILLER_KUBE_MASTER"

// Log level: debug, info, warn, error.
const envKeyLogLevel = "PREOOMKILLER_LOG_LEVEL"

// Log format: json or text.
const envKeyLogFormat = "PREOOMKILLER_LOG_FORMAT"

// Port for health/readiness HTTP server.
const envKeyHTTPPort = "PREOOMKILLER_HTTP_PORT"

// Port for Prometheus metrics (GET /metrics).
const envKeyMetricsPort = "PREOOMKILLER_METRICS_PORT"

// Label selector to list pods (e.g. preoomkiller.beta.k8s.skillcoder.com/enabled=true).
const envKeyPodLabelSelector = "PREOOMKILLER_POD_LABEL_SELECTOR"

// Annotation key for memory threshold on pod metadata.
const envKeyAnnotationMemoryThreshold = "PREOOMKILLER_ANNOTATION_MEMORY_THRESHOLD"

// Annotation key for scheduled restart cron expression.
const envKeyAnnotationRestartSchedule = "PREOOMKILLER_ANNOTATION_RESTART_SCHEDULE"

// Annotation key for schedule timezone (IANA, e.g. America/New_York).
const envKeyAnnotationTZ = "PREOOMKILLER_ANNOTATION_TZ"

// Reconciliation interval. Units: s, m, h (e.g. 300s, 5m).
const (
	envKeyInterval = "PREOOMKILLER_INTERVAL"
	envMinInterval = 30 * time.Second
)

// Pinger check interval. Units: s, m, h (e.g. 10s, 1m).
const (
	envKeyPingerInterval = "PREOOMKILLER_PINGER_INTERVAL"
	envMinPingerInterval = time.Second
)

// Max jitter added to scheduled eviction time. Units: s, m, h (e.g. 30s).
const (
	envKeyRestartScheduleJitterMax = "PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX"
	envMinRestartScheduleJitterMax = time.Second
)

// Minimum pod age before eviction is allowed; 0 disables the check. Units: s, m, h (e.g. 30m).
const (
	envKeyMinPodAgeBeforeEviction = "PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION"
	envMinMinPodAgeBeforeEviction = time.Minute
)

// Standard k8s env keys used as fallback when PREOOMKILLER_* are unset.
const (
	envKeyKubeConfigFallback = "KUBECONFIG"
	envKeyKubeMasterFallback = "KUBERNETES_MASTER"
)
