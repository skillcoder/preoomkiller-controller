package controller

import "k8s.io/apimachinery/pkg/api/resource"

// Pod represents a Kubernetes pod in the domain layer.
type Pod struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	// MemoryLimit is the sum of all container memory limits; nil when no container sets a limit.
	MemoryLimit *resource.Quantity
}

// PodMetrics represents pod metrics in the domain layer.
type PodMetrics struct {
	MemoryUsage *resource.Quantity
}

// ContainerMetrics represents container metrics in the domain layer.
type ContainerMetrics struct {
	Name        string
	MemoryUsage *resource.Quantity
	CPUUsage    *resource.Quantity
}
