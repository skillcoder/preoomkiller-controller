# Unit test plan: internal/logic/controller/service.go

## Scope

- **File:** `internal/logic/controller/service.go`
- **Test file:** `*_internal_test.go`, **package controller** (same package for unexported symbols).

## Testable logic

| Target | Description |
|--------|-------------|
| `resolveMemoryThreshold` | Already tested. Keep; align case struct with give/want. |
| `resolveMemoryThresholdFromPercent` | Covered indirectly via `resolveMemoryThreshold`. |
| `getPodMemoryUsageOrSkip` | GetPodMetricsQuery → not found / nil / zero / error / success. |
| `evictPodCommand` | EvictPodCommand → success, not found, too many requests, other error. |
| `processPod` | Threshold resolution + metrics + eviction; many branches. |
| `ReconcileCommand` | ListPodsQuery, then processPod per pod; context done. |
| Service lifecycle | Start, Ready(), Shutdown; Ping before/after ready, reconcile age. |

## Test cases (table-driven, give/want, mockery .EXPECT())

- **TestService_getPodMemoryUsageOrSkip**: givePod, giveMetrics/giveMetricsErr, wantSkip, wantErr, wantUsage.
- **TestService_evictPodCommand**: giveEvictErr, wantOk, wantErr (use k8s adapter errors or test types for notFound/tooManyRequests).
- **TestService_processPod**: givePod, giveMetrics/giveMetricsErr, giveEvictErr, wantEvicted, wantErr.
- **TestService_ReconcileCommand**: giveListPods, giveContextDone, wantErr.
- **TestService_Start_Ready_Shutdown**, **TestService_Ping**: one scenario each; mock Repository with .EXPECT().

## Mocks

- Use **mockery**-generated mock for `Repository`. Set expectations with `.EXPECT()` only.
