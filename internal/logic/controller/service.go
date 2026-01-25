package controller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

type service struct {
	logger   *slog.Logger
	repo     Repository
	interval time.Duration
}

// NewService creates a new controller service.
func NewService(
	logger *slog.Logger,
	repo Repository,
	interval time.Duration,
) UseCase {
	return &service{
		logger:   logger,
		repo:     repo,
		interval: interval,
	}
}

var _ UseCase = (*service)(nil)

// ReconcileCommand runs one iteration of the reconciliation loop.
func (s *service) ReconcileCommand(ctx context.Context) error {
	logger := s.logger.With("controller", "ReconcileCommand")

	pods, err := s.repo.ListPodsQuery(
		ctx,
		PreoomkillerPodLabelSelector,
	)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	logger.DebugContext(ctx, "starting to process pods", "count", len(pods))

	evictedCount := 0

	for i := range pods {
		select {
		case <-ctx.Done():
			logger.InfoContext(ctx, "context done, stopping reconciliation")

			return nil
		default:
		}

		evicted, err := s.processPod(ctx, logger, pods[i])
		if err != nil {
			logger.ErrorContext(ctx, "process pod error",
				"pod", pods[i].Name,
				"namespace", pods[i].Namespace,
				"reason", err,
			)

			continue
		}

		if evicted {
			evictedCount++
		}

		select {
		case <-ctx.Done():
			logger.InfoContext(ctx, "context done, stopping reconciliation")

			return nil
		// small delay to avoid overwhelming the API server
		case <-time.After(1 * time.Second):
		}
	}

	logger.InfoContext(ctx, "pods evicted", "count", len(pods), "evicted", evictedCount)

	return nil
}

func (s *service) processPod(
	ctx context.Context,
	logger *slog.Logger,
	pod Pod,
) (bool, error) {
	logger = logger.With("pod", pod.Name, "namespace", pod.Namespace, "controller", "processPod")

	memoryThresholdStr, ok := pod.Annotations[PreoomkillerAnnotationMemoryThresholdKey]
	if !ok {
		return false, fmt.Errorf(
			"%w: annotation %s not found",
			ErrMemoryThresholdParse,
			PreoomkillerAnnotationMemoryThresholdKey,
		)
	}

	podMemoryThreshold, err := resource.ParseQuantity(memoryThresholdStr)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrMemoryThresholdParse, err)
	}

	logger = logger.With("memoryThreshold", podMemoryThreshold.String())

	logger.DebugContext(ctx, "processing pod")

	podMetrics, err := s.repo.GetPodMetricsQuery(ctx, pod.Namespace, pod.Name)
	if err != nil {
		var target notFound
		if errors.As(err, &target) {
			logger.WarnContext(ctx, "pod metrics not found, skipping")

			return false, nil
		}

		return false, fmt.Errorf("%w: %w", ErrGetPodMetrics, err)
	}

	if podMetrics.MemoryUsage == nil {
		logger.WarnContext(ctx, "pod memory usage is nil, skipping")

		return false, nil
	}

	logger.DebugContext(ctx, "pod memory usage", "memoryUsage", podMetrics.MemoryUsage.String())

	if podMetrics.MemoryUsage.Cmp(podMemoryThreshold) == 1 {
		ok, err := s.evictPodCommand(ctx, logger, pod.Name, pod.Namespace)
		if err != nil {
			return false, fmt.Errorf("%w: %w", ErrEvictPod, err)
		}

		if ok {
			logger.InfoContext(ctx, "pod evicted", "memoryUsage", podMetrics.MemoryUsage.String())

			return true, nil
		}
	}

	return false, nil
}

// RunCommand runs the controller in a loop with the configured interval.
func (s *service) RunCommand(ctx context.Context) error {
	logger := s.logger.With("controller", "RunCommand")

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		err := s.ReconcileCommand(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "reconcile error", "reason", err)
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			logger.InfoContext(ctx, "terminating main controller loop")

			return nil
		}
	}
}

func (s *service) evictPodCommand(
	ctx context.Context,
	logger *slog.Logger,
	podName,
	podNamespace string,
) (bool, error) {
	err := s.repo.EvictPodCommand(ctx, podNamespace, podName)
	if err != nil {
		var target notFound
		if errors.As(err, &target) {
			logger.DebugContext(ctx, "pod not found when evicting")

			return false, nil
		}

		var tooManyRequestsTarget tooManyRequests
		if errors.As(err, &tooManyRequestsTarget) {
			logger.DebugContext(ctx, "too many requests when evicting, will retry later")

			return false, nil
		}

		return false, fmt.Errorf("%w: %w", ErrEvictPod, err)
	}

	return true, nil
}
