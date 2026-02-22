# Scheduled pod restart (restart-schedule + tz annotations)

## Problem

Slow memory leaks cause pods to eventually OOM during business hours.
Instead of waiting for memory to grow, restart pods on a cron schedule during
low-usage (non-business) hours.

## Annotations

### User-defined (on pod template)

```yaml
preoomkiller.beta.k8s.skillcoder.com/restart-schedule: "40 7 * * *"
preoomkiller.beta.k8s.skillcoder.com/tz: "America/New_York"
```

- **restart-schedule** — standard 5-field cron (minute-first). Required for scheduled restart.
- **tz** — IANA timezone for the schedule. Optional; defaults to UTC.
  Ignored when schedule uses inline `CRON_TZ=`.

Inline `CRON_TZ=` is also supported: `"CRON_TZ=America/New_York 0 6 * * *"`.

### Controller-managed (written at runtime)

```yaml
preoomkiller.beta.k8s.skillcoder.com/restart-at: "2026-02-16T07:40:00-05:00"
```

- **restart-at** — ISO 8601 (RFC 3339) timestamp in the corresponding timezone
  (UTC when no tz annotation). Written by the controller when it decides to
  schedule a restart. This annotation **is the state** — the K8s API is the
  single source of truth for pending restarts.

Because `restart-at` is a runtime annotation on the pod object (not on the
pod template), it disappears when the pod is evicted and recreated.

Pods may have `restart-schedule` only, `memory-threshold` only, or both.
Eviction fires when **either** condition is met.

## Cron library

Use **`github.com/netresearch/go-cron`** — maintained fork of `robfig/cron` (100% API compatible,
Go 1.25+, fixes panics on malformed `TZ=`, proper DST handling).

Imported only in `internal/infra/cronparser` (not in the logic layer). The
logic layer depends on a private `scheduleParser` interface.

## Strategy: annotation-based state + goroutine scheduling

On each reconcile, for each pod with `restart-schedule`, the controller:
1. Reads `restart-at` annotation from `pod.Annotations` (state).
2. Reads `restart-schedule` and `tz` annotations from `pod.Annotations` (cron spec).
3. Parses the cron spec via injected `scheduleParser` interface to compute
   `schedule.Next(now)`.
4. Decides: recover, evict (missed), or write annotation + schedule goroutine.

State management is fully offloaded to the K8s API. The controller never
relies on in-memory state for scheduling decisions — only for goroutine
de-duplication.

### Reconcile flow per pod with `restart-schedule`

```
processScheduledRestart(pod):

  1. Read restart-at annotation from pod.Annotations
     ├─ exists, timestamp in future
     │    → schedule goroutine (recovery after controller restart, de-dup)
     │
     ├─ exists, timestamp in past, pod created BEFORE timestamp
     │    → missed eviction → evict immediately
     │
     └─ exists, timestamp in past, pod created AFTER timestamp
          → stale (pod was already restarted) → log warning, fall through to 2

  2. No valid restart-at (absent, stale, or invalid):
     Read restart-schedule + tz from pod.Annotations
     Parse via scheduleParser.NextAfter(spec, tz, now)
       → on success: write restart-at annotation to pod via repo
       → on annotation write success: schedule goroutine
```

### State recovery (controller restart)

On restart the controller has no in-memory state. During the first reconcile:
- Pods with `restart-at` in the **future** → goroutine is re-scheduled.
- Pods with `restart-at` in the **past** → missed eviction, evict now.

No state is lost.

### Jitter

Random jitter in `[0, jitterMax]` added to goroutine delay to avoid
thundering herd. Configurable via env (default 30 s).

### Goroutine de-duplication

The controller keeps a `pendingTimers map[string]*time.Timer` (keyed by
`namespace/name`) solely to prevent scheduling multiple goroutines for the
same pod. This is **not** state management — the annotation is the state.

The map entry exists from goroutine creation until eviction completes
(success or failure), preventing duplicate eviction attempts during
overlapping reconcile cycles.

### Shutdown

