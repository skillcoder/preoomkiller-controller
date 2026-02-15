package controller

import (
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Pod represents a Kubernetes pod in the domain layer.
type Pod struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	// MemoryLimit is the sum of all container memory limits; nil when no container sets a limit.
	MemoryLimit *resource.Quantity
	// CreatedAt is the pod creation timestamp; used to detect missed scheduled restarts after controller downtime.
	CreatedAt time.Time
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
