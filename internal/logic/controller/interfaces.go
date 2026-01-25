package controller

import "context"

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
}

// UseCase is the interface for controller use cases.
type UseCase interface {
	ReconcileCommand(ctx context.Context) error
	RunCommand(ctx context.Context) error
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
