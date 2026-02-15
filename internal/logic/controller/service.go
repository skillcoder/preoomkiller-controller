package controller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

type Service struct {
	logger                       *slog.Logger
	repo                         Repository
	scheduleParser               scheduleParser
	interval                     time.Duration
	labelSelector                string
	annotationMemoryThresholdKey string
	annotationRestartScheduleKey string
	annotationTZKey              string
	annotationRestartAtKey       string
	jitterMax                    time.Duration
	ready                        chan struct{}
	doneCh                       chan struct{}
	inShutdown                   atomic.Bool
	mu                           sync.RWMutex
	lastReconcileEndTime         time.Time
	timerMu                      sync.Mutex
	pendingTimers                map[string]*time.Timer
	inFlightWg                   sync.WaitGroup
}

// New creates a new controller service.
func New(
	logger *slog.Logger,
	repo Repository,
	parser scheduleParser,
	interval time.Duration,
	labelSelector string,
	annotationMemoryThresholdKey string,
	annotationRestartScheduleKey string,
	annotationTZKey string,
	annotationRestartAtKey string,
	jitterMax time.Duration,
) *Service {
	return &Service{
		logger:                       logger,
		repo:                         repo,
		scheduleParser:               parser,
		interval:                     interval,
		labelSelector:                labelSelector,
		annotationMemoryThresholdKey: annotationMemoryThresholdKey,
		annotationRestartScheduleKey: annotationRestartScheduleKey,
		annotationTZKey:              annotationTZKey,
		annotationRestartAtKey:       annotationRestartAtKey,
		jitterMax:                    jitterMax,
		ready:                        make(chan struct{}),
		doneCh:                       make(chan struct{}),
		pendingTimers:                make(map[string]*time.Timer, _defaultPendingTimersCapacity),
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

		return nil
	}

	defer func() {
		s.logger.InfoContext(ctx, "controller service shut downed")
	}()

	s.logger.InfoContext(ctx, "shutting down controller service")

	s.stopPendingTimers()

	select {
	case <-ctx.Done():
		return fmt.Errorf("shutdown context done before controller loop exited: %w", ctx.Err())
	case <-s.doneCh:
		s.logger.InfoContext(ctx, "controller loop exited")
	}

	done := make(chan struct{})

	go func() {
		s.inFlightWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.InfoContext(ctx, "all scheduled evictions finished")
	case <-ctx.Done():
		return fmt.Errorf("shutdown wait for evictions: %w", ctx.Err())
	}

	return nil
}

func (s *Service) stopPendingTimers() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()

	for key, timer := range s.pendingTimers {
		if timer.Stop() {
			s.inFlightWg.Done()
		}

		delete(s.pendingTimers, key)
	}
}

func (s *Service) processScheduledRestart(
	ctx context.Context,
	logger *slog.Logger,
	pod Pod,
) {
	logger = logger.With("pod", pod.Name, "namespace", pod.Namespace)

	restartAtStr, hasRestartAt := pod.Annotations[s.annotationRestartAtKey]
	if hasRestartAt {
		if s.handleExistingRestartAt(ctx, logger, pod, restartAtStr) {
			return
		}
	}

	spec := pod.Annotations[s.annotationRestartScheduleKey]
	tz := pod.Annotations[s.annotationTZKey]

	nextRun, err := s.scheduleParser.NextAfter(spec, tz, time.Now())
	if err != nil {
		logger.WarnContext(ctx, "invalid restart schedule",
			"spec", spec,
			"tz", tz,
			"reason", err,
		)

		return
	}

	restartAtValue := nextRun.Format(time.RFC3339)

	if err := s.repo.SetAnnotationCommand(
		ctx,
		pod.Namespace,
		pod.Name,
		s.annotationRestartAtKey,
		restartAtValue,
	); err != nil {
		logger.ErrorContext(ctx, "set restart-at annotation",
			"reason", err,
		)

		return
	}

	s.scheduleEviction(ctx, logger, pod.Namespace, pod.Name, nextRun)
}

func (s *Service) handleExistingRestartAt(
	ctx context.Context,
	logger *slog.Logger,
	pod Pod,
	restartAtStr string,
) bool {
	restartAt, err := time.Parse(time.RFC3339, restartAtStr)
	if err != nil {
		logger.WarnContext(ctx, "invalid restart-at annotation",
			"restartAt", restartAtStr,
			"reason", err,
		)

		return false
	}

	now := time.Now()

	if restartAt.After(now) {
		logger.DebugContext(ctx, "recovering scheduled eviction",
			"restartAt", restartAtStr,
		)
		s.scheduleEviction(ctx, logger, pod.Namespace, pod.Name, restartAt)

		return true
	}

	if pod.CreatedAt.Before(restartAt) {
		logger.InfoContext(ctx, "missed scheduled eviction, evicting now",
			"restartAt", restartAtStr,
			"podCreatedAt", pod.CreatedAt.Format(time.RFC3339),
		)

		ok, evictErr := s.evictPodCommand(ctx, logger, pod.Name, pod.Namespace)
		if evictErr != nil {
			logger.ErrorContext(ctx, "missed eviction failed",
				"reason", evictErr,
			)
		}

		if ok {
			logger.InfoContext(ctx, "pod evicted (missed schedule)")
		}

		return true
	}

	logger.WarnContext(ctx, "stale restart-at annotation, rescheduling",
		"restartAt", restartAtStr,
		"podCreatedAt", pod.CreatedAt.Format(time.RFC3339),
	)

	return false
}

