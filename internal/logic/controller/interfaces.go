package controller

import (
	"context"
	"time"
)

// Repository is the port interface for K8s operations.
// Implementations are provided by adapters in the outbound layer.
type Repository interface {
	ListPodsQuery(
		ctx context.Context,
		labelSelector string,
	) ([]Pod, error)

	GetPodMetricsQuery(
		ctx context.Context,
		namespace,
		name string,
	) (*PodMetrics, error)

	EvictPodCommand(
		ctx context.Context,
		namespace,
		name string,
	) error

	// SetAnnotationCommand sets (or removes when value is empty) a single annotation on the given pod via a merge-patch.
	SetAnnotationCommand(
		ctx context.Context,
		namespace,
		name,
		key,
		value string,
	) error
}

// scheduleParser computes the next cron occurrence. Implemented by infra/cronparser using go-cron.
type scheduleParser interface {
	NextAfter(spec, tz string, after time.Time) (time.Time, error)
}

// notFound is a private interface for checking "not found" errors
// without importing the adapter package.
type notFound interface {
	IsNotFound()
}

// tooManyRequests is a private interface for checking "too many requests" errors
// without importing the adapter package.
type tooManyRequests interface {
	IsTooManyRequests()
}