The controller's existing `Shutdown` is extended to:
1. Stop all pending eviction timers (cancel goroutines that haven't fired).
2. Wait for any in-flight eviction goroutines to finish.

## Architecture

```
Controller Service (logic)
  │
  ├─ ReconcileCommand
  │    ├─ repo.ListPodsQuery(ctx, label)          → K8s API (unchanged)
  │    │
  │    ├─ for each pod with restart-schedule annotation:
  │    │     s.processScheduledRestart(pod)
  │    │       ├─ read restart-at, restart-schedule, tz from pod.Annotations
  │    │       ├─ s.scheduleParser.NextAfter(spec, tz, now)
  │    │       ├─ restart-at exists, future → s.scheduleEviction(...)
  │    │       ├─ restart-at exists, past, missed → s.evictPodCommand(...)
  │    │       └─ no/stale restart-at
  │    │            → repo.SetAnnotationCommand(restart-at = next)
  │    │            → s.scheduleEviction(...)
  │    │
  │    └─ for each pod with memory-threshold annotation:
  │          s.processPod (existing, unchanged)
  │
  ├─ scheduleEviction (private)
  │    └─ time.AfterFunc(delay + jitter)
  │         └─ repo.EvictPodCommand(ctx, ns, name)
  │
  └─ Shutdown
       ├─ stop pending timers
       └─ wait for in-flight evictions

Infra: cronparser
  └─ Implements scheduleParser interface using go-cron
```

## Changes (file by file)

### 1. Dependency — `go.mod`

```
go get github.com/netresearch/go-cron
```

### 2. Constants — `internal/logic/controller/constants.go`

```go
PreoomkillerAnnotationRestartScheduleKey = "preoomkiller.beta.k8s.skillcoder.com/restart-schedule"
PreoomkillerAnnotationTZKey              = "preoomkiller.beta.k8s.skillcoder.com/tz"
PreoomkillerAnnotationRestartAtKey       = "preoomkiller.beta.k8s.skillcoder.com/restart-at"
```

### 3. Config — `internal/config/config.go`

```go
AnnotationRestartScheduleKey string         // env PREOOMKILLER_ANNOTATION_RESTART_SCHEDULE
AnnotationTZKey              string         // env PREOOMKILLER_ANNOTATION_TZ
RestartScheduleJitterMax     time.Duration  // env PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX_SEC, default 30s
```

Defaults for the two annotation keys from the controller constants.
**restart-at** annotation key is not configurable — the controller uses
the hardcoded constant `PreoomkillerAnnotationRestartAtKey`. Jitter parsed
as integer seconds (same pattern as `Interval`).

### 4. Pod DTO — `internal/logic/controller/dto.go`

Add only `CreatedAt`. No `ScheduledRestartAt` — the logic layer reads raw
annotations and computes the next run via `scheduleParser`.

```go
type Pod struct {
    Name        string
    Namespace   string
    Annotations map[string]string
    MemoryLimit *resource.Quantity
    // CreatedAt is the pod creation timestamp; used to detect missed
    // scheduled restarts after controller downtime.
    CreatedAt time.Time
}
```

### 5. Interfaces — `internal/logic/controller/interfaces.go`

`ListPodsQuery` **unchanged**. New `SetAnnotationCommand` on repo.
New private `scheduleParser` interface for cron computation.

```go
type Repository interface {
    ListPodsQuery(
        ctx context.Context,
        labelSelector string,
    ) ([]Pod, error)

    GetPodMetricsQuery(
        ctx context.Context,
        namespace,
        name string,
    ) (*PodMetrics, error)

    EvictPodCommand(
        ctx context.Context,
        namespace,
        name string,
    ) error

    // SetAnnotationCommand sets (or removes when value is empty) a single
    // annotation on the given pod via a merge-patch.
    SetAnnotationCommand(
        ctx context.Context,
        namespace,
        name string,
        key,
        value string,
    ) error
}

// scheduleParser computes the next cron occurrence.
// Implemented by infra/cronparser using go-cron.
type scheduleParser interface {
    NextAfter(spec, tz string, after time.Time) (time.Time, error)
}
```

### 6. Cron parser — `internal/infra/cronparser/parser.go` (new package)

Small infra package wrapping `go-cron`. Only place in the codebase that
imports the cron library.

```go
package cronparser

import (
    "fmt"
    "strings"
    "time"

    cron "github.com/netresearch/go-cron"
)

var _parser = cron.NewParser(
    cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// Parser computes next cron occurrences using go-cron.
type Parser struct{}

// New creates a new cron parser.
func New() *Parser {
    return &Parser{}
}

// NextAfter returns the next cron occurrence strictly after `after`.
// If tz is non-empty and the spec has no CRON_TZ=/TZ= prefix,
// it prepends CRON_TZ=<tz>. Defaults to UTC when no tz is given.
func (p *Parser) NextAfter(
    spec,
    tz string,
    after time.Time,
) (time.Time, error) {
    fullSpec := buildSpec(spec, tz)

    schedule, err := _parser.Parse(fullSpec)
    if err != nil {
        return time.Time{}, fmt.Errorf("parse cron spec %q: %w", spec, err)
    }

    return schedule.Next(after), nil
}

func buildSpec(spec, tz string) string {
    hasTZPrefix := strings.HasPrefix(spec, "CRON_TZ=") ||
        strings.HasPrefix(spec, "TZ=")

    if tz != "" && !hasTZPrefix {
        return "CRON_TZ=" + tz + " " + spec
    }

    if !hasTZPrefix {
        return "CRON_TZ=UTC " + spec
    }

    return spec
}
```

### 7. Controller service — `internal/logic/controller/service.go`

#### 7a. Struct — new fields

```go
type Service struct {
    // ... existing fields ...
    scheduleParser               scheduleParser
    annotationRestartScheduleKey string
    annotationTZKey              string
    annotationRestartAtKey       string   // always controller.PreoomkillerAnnotationRestartAtKey
    jitterMax                    time.Duration

    timerMu       sync.Mutex
    pendingTimers map[string]*time.Timer
    inFlightWg    sync.WaitGroup
}
```

#### 7b. Constructor

```go
func New(
    logger *slog.Logger,
    repo Repository,
    parser scheduleParser,
    interval time.Duration,
    labelSelector string,
    annotationMemoryThresholdKey string,
    annotationRestartScheduleKey string,
    annotationTZKey string,
    annotationRestartAtKey string, // from app: controller.PreoomkillerAnnotationRestartAtKey only
    jitterMax time.Duration,
) *Service {
    return &Service{
        // ... existing fields ...
        scheduleParser:               parser,
        annotationRestartScheduleKey: annotationRestartScheduleKey,
        annotationTZKey:              annotationTZKey,
        annotationRestartAtKey:       annotationRestartAtKey,
        jitterMax:                    jitterMax,
        pendingTimers:                make(map[string]*time.Timer, 16),
    }
}
```

#### 7c. `ReconcileCommand` — dual eviction paths

```go
func (s *Service) ReconcileCommand(ctx context.Context) error {
    logger := s.logger.With("controller", "ReconcileCommand")

    pods, err := s.repo.ListPodsQuery(ctx, s.labelSelector)
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

        // 1. Schedule-based eviction
        if _, hasSchedule := pods[i].Annotations[s.annotationRestartScheduleKey]; hasSchedule {
            s.processScheduledRestart(ctx, logger, pods[i])
        }

        // 2. Memory-threshold eviction (existing, only if annotation present)
        if _, hasThreshold := pods[i].Annotations[s.annotationMemoryThresholdKey]; hasThreshold {
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
```

#### 7d. `processScheduledRestart` — reads raw annotations, computes cron

```go
func (s *Service) processScheduledRestart(
    ctx context.Context,
    logger *slog.Logger,
    pod Pod,
) {
    logger = logger.With("pod", pod.Name, "namespace", pod.Namespace)

    // Check existing restart-at annotation (state on K8s)
    restartAtStr, hasRestartAt := pod.Annotations[s.annotationRestartAtKey]
    if hasRestartAt {
        if s.handleExistingRestartAt(ctx, logger, pod, restartAtStr) {
            return // handled (future schedule or missed eviction)
        }
        // Fall through: stale or invalid → schedule next occurrence
    }

    // Compute next cron run from raw annotations
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

    // Write restart-at annotation (K8s becomes source of truth)
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

    s.scheduleEviction(logger, pod.Namespace, pod.Name, nextRun)
}

// handleExistingRestartAt processes a pod that already has a restart-at
// annotation. Returns true if handled (future or missed), false if
// stale/invalid (caller should schedule the next occurrence).
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
        return false // invalid → fall through to reschedule
    }

    now := time.Now()

    if restartAt.After(now) {
        // Future: schedule goroutine (recovery/de-dup)
        logger.DebugContext(ctx, "recovering scheduled eviction",
            "restartAt", restartAtStr,
        )
        s.scheduleEviction(logger, pod.Namespace, pod.Name, restartAt)
        return true
    }

    // Past: check if the pod existed at the scheduled time
    if pod.CreatedAt.Before(restartAt) {
        // Pod was alive when it should have been restarted → missed
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

    // Pod created after restart-at → stale
    logger.WarnContext(ctx, "stale restart-at annotation, rescheduling",
        "restartAt", restartAtStr,
        "podCreatedAt", pod.CreatedAt.Format(time.RFC3339),
    )
    return false // fall through → reschedule with next cron occurrence
}
```

#### 7e. `scheduleEviction` — goroutine with jitter + de-duplication

```go
func (s *Service) scheduleEviction(
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
        return // goroutine already active
    }

    delay := time.Until(at)
    if delay < 0 {
        delay = 0
    }

    jitter := time.Duration(rand.Int64N(int64(s.jitterMax + 1)))

    s.inFlightWg.Add(1)

    timer := time.AfterFunc(delay+jitter, func() {
        defer s.inFlightWg.Done()

        if s.inShutdown.Load() {
            s.timerMu.Lock()
            delete(s.pendingTimers, key)
            s.timerMu.Unlock()
            return
        }

        evictCtx, cancel := context.WithTimeout(
            context.Background(),
            30*time.Second,
        )
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

        // Remove AFTER eviction to prevent duplicate goroutines
        s.timerMu.Lock()
        delete(s.pendingTimers, key)
        s.timerMu.Unlock()
    })

    s.pendingTimers[key] = timer

    logger.DebugContext(context.Background(), "scheduled eviction goroutine",
        "pod", name,
        "namespace", namespace,
        "at", at.Format(time.RFC3339),
        "delay", delay+jitter,
    )
}
```

#### 7f. `Shutdown` — extended to stop timers and wait for in-flight

```go
func (s *Service) Shutdown(ctx context.Context) error {
    if !s.inShutdown.CompareAndSwap(false, true) {
        s.logger.ErrorContext(ctx, "controller service is already shutting down")
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
```

`processPod` stays **completely unchanged**.

### 8. K8s adapter — `internal/adapters/outbound/k8s/`

The adapter is thin — just K8s API calls. No cron parsing, no schedule keys.

#### 8a. `adapter.go` — unchanged struct, new `SetAnnotationCommand`

Struct and constructor stay **unchanged** from current code (no new fields).
Only a new method:

```go
func (a *adapter) SetAnnotationCommand(
    ctx context.Context,
    namespace,
    name string,
    key,
    value string,
) error {
    annotations := map[string]any{key: value}
    if value == "" {
        annotations[key] = nil // remove annotation
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
```

#### 8b. `convert.go` — add `CreatedAt`

```go
func toDomainPod(pod *corev1.Pod) controller.Pod {
    out := controller.Pod{
        Name:        pod.Name,
        Namespace:   pod.Namespace,
        Annotations: pod.Annotations,
        CreatedAt:   pod.CreationTimestamp.Time,
    }
    // ... existing MemoryLimit logic unchanged ...
    return out
}
```

### 9. App wiring — `internal/app/app.go`

```go
cronParser := cronparser.New()

k8sRepo := k8s.New(logger, clientset, metricsClientset)

controllerService := controller.New(
    logger,
    k8sRepo,
    cronParser,
    cfg.Interval,
    cfg.PodLabelSelector,
    cfg.AnnotationMemoryThresholdKey,
    cfg.AnnotationRestartScheduleKey,
    cfg.AnnotationTZKey,
    controller.PreoomkillerAnnotationRestartAtKey, // hardcoded const, no config/env
    cfg.RestartScheduleJitterMax,
)
```

No changes to shutdowner registration — the controller service is already
registered and its `Shutdown` now handles timer cleanup.

### 10. Mockery — `.mockery.yml`

Add `scheduleParser` to the controller mock generation:

```yaml
  github.com/skillcoder/preoomkiller-controller/internal/logic/controller:
    config:
      all: false
      include-interface-regex: "^(Repository|scheduleParser)$"
      dir: internal/logic/controller/mocks
      filename: mock_repository.go
      pkgname: mocks
```

### 11. RBAC

The controller needs an additional RBAC permission to patch pods:

```yaml
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "patch"]
```

### 12. Tests

**Controller tests** (`service_test.go`):
- `ListPodsQuery` mock expectations **unchanged** (no new params).
- New `SetAnnotationCommand` mock expectations.
- New `scheduleParser.NextAfter` mock expectations.
- New case: pod with `restart-schedule`, no `restart-at` →
  `NextAfter` called, `SetAnnotationCommand` called, `EvictPodCommand`
  called (via goroutine).
- New case: pod with `restart-at` in future → goroutine re-scheduled.
- New case: pod with `restart-at` in past, `CreatedAt` before →
  `EvictPodCommand` called immediately (missed).
- New case: pod with `restart-at` in past, `CreatedAt` after →
  stale, `NextAfter` + `SetAnnotationCommand` called (reschedule).
- New case: invalid `restart-schedule` spec → `NextAfter` returns error, skip.

**Controller internal tests** (`service_internal_test.go`):
- `processScheduledRestart` unit tests covering all branches.
- `handleExistingRestartAt` tests: future, missed, stale, invalid.
- `scheduleEviction` tests: fires, de-dup, shutdown stops.

**Cronparser tests** (`internal/infra/cronparser/parser_test.go`):
- Table-driven `NextAfter` tests:
  - Standard spec → correct next time.
  - With tz → uses timezone.
  - With inline `CRON_TZ=` → ignores tz param.
  - No tz → defaults to UTC.
  - Malformed spec → error.

**Adapter tests**:
- `SetAnnotationCommand`: sets annotation, removes when empty.
- `toDomainPod`: populates `CreatedAt`.

**Config tests** (`config_test.go`):
- Verify new env vars and defaults.

### 13. Documentation — `README.md`

Add section documenting:
- The three annotations (`restart-schedule`, `tz`, `restart-at`).
- Inline `CRON_TZ=` support.
- That `restart-at` is managed by the controller (do not set manually).
- Eviction fires via goroutine at the scheduled time plus random jitter.
- Missed evictions are detected and retried on controller restart.
- New env vars: `PREOOMKILLER_ANNOTATION_RESTART_SCHEDULE`, `PREOOMKILLER_ANNOTATION_TZ`,
  `PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX_SEC`. The `restart-at` annotation key
  is not configurable (hardcoded constant).

## Files touched (summary)

| File | Change |
|------|--------|
| `go.mod`, `go.sum` | Add `github.com/netresearch/go-cron` |
| `internal/logic/controller/constants.go` | Three new annotation key constants |
| `internal/config/config.go` | Three new config fields + env loading (no restart-at key) |
| `internal/logic/controller/dto.go` | `CreatedAt time.Time` |
| `internal/logic/controller/interfaces.go` | New `SetAnnotationCommand` on repo, new `scheduleParser` interface |
| `internal/infra/cronparser/parser.go` | **New package**: `Parser` implementing `scheduleParser` |
| `internal/logic/controller/service.go` | New fields, constructor, `processScheduledRestart`, `handleExistingRestartAt`, `scheduleEviction`, `stopPendingTimers`, extended `ReconcileCommand` + `Shutdown` |
| `internal/adapters/outbound/k8s/adapter.go` | New `SetAnnotationCommand` method |
| `internal/adapters/outbound/k8s/convert.go` | `toDomainPod` populates `CreatedAt` |
| `internal/app/app.go` | Create cronparser, pass to controller |
| `.mockery.yml` | Add `scheduleParser` to regex |
| `internal/logic/controller/mocks/` | Regenerate (`just generate`) |
| `deploy/` | RBAC: add `patch` verb for pods |
| Tests | Add annotation + schedule + recovery + cronparser cases |
| `README.md` | Document annotations, env vars, scheduling + recovery |

## What stays unchanged

- `ListPodsQuery` signature — no new params
- `processPod` — no changes
- Adapter struct and constructor — no new fields (just a new method)
- Existing tests — no mock signature changes for `ListPodsQuery`

## Goroutine lifecycle

```
processScheduledRestart         scheduleEviction        AfterFunc callback        Shutdown
───────────────────────         ────────────────        ──────────────────        ────────
read annotations
scheduleParser.NextAfter()
repo.SetAnnotationCommand()
  (write restart-at to pod)
  ↓ on success
                                timerMu.Lock
                                check pendingTimers
                                inFlightWg.Add(1)
                                time.AfterFunc(delay+jitter)
                                store in pendingTimers
                                timerMu.Unlock
                                                        (delay + jitter elapses)
                                                        check inShutdown
                                                        repo.EvictPodCommand()
                                                        delete from pendingTimers
                                                        inFlightWg.Done()
                                                                                  inShutdown = true
                                                                                  stopPendingTimers()
                                                                                    → timer.Stop: wg.Done
                                                                                  wait doneCh
                                                                                  inFlightWg.Wait()
```

- Annotation on pod = source of truth (survives controller restarts).
- In-memory map = goroutine de-duplication only.
- Logic layer reads raw annotations + uses `scheduleParser` interface.
- go-cron imported only in `internal/infra/cronparser`.
