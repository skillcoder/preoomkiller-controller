package config

// Env key constants. All controller configuration env vars use PREOOMKILLER_ prefix;
// duration values are in seconds and use _SEC suffix.

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

// Reconciliation interval in seconds.
const envKeyIntervalSec = "PREOOMKILLER_INTERVAL_SEC"

// Pinger check interval in seconds.
const envKeyPingerIntervalSec = "PREOOMKILLER_PINGER_INTERVAL_SEC"

// Max jitter in seconds added to scheduled eviction time.
const envKeyRestartScheduleJitterMaxSec = "PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX_SEC"

// Minimum pod age in seconds before eviction is allowed; 0 disables the check.
const envKeyMinPodAgeBeforeEvictionSec = "PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION_SEC"

// Standard k8s env keys used as fallback when PREOOMKILLER_* are unset.
const (
	envKeyKubeConfigFallback = "KUBECONFIG"
	envKeyKubeMasterFallback = "KUBERNETES_MASTER"
)