const (
	_defaultPendingTimersCapacity = 16
	_evictionTimeout              = 90 * time.Second
)

func (s *Service) scheduleEviction(
	ctx context.Context,
	logger *slog.Logger,
	namespace,
	name string,
	at time.Time,
) {
	if s.inShutdown.Load() {
		return
	}

	key := namespace + "/" + name

	s.timerMu.Lock()
	defer s.timerMu.Unlock()

	if _, exists := s.pendingTimers[key]; exists {
		return
	}

	delay := max(time.Until(at), 0)

	// Jitter does not require cryptographic randomness.
	// #nosec G404
	jitter := time.Duration(rand.Int63n(int64(s.jitterMax + 1)))

	s.inFlightWg.Add(1)

	// Callback runs asynchronously; passing ctx would be incorrect (it may be cancelled by then).
	//nolint:contextcheck // runScheduledEviction uses context.Background() for the eviction call.
	timer := time.AfterFunc(delay+jitter, func() {
		s.runScheduledEviction(logger, key, namespace, name)
	})

	s.pendingTimers[key] = timer

	logger.DebugContext(ctx, "scheduled eviction goroutine",
		"pod", name,
		"namespace", namespace,
		"at", at.Format(time.RFC3339),
		"delay", delay+jitter,
	)
}

func (s *Service) runScheduledEviction(
	logger *slog.Logger,
	key,
	namespace,
	name string,
) {
	defer s.inFlightWg.Done()

	if s.inShutdown.Load() {
		s.timerMu.Lock()
		delete(s.pendingTimers, key)
		s.timerMu.Unlock()

		return
	}

	evictCtx, cancel := context.WithTimeout(context.Background(), _evictionTimeout)
	defer cancel()

	logger.InfoContext(evictCtx, "executing scheduled eviction",
		"pod", name,
		"namespace", namespace,
	)

	ok, err := s.evictPodCommand(evictCtx, logger, name, namespace)
	if err != nil {
		logger.ErrorContext(evictCtx, "scheduled eviction failed",
			"pod", name,
			"namespace", namespace,
			"reason", err,
		)
	}

	if ok {
		logger.InfoContext(evictCtx, "pod evicted by schedule",
			"pod", name,
			"namespace", namespace,
		)
	}

	s.timerMu.Lock()
	delete(s.pendingTimers, key)
	s.timerMu.Unlock()
}

// ReconcileCommand runs one iteration of the reconciliation loop.
func (s *Service) ReconcileCommand(ctx context.Context) error {
	logger := s.logger.With("controller", "ReconcileCommand")

	pods, err := s.repo.ListPodsQuery(ctx, s.labelSelector)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	logger.DebugContext(ctx, "starting to process pods", "count", len(pods))

	evictedCount := 0

	for i := range pods {
		if done := s.reconcileOnePod(ctx, logger, pods[i], &evictedCount); done {
			return nil
		}

		select {
		case <-ctx.Done():
			logger.InfoContext(ctx, "context done, stopping reconciliation")

			return nil
		case <-time.After(1 * time.Second):
		}
	}

	logger.InfoContext(ctx, "pods evicted", "count", len(pods), "evicted", evictedCount)

	return nil
}

// reconcileOnePod processes one pod (schedule-based and memory-threshold). Returns true if context is done.
func (s *Service) reconcileOnePod(
	ctx context.Context,
	logger *slog.Logger,
	pod Pod,
	evictedCount *int,
) bool {
	select {
	case <-ctx.Done():
		logger.InfoContext(ctx, "context done, stopping reconciliation")

		return true
	default:
	}

	if _, hasSchedule := pod.Annotations[s.annotationRestartScheduleKey]; hasSchedule {
		s.processScheduledRestart(ctx, logger, pod)
	}

	if _, hasThreshold := pod.Annotations[s.annotationMemoryThresholdKey]; hasThreshold {
		evicted, err := s.processPod(ctx, logger, pod)
		if err != nil {
			logger.ErrorContext(ctx, "process pod error",
				"pod", pod.Name,
				"namespace", pod.Namespace,
				"reason", err,
			)

			return false
		}

		if evicted {
			*evictedCount++
		}
	}

	return false
}

