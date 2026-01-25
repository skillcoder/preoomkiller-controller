package k8s

import (
	"context"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/skillcoder/preoomkiller-controller/internal/logic/controller"
)

func toDomainPod(pod *corev1.Pod) controller.Pod {
	return controller.Pod{
		Name:        pod.Name,
		Namespace:   pod.Namespace,
		Annotations: pod.Annotations,
	}
}

func toDomainPodMetrics(
	ctx context.Context,
	logger *slog.Logger,
	podMetrics *metricsv1beta1.PodMetrics,
) *controller.PodMetrics {
	memoryUsage := resource.NewQuantity(0, resource.BinarySI)

	for i := range podMetrics.Containers {
		containerMemoryUsage := podMetrics.Containers[i].Usage.Memory()
		if containerMemoryUsage == nil {
			logger.WarnContext(ctx, "container memory usage is nil, skipping",
				"pod", podMetrics.Name,
				"namespace", podMetrics.Namespace,
				"container", podMetrics.Containers[i].Name,
			)

			continue
		}

		memoryUsage.Add(*containerMemoryUsage)
		logger.DebugContext(ctx, "container metrics",
			"pod", podMetrics.Name,
			"namespace", podMetrics.Namespace,
			"container", podMetrics.Containers[i].Name,
			"memory", containerMemoryUsage.String(),
		)
	}

	return &controller.PodMetrics{
		MemoryUsage: memoryUsage,
	}
}
