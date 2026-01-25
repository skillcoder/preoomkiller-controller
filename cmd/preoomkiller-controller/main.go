package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	policy "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/klog/v2"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	EvictionKind                             = "Eviction"
	PreoomkillerPodLabelSelector             = "preoomkiller-enabled=true"
	PreoomkillerAnnotationMemoryThresholdKey = "preoomkiller.alpha.k8s.zapier.com/memory-threshold"
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
			APIVersion: "policy/v1",
			Kind:       EvictionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
		},
	}
	err := client.PolicyV1().Evictions(eviction.Namespace).Evict(ctx, eviction)

	if err == nil {
		return true, nil
	} else if apierrors.IsTooManyRequests(err) {
		return false, fmt.Errorf("error when evicting pod (ignoring) %q: %v", podName, err)
	} else if apierrors.IsNotFound(err) {
		return true, fmt.Errorf("pod not found when evicting %q: %v", podName, err)
	} else {
		return false, err
	}
}

// RunOnce runs one sigle iteration of reconciliation loop
func (c *Controller) RunOnce(ctx context.Context) error {
	evictionCount := 0

	podList, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: PreoomkillerPodLabelSelector,
	})
	if err != nil {
		slog.ErrorContext(ctx, "pod list error",
			"labelSelector", PreoomkillerPodLabelSelector,
			"reason", err)
		return err
	}

	for _, pod := range podList.Items {
		podName, podNamespace := pod.ObjectMeta.Name, pod.ObjectMeta.Namespace
		podMemoryThreshold, err := resource.ParseQuantity(pod.ObjectMeta.Annotations[PreoomkillerAnnotationMemoryThresholdKey])
		if err != nil {
			slog.ErrorContext(ctx, "pod memory threshold fetch error",
				"pod", podName,
				"namespace", podNamespace,
				"reason", err)
			continue
		}

		podMemoryUsage := &resource.Quantity{}

		podMetrics, err := c.metricsClientset.MetricsV1beta1().PodMetricses(podNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			slog.ErrorContext(ctx, "pod metrics fetch error",
				"pod", podName,
				"namespace", podNamespace,
				"reason", err)
			return err
		}

		for _, containerMetrics := range podMetrics.Containers {
			podMemoryUsage.Add(*containerMetrics.Usage.Memory())
			slog.DebugContext(ctx, "container metrics",
				"pod", podName,
				"namespace", podNamespace,
				"container", containerMetrics.Name,
				"cpu", containerMetrics.Usage.Cpu().String(),
				"memory", containerMetrics.Usage.Memory().String())
		}
		slog.DebugContext(ctx, "pod memory usage",
			"pod", podName,
			"namespace", podNamespace,
			"memoryUsage", podMemoryUsage.String())
		if podMemoryUsage.Cmp(podMemoryThreshold) == 1 {
			_, err := evictPod(ctx, c.clientset, podName, podNamespace, false)
			if err != nil {
				slog.ErrorContext(ctx, "pod eviction error",
					"pod", podName,
					"namespace", podNamespace,
					"reason", err)
			} else {
				evictionCount += 1
				slog.InfoContext(ctx, "pod evicted",
					"pod", podName,
					"namespace", podNamespace,
					"memoryUsage", podMemoryUsage.String())
			}
		}
	}
	slog.InfoContext(ctx, "pods evicted during this run",
		"evictionCount", evictionCount)
	return nil
}

// Run runs RunOnce in a loop with a delay until stopCh receives a value.
func (c *Controller) Run(ctx context.Context, stopCh chan struct{}) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		err := c.RunOnce(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "run once error", "reason", err)
		}
		select {
		case <-ticker.C:
		case <-stopCh:
			slog.InfoContext(ctx, "terminating main controller loop")
			return
		}
	}
}

func main() {
	var kubeconfig string
	var master string
	var loglevel string
	var logformat string
	var interval int

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.IntVar(&interval, "interval", 60, "Interval (in seconds)")
	flag.StringVar(&loglevel, "loglevel", "info", "Log level, one of debug, info, warn, error")
	flag.StringVar(&logformat, "logformat", "json", "Log format, one of json, text")
	flag.Set("logtostderr", "true")
	flag.Parse()

	// Setup logging
	var level slog.Level
	switch loglevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "info":
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	switch logformat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	case "text":
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	default:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()

	// creates the connection
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build config", "reason", err)
		os.Exit(1)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create clientset", "reason", err)
		os.Exit(1)
	}

	//
	metricsClientset, err := metricsv.NewForConfig(config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create metrics clientset", "reason", err)
		os.Exit(1)
	}

	controller := NewController(clientset, metricsClientset, time.Duration(interval)*time.Second)

	// Now let's start the controller
	stopCh := make(chan struct{})
	go handleSigterm(stopCh)
	defer close(stopCh)
	controller.Run(ctx, stopCh)
}

func handleSigterm(stopCh chan struct{}) {
	ctx := context.Background()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	<-signals
	slog.InfoContext(ctx, "received sigterm, terminating")
	close(stopCh)
}
