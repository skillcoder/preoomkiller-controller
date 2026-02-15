package controller

const (
	PreoomkillerPodLabelSelector             = "preoomkiller.beta.k8s.skillcoder.com/enabled=true"
	PreoomkillerAnnotationMemoryThresholdKey = "preoomkiller.beta.k8s.skillcoder.com/memory-threshold"

	// percentScale is the divisor for percentage values (e.g. 80% -> 80/100).
	percentScale = 100
)
