package controller

const (
	PreoomkillerPodLabelSelector             = "preoomkiller.beta.k8s.skillcoder.com/enabled=true"
	PreoomkillerAnnotationMemoryThresholdKey = "preoomkiller.beta.k8s.skillcoder.com/memory-threshold"
	PreoomkillerAnnotationRestartScheduleKey = "preoomkiller.beta.k8s.skillcoder.com/restart-schedule"
	PreoomkillerAnnotationTZKey              = "preoomkiller.beta.k8s.skillcoder.com/tz"
	PreoomkillerAnnotationRestartAtKey       = "preoomkiller.beta.k8s.skillcoder.com/restart-at"

	// percentScale is the divisor for percentage values (e.g. 80% -> 80/100).
	percentScale = 100
)
