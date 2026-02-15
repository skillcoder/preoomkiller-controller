package controller_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/cronparser"
	"github.com/skillcoder/preoomkiller-controller/internal/logic/controller"
	"github.com/skillcoder/preoomkiller-controller/internal/logic/controller/mocks"
)

// testNotFoundError and testTooManyRequestsError implement the controller's private error interfaces
// so the mock can return them and the controller recognizes them.
type testNotFoundError struct{}

func (testNotFoundError) Error() string { return "not found" }
func (testNotFoundError) IsNotFound()   {}

type testTooManyRequestsError struct{}

func (testTooManyRequestsError) Error() string      { return "too many requests" }
func (testTooManyRequestsError) IsTooManyRequests() {}

func ptrQty(q resource.Quantity) *resource.Quantity {
	return &q
}

func testQty(s string) resource.Quantity {
	return resource.MustParse(s)
}

func TestService_ReconcileCommand(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	t.Run("empty list succeeds", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepository(t)
		svc := controller.New(
			logger,
			repo,
			cronparser.New(),
			1*time.Second,
			"label",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
			controller.PreoomkillerAnnotationRestartScheduleKey,
			controller.PreoomkillerAnnotationTZKey,
			controller.PreoomkillerAnnotationRestartAtKey,
			30*time.Second,
			0,
		)

		repo.EXPECT().
			ListPodsQuery(mock.Anything, "label").
			Return([]controller.Pod{}, nil).
			Once()

		err := svc.ReconcileCommand(t.Context())
		require.NoError(t, err)
	})

	t.Run("list error returns error", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepository(t)
		svc := controller.New(
			logger,
			repo,
			cronparser.New(),
			1*time.Second,
			"label",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
			controller.PreoomkillerAnnotationRestartScheduleKey,
			controller.PreoomkillerAnnotationTZKey,
			controller.PreoomkillerAnnotationRestartAtKey,
			30*time.Second,
			0,
		)

		repo.EXPECT().
			ListPodsQuery(mock.Anything, "label").
			Return(nil, context.DeadlineExceeded).
			Once()

		err := svc.ReconcileCommand(t.Context())
		require.Error(t, err)
	})

	t.Run("one pod over threshold evicts", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepository(t)
		svc := controller.New(
			logger,
			repo,
			cronparser.New(),
			1*time.Second,
			"label",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
			controller.PreoomkillerAnnotationRestartScheduleKey,
			controller.PreoomkillerAnnotationTZKey,
			controller.PreoomkillerAnnotationRestartAtKey,
			30*time.Second,
			0,
		)

		pod := controller.Pod{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				controller.PreoomkillerAnnotationMemoryThresholdKey: "256Mi",
			},
			MemoryLimit: ptrQty(testQty("1Gi")),
		}

		repo.EXPECT().
			ListPodsQuery(mock.Anything, "label").
			Return([]controller.Pod{pod}, nil).
			Once()
		repo.EXPECT().
			GetPodMetricsQuery(mock.Anything, "default", "test-pod").
			Return(&controller.PodMetrics{MemoryUsage: ptrQty(testQty("512Mi"))}, nil).
			Once()
		repo.EXPECT().
			EvictPodCommand(mock.Anything, "default", "test-pod").
			Return(nil).
			Once()

		err := svc.ReconcileCommand(t.Context())
		require.NoError(t, err)
	})

	t.Run("evict too many requests skips", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepository(t)
		svc := controller.New(
			logger,
			repo,
			cronparser.New(),
			1*time.Second,
			"label",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
			controller.PreoomkillerAnnotationRestartScheduleKey,
			controller.PreoomkillerAnnotationTZKey,
			controller.PreoomkillerAnnotationRestartAtKey,
			30*time.Second,
			0,
		)

		pod := controller.Pod{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				controller.PreoomkillerAnnotationMemoryThresholdKey: "256Mi",
			},
			MemoryLimit: ptrQty(testQty("1Gi")),
		}

		repo.EXPECT().
			ListPodsQuery(mock.Anything, "label").
			Return([]controller.Pod{pod}, nil).
			Once()
		repo.EXPECT().
			GetPodMetricsQuery(mock.Anything, "default", "test-pod").
			Return(&controller.PodMetrics{MemoryUsage: ptrQty(testQty("512Mi"))}, nil).
			Once()
		repo.EXPECT().
			EvictPodCommand(mock.Anything, "default", "test-pod").
			Return(testTooManyRequestsError{}).
			Once()

		err := svc.ReconcileCommand(t.Context())
		require.NoError(t, err)
	})

	t.Run("pod over threshold but too young skips eviction", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepository(t)
		minPodAge := 30 * time.Minute
		svc := controller.New(
			logger,
			repo,
			cronparser.New(),
			1*time.Second,
			"label",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
			controller.PreoomkillerAnnotationRestartScheduleKey,
			controller.PreoomkillerAnnotationTZKey,
			controller.PreoomkillerAnnotationRestartAtKey,
			30*time.Second,
			minPodAge,
		)

		pod := controller.Pod{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				controller.PreoomkillerAnnotationMemoryThresholdKey: "256Mi",
			},
			MemoryLimit: ptrQty(testQty("1Gi")),
			CreatedAt:   time.Now().Add(-10 * time.Minute),
		}

		repo.EXPECT().
			ListPodsQuery(mock.Anything, "label").
			Return([]controller.Pod{pod}, nil).
			Once()
		repo.EXPECT().
			GetPodMetricsQuery(mock.Anything, "default", "test-pod").
			Return(&controller.PodMetrics{MemoryUsage: ptrQty(testQty("512Mi"))}, nil).
			Once()
		// EvictPodCommand must not be called (eviction skipped due to pod too young)
		repo.EXPECT().
			EvictPodCommand(mock.Anything, mock.Anything, mock.Anything).
			Maybe()

		err := svc.ReconcileCommand(t.Context())
		require.NoError(t, err)
	})

	t.Run("missed scheduled eviction but pod too young skips eviction", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepository(t)
		minPodAge := 30 * time.Minute
		svc := controller.New(
			logger,
			repo,
			cronparser.New(),
			1*time.Second,
			"label",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
			controller.PreoomkillerAnnotationRestartScheduleKey,
			controller.PreoomkillerAnnotationTZKey,
			controller.PreoomkillerAnnotationRestartAtKey,
			30*time.Second,
			minPodAge,
		)

		now := time.Now()
		restartAt := now.Add(-5 * time.Minute)
		pod := controller.Pod{
			Name:      "scheduled-pod",
			Namespace: "default",
			Annotations: map[string]string{
				controller.PreoomkillerAnnotationRestartScheduleKey: "0 * * * *",
				controller.PreoomkillerAnnotationRestartAtKey:       restartAt.Format(time.RFC3339),
			},
			CreatedAt: now.Add(-10 * time.Minute),
		}

		repo.EXPECT().
			ListPodsQuery(mock.Anything, "label").
			Return([]controller.Pod{pod}, nil).
			Once()
		// EvictPodCommand must not be called (pod too young)
		repo.EXPECT().
			EvictPodCommand(mock.Anything, mock.Anything, mock.Anything).
			Maybe()

		err := svc.ReconcileCommand(t.Context())
		require.NoError(t, err)
	})

	t.Run("metrics not found skips pod", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepository(t)
		svc := controller.New(
			logger,
			repo,
			cronparser.New(),
			1*time.Second,
			"label",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
			controller.PreoomkillerAnnotationRestartScheduleKey,
			controller.PreoomkillerAnnotationTZKey,
			controller.PreoomkillerAnnotationRestartAtKey,
			30*time.Second,
			0,
		)

		pod := controller.Pod{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				controller.PreoomkillerAnnotationMemoryThresholdKey: "256Mi",
			},
			MemoryLimit: ptrQty(testQty("1Gi")),
		}

		repo.EXPECT().
			ListPodsQuery(mock.Anything, "label").
			Return([]controller.Pod{pod}, nil).
			Once()
		repo.EXPECT().
			GetPodMetricsQuery(mock.Anything, "default", "test-pod").
			Return(nil, testNotFoundError{}).
			Once()

		err := svc.ReconcileCommand(t.Context())
		require.NoError(t, err)
	})
}

