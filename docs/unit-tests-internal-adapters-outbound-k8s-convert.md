# Unit test plan: internal/adapters/outbound/k8s/convert.go

## Scope

- **File:** `internal/adapters/outbound/k8s/convert.go`
- **Package:** `k8s_test` (black-box) or `k8s` with `*_internal_test.go` (unexported `toDomainPod`, `toDomainPodMetrics`).

## Testable logic

| Target | Description |
|--------|-------------|
| `toDomainPod` | corev1.Pod → controller.Pod; sums container memory limits, sets Annotations. |
| `toDomainPodMetrics` | PodMetrics → controller.PodMetrics; sums container memory usage, skips nil. |

## Test cases (table-driven, give/want)

- **TestToDomainPod**: give corev1.Pod (with/without memory limits, multiple containers), want controller.Pod (Name, Namespace, Annotations, MemoryLimit nil or value).
- **TestToDomainPodMetrics**: give metricsv1beta1.PodMetrics (containers with/without nil usage), want controller.PodMetrics MemoryUsage. Requires logger and context; use slog.Default() and t.Context().

## Notes

- If functions are unexported, use `convert_internal_test.go` with package `k8s` to call them directly.