// resolveMemoryThreshold returns the effective memory threshold from the pod annotation.
// The annotation may be an absolute quantity (e.g. "512Mi") or a percentage of the pod's memory limit (e.g. "80%").
// Returns ErrMemoryLimitNotDefined when the annotation is a percentage but the pod has no memory limit (caller should skip eviction).
func resolveMemoryThreshold(
	ctx context.Context,
	logger *slog.Logger,
	pod Pod,
	annotationKey string,
) (resource.Quantity, error) {
	memoryThresholdStr, ok := pod.Annotations[annotationKey]
	if !ok {
		return resource.Quantity{}, fmt.Errorf(
			"%w: annotation %s not found",
			ErrMemoryThresholdParse,
			annotationKey,
		)
	}

	logger = logger.With(
		"annotationKey", annotationKey,
		"annotationValue", memoryThresholdStr,
	)

	if before, ok0 := strings.CutSuffix(memoryThresholdStr, "%"); ok0 {
		return resolveMemoryThresholdFromPercent(ctx, logger, strings.TrimSpace(before), pod.MemoryLimit)
	}

	// Absolute quantity
	threshold, err := resource.ParseQuantity(memoryThresholdStr)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("%w: %w", ErrMemoryThresholdParse, err)
	}

	logger.DebugContext(ctx, "resolved absolute threshold",
		"memoryThreshold", threshold.String(),
	)

	return threshold, nil
}

// resolveMemoryThresholdFromPercent interprets percentStr as a percentage of the memory limit
// and returns the corresponding absolute threshold.
func resolveMemoryThresholdFromPercent(
	ctx context.Context,
	logger *slog.Logger,
	percentStr string,
	memoryLimit *resource.Quantity,
) (resource.Quantity, error) {
	percent, err := strconv.ParseFloat(percentStr, 64)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("%w: invalid percentage %q: %w", ErrMemoryThresholdParse, percentStr, err)
	}

	if percent <= 0 || percent > 100 {
		return resource.Quantity{}, fmt.Errorf("%w: percentage must be in (0, 100], got %q",
			ErrMemoryThresholdParse, percentStr,
		)
	}

	if memoryLimit == nil || memoryLimit.IsZero() {
		logger.WarnContext(ctx, "memory threshold is percentage but pod has no memory limit, skipping eviction",
			"memoryLimitSet", false,
		)

		return resource.Quantity{}, ErrMemoryLimitNotDefined
	}

	limitFloat := memoryLimit.AsApproximateFloat64()
	thresholdFloat := limitFloat * (percent / percentScale)
	threshold := resource.NewQuantity(int64(thresholdFloat), resource.BinarySI)

	logger.DebugContext(ctx, "resolved percentage threshold",
		"memoryLimit", memoryLimit.String(),
		"memoryThreshold", threshold.String(),
	)

	return *threshold, nil
}

// getPodMemoryUsageOrSkip fetches pod metrics; skip is true when the pod should be skipped (e.g. not found, no metrics).
func (s *Service) getPodMemoryUsageOrSkip(ctx context.Context, logger *slog.Logger, pod Pod) (resource.Quantity, bool, error) {
	podMetrics, err := s.repo.GetPodMetricsQuery(ctx, pod.Namespace, pod.Name)
	if err != nil {
		var target notFound
		if errors.As(err, &target) {
			logger.WarnContext(ctx, "pod metrics not found, skipping")

			return resource.Quantity{}, true, nil
		}

		return resource.Quantity{}, false, fmt.Errorf("%w: %w", ErrGetPodMetrics, err)
	}

	if podMetrics.MemoryUsage == nil {
		logger.WarnContext(ctx, "pod memory usage is nil, skipping")

		return resource.Quantity{}, true, nil
	}

	if podMetrics.MemoryUsage.IsZero() {
		logger.WarnContext(ctx, "pod memory usage is zero, skipping")

		return resource.Quantity{}, true, nil
	}

	return *podMetrics.MemoryUsage, false, nil
}

func (s *Service) processPod(
	ctx context.Context,
	logger *slog.Logger,
	pod Pod,
) (bool, error) {
	logger = logger.With("pod", pod.Name, "namespace", pod.Namespace, "controller", "processPod")

	podMemoryThreshold, err := resolveMemoryThreshold(ctx, logger, pod, s.annotationMemoryThresholdKey)
	if err != nil {
		if errors.Is(err, ErrMemoryLimitNotDefined) {
			return false, nil
		}

		return false, err
	}

	logger = logger.With("memoryThreshold", podMemoryThreshold.String())

	if pod.MemoryLimit != nil {
		logger = logger.With("memoryLimit", pod.MemoryLimit.String())
	}

	if podMemoryThreshold.IsZero() {
		logger.WarnContext(ctx, "memory threshold is zero, skipping")

		return false, nil
	}

	logger.DebugContext(ctx, "processing pod")

	podMemoryUsage, skip, err := s.getPodMemoryUsageOrSkip(ctx, logger, pod)
	if skip {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	logger.DebugContext(ctx, "pod memory usage", "memoryUsage", podMemoryUsage.String())

	if podMemoryUsage.Cmp(podMemoryThreshold) == 1 {
		ok, err := s.evictPodCommand(ctx, logger, pod.Name, pod.Namespace)
		if err != nil {
			return false, fmt.Errorf("%w: %w", ErrEvictPod, err)
		}

		if ok {
			logger.InfoContext(ctx, "pod evicted", "memoryUsage", podMemoryUsage.String())

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

	// NOTE: set immidiatly to speed up first ready signal for pinger.
	s.setLastReconcileEndTime()

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