func TestService_Start_Ready_Shutdown(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	repo := mocks.NewMockRepository(t)
	svc := controller.New(
		logger,
		repo,
		cronparser.New(),
		10*time.Second,
		"",
		controller.PreoomkillerAnnotationMemoryThresholdKey,
		controller.PreoomkillerAnnotationRestartScheduleKey,
		controller.PreoomkillerAnnotationTZKey,
		controller.PreoomkillerAnnotationRestartAtKey,
		30*time.Second,
		0,
	)

	repo.EXPECT().
		ListPodsQuery(mock.Anything, mock.Anything).
		Return([]controller.Pod{}, nil).
		Maybe()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	require.NoError(t, svc.Start(ctx))

	select {
	case <-svc.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("service did not become ready")
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	require.NoError(t, svc.Shutdown(shutdownCtx))
}

func TestService_Ping(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	t.Run("before ready returns error", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepository(t)
		svc := controller.New(
			logger,
			repo,
			cronparser.New(),
			10*time.Second,
			"",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
			controller.PreoomkillerAnnotationRestartScheduleKey,
			controller.PreoomkillerAnnotationTZKey,
			controller.PreoomkillerAnnotationRestartAtKey,
			30*time.Second,
			0,
		)

		err := svc.Ping(t.Context())
		require.Error(t, err)
	})

	t.Run("after ready returns nil", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepository(t)
		svc := controller.New(
			logger,
			repo,
			cronparser.New(),
			10*time.Second,
			"",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
			controller.PreoomkillerAnnotationRestartScheduleKey,
			controller.PreoomkillerAnnotationTZKey,
			controller.PreoomkillerAnnotationRestartAtKey,
			30*time.Second,
			0,
		)

		repo.EXPECT().
			ListPodsQuery(mock.Anything, mock.Anything).
			Return([]controller.Pod{}, nil).
			Maybe()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		require.NoError(t, svc.Start(ctx))

		select {
		case <-svc.Ready():
		case <-time.After(2 * time.Second):
			t.Fatal("service did not become ready")
		}

		require.NoError(t, svc.Ping(t.Context()))
		cancel()
	})
}
