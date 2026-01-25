package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/klog/v2"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/skillcoder/preoomkiller-controller/internal/config"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/logging"
)

const (
	EvictionKind                             = "Eviction"
	EvictionAPIVersion                       = "policy/v1"
	PreoomkillerPodLabelSelector             = "preoomkiller-enabled=true"
	PreoomkillerAnnotationMemoryThresholdKey = "preoomkiller.beta.k8s.skillcoder.com/memory-threshold"
)

// Controller is responsible for ensuring that pods matching PreoomkillerPodLabelSelector
// are evicted.
type Controller struct {
	clientset        kubernetes.Interface
	metricsClientset *metricsv.Clientset
	interval         time.Duration
}

func NewController(clientset kubernetes.Interface, metricsClientset *metricsv.Clientset, interval time.Duration) *Controller {
	return &Controller{
		clientset:        clientset,
		metricsClientset: metricsClientset,
		interval:         interval,
	}
}

// evictPod attempts to evict a pod in a given namespace
func evictPod(ctx context.Context, client kubernetes.Interface, podName, podNamespace string, dryRun bool) (bool, error) {
	if dryRun {
		return true, nil
	}

	eviction := &policy.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: EvictionAPIVersion,
			Kind:       EvictionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
		},
	}

	err := client.PolicyV1().Evictions(eviction.Namespace).Evict(ctx, eviction)
	if err != nil {
		switch {
		case apierrors.IsTooManyRequests(err):
			// ignore, we will retry later on next iteration.
			return false, fmt.Errorf("error when evicting pod (ignoring): %w", err)
		case apierrors.IsNotFound(err):
			return true, fmt.Errorf("pod not found when evicting: %w", err)
		}

		return false, fmt.Errorf("evict pod: %w", err)
	}

	return true, nil
}

// RunOnce runs one sigle iteration of reconciliation loop
func (c *Controller) RunOnce(ctx context.Context) error {
	logger := slog.With("controller", "RunOnce")
	evictedCount := 0

	podList, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: PreoomkillerPodLabelSelector,
	})
	if err != nil {
		return fmt.Errorf("pod list error: %w", err)
	}

	logger.DebugContext(ctx, "starting to process pods", "count", len(podList.Items))

	for i := range podList.Items {
		ok, err := c.processPod(ctx, logger, &podList.Items[i])
		if err != nil {
			logger.ErrorContext(ctx, "process pod error",
				"pod", podList.Items[i].Name,
				"namespace", podList.Items[i].Namespace,
				"reason", err,
			)

			continue
		}

		if ok {
			evictedCount++
		}
	}

	logger.InfoContext(ctx, "pods evicted", "count", len(podList.Items), "evicted", evictedCount)

	return nil
}

func (c *Controller) processPod(ctx context.Context, logger *slog.Logger, pod *corev1.Pod) (bool, error) {
	podName, podNamespace := pod.Name, pod.Namespace

	logger = logger.With("pod", podName, "namespace", podNamespace, "controller", "processPod")

	logger.DebugContext(ctx, "processing pod")

	podMemoryThreshold, err := resource.ParseQuantity(pod.Annotations[PreoomkillerAnnotationMemoryThresholdKey])
	if err != nil {
		return false, fmt.Errorf("parse memory threshold: %w", err)
	}

	logger = logger.With("memoryThreshold", podMemoryThreshold.String())

	podMemoryUsage := &resource.Quantity{}

	podMetrics, err := c.metricsClientset.MetricsV1beta1().PodMetricses(podNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get pod metrics: %w", err)
	}

	for _, containerMetrics := range podMetrics.Containers {
		containerMemoryUsage := containerMetrics.Usage.Memory()
		if containerMemoryUsage == nil {
			logger.WarnContext(ctx, "container memory usage is nil",
				"container", containerMetrics.Name,
			)

			continue
		}

		podMemoryUsage.Add(*containerMemoryUsage)
		logger.DebugContext(ctx, "container metrics",
			"container", containerMetrics.Name,
			"cpu", containerMetrics.Usage.Cpu().String(),
			"memory", containerMemoryUsage.String(),
		)
	}

	logger.DebugContext(ctx, "pod memory usage", "memoryUsage", podMemoryUsage.String())

	if podMemoryUsage.Cmp(podMemoryThreshold) == 1 {
		_, err := evictPod(ctx, c.clientset, podName, podNamespace, false)
		if err != nil {
			return false, fmt.Errorf("evict pod: %w", err)
		}

		logger.InfoContext(ctx, "pod evicted", "memoryUsage", podMemoryUsage.String())

		return true, nil
	}

	return false, nil
}

// Run runs RunOnce in a loop with a delay until stopCh receives a value.
func (c *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		err := c.RunOnce(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "run once error", "reason", err)
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			slog.InfoContext(ctx, "terminating main controller loop")

			return
		}
	}
}

func main() {
	ctx := context.Background()

	err := run(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to run", "reason", err)
		// Give the logger some time to flush
		time.Sleep(1 * time.Second)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "bye")
}

func run(originCtx context.Context) error {
	// Start listening for signals immediately
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancelAppCtx := context.WithCancel(originCtx)
	defer cancelAppCtx()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.New(cfg.LogFormat, cfg.LogLevel)

	// creates the connection
	kubeConfig, err := clientcmd.BuildConfigFromFlags(cfg.KubeMaster, cfg.KubeConfig)
	if err != nil {
		return fmt.Errorf("build config: %w", err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("create clientset: %w", err)
	}

	metricsClientset, err := metricsv.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("create metrics clientset: %w", err)
	}

	controller := NewController(clientset, metricsClientset, cfg.Interval)

	// check if we need exit without running the controller.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Handle signals in a goroutine
	go handleSignals(ctx, signals, cancelAppCtx)

	logger.InfoContext(ctx, "starting controller")

	controller.Run(ctx)

	return nil
}

func handleSignals(ctx context.Context, signals chan os.Signal, cancelAppCtx func()) {
	select {
	case <-ctx.Done():
		slog.InfoContext(ctx, "terminating signal handler due to context done")

		return
	case <-signals:
	}

	slog.InfoContext(ctx, "received termination signal, terminating")
	cancelAppCtx()
}
