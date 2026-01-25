package controller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

type Service struct {
	logger               *slog.Logger
	repo                 Repository
	interval             time.Duration
	ready                chan struct{}
	doneCh               chan struct{}
	inShutdown           atomic.Bool
	mu                   sync.RWMutex
	lastReconcileEndTime time.Time
}

// New creates a new controller service.
func New(
	logger *slog.Logger,
	repo Repository,
	interval time.Duration,
) *Service {
	return &Service{
		logger:   logger,
		repo:     repo,
		interval: interval,
		ready:    make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (s *Service) Start(ctx context.Context) error {
	if s.inShutdown.Load() {
		s.logger.InfoContext(ctx, "controller service is shutting down, skipping start")

		return nil
	}

	go s.RunCommand(ctx)

	return nil
}

// Name returns the name of the server component
func (s *Service) Name() string {
	return "preoomkiller-controller"
}

func (s *Service) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ready:
		lastReconsileAge := s.getLastReconcileAge()
		if lastReconsileAge > 2*s.interval {
			return fmt.Errorf("last reconcile was too long ago: %s", lastReconsileAge.Round(time.Second).String())
		}

		return nil
	default:
		return fmt.Errorf("controller service is not ready")
	}
}

func (s *Service) Shutdown(ctx context.Context) error {
	if !s.inShutdown.CompareAndSwap(false, true) {
		s.logger.ErrorContext(ctx, "controller service is already shutting down, skipping shutdown")

		return nil // Already shutting down
	}

	defer func() {
		s.logger.InfoContext(ctx, "controller service shut downed")
	}()

	s.logger.InfoContext(ctx, "shutting down controller service")

	// Wait for RunCommand to exit, respecting shutdown context
	// RunCommand will exit when ctx.Done() is triggered (context cancellation)
	select {
	case <-ctx.Done():
		return fmt.Errorf("shutdown context done before controller loop exited: %w", ctx.Err())
	case <-s.doneCh:
		s.logger.InfoContext(ctx, "controller loop exited")
	}

	return nil
}

// ReconcileCommand runs one iteration of the reconciliation loop.
func (s *Service) ReconcileCommand(ctx context.Context) error {
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

func (s *Service) processPod(
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

func (s *Service) Ready() <-chan struct{} {
	return s.ready
}

// RunCommand runs the controller in a loop with the configured interval.
func (s *Service) RunCommand(ctx context.Context) {
	defer close(s.doneCh)

	logger := s.logger.With("controller", "RunCommand")

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	close(s.ready)

	for {
		err := s.ReconcileCommand(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "reconcile error", "reason", err)
		}

		s.setLastReconcileEndTime()

		select {
		case <-ticker.C:
		case <-ctx.Done():
			logger.InfoContext(ctx, "terminating main controller loop")

			return
		}
	}
}

func (s *Service) evictPodCommand(
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

func (s *Service) getLastReconcileAge() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return time.Since(s.lastReconcileEndTime)
}

func (s *Service) setLastReconcileEndTime() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastReconcileEndTime = time.Now()
}
