package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	policy "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/skillcoder/preoomkiller-controller/internal/logic/controller"
)

const (
	evictionKind       = "Eviction"
	evictionAPIVersion = "policy/v1"
)

type adapter struct {
	logger           *slog.Logger
	clientset        kubernetes.Interface
	metricsClientset *metricsv.Clientset
}

// New creates a new K8s adapter.
func New(
	logger *slog.Logger,
	clientset kubernetes.Interface,
	metricsClientset *metricsv.Clientset,
) controller.Repository {
	return &adapter{
		logger:           logger,
		clientset:        clientset,
		metricsClientset: metricsClientset,
	}
}

var _ controller.Repository = (*adapter)(nil)

func (a *adapter) ListPodsQuery(
	ctx context.Context,
	labelSelector string,
) ([]controller.Pod, error) {
	podList, err := a.clientset.CoreV1().Pods("").List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	pods := make([]controller.Pod, 0, len(podList.Items))
	for i := range podList.Items {
		pods = append(pods, toDomainPod(&podList.Items[i]))
	}

	return pods, nil
}

func (a *adapter) GetPodMetricsQuery(
	ctx context.Context,
	namespace,
	name string,
) (*controller.PodMetrics, error) {
	podMetrics, err := a.metricsClientset.MetricsV1beta1().PodMetricses(namespace).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get pod metrics: %w", errPodNotFound)
		} else if apierrors.IsTooManyRequests(err) {
			return nil, fmt.Errorf("get pod metrics: %w", errTooManyRequests)
		}

		return nil, fmt.Errorf("get pod metrics: %w", err)
	}

	return toDomainPodMetrics(ctx, a.logger, podMetrics), nil
}

func (a *adapter) EvictPodCommand(
	ctx context.Context,
	namespace,
	name string,
) error {
	eviction := &policy.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: evictionAPIVersion,
			Kind:       evictionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	err := a.clientset.PolicyV1().Evictions(eviction.Namespace).Evict(ctx, eviction)
	if err != nil {
		switch {
		case apierrors.IsTooManyRequests(err):
			return fmt.Errorf("evict pod: %w", errTooManyRequests)
		case apierrors.IsNotFound(err):
			return fmt.Errorf("evict pod: %w", errPodNotFound)
		}

		return fmt.Errorf("evict pod: %w", err)
	}

	return nil
}

func (a *adapter) SetAnnotationCommand(
	ctx context.Context,
	namespace,
	name string,
	key,
	value string,
) error {
	annotations := map[string]any{key: value}
	if value == "" {
		annotations[key] = nil
	}

	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal annotation patch: %w", err)
	}

	_, err = a.clientset.CoreV1().Pods(namespace).Patch(
		ctx,
		name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("patch pod annotation: %w", err)
	}

	return nil
}
