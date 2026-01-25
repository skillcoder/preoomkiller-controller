# Refactoring Plan: Extract Logic from main.go to Hexagonal Architecture

## Current State Analysis

The `main.go` file currently contains:

- Controller struct and all business logic
- Pod eviction logic
- Pod processing logic  
- K8s client initialization
- Signal handling
- Application wiring
- Constants

## Target Architecture (Following AGENTS.md)

Following hexagonal architecture with these layers:

- `cmd/preoomkiller-controller/main.go` - Minimal entry point
- `internal/app/app.go` - Application wiring (`New()` and `Run()`)
- `internal/logic/controller/` - Business logic layer
- `internal/adapters/outbound/k8s/` - K8s API adapter (secondary adapter)
- `internal/infra/shutdown/` - Graceful shutdown handling

## Refactoring Steps

### 1. Create K8s Adapter (Secondary Adapter)

**Location**: `internal/adapters/outbound/k8s/`

- **Files to create**:
  - `adapter.go` - Implements K8s client interface
  - `interfaces.go` - Defines repository port interfaces (private)
  - `errors.go` - Package-specific errors
  - `convert.go` - Conversion functions if needed

- **Responsibilities**:
  - Wrap K8s clientset and metrics clientset
  - Provide methods for:
    - Listing pods with label selector (Query)
    - Getting pod metrics (Query)
    - Evicting pods (Command)
  - Translate K8s API errors to domain errors
  - All methods must have `Command` or `Query` suffix (CQRS)

- **Interface pattern**:
  ```go
  type k8sClient interface {
    ListPodsQuery(ctx context.Context, labelSelector string) ([]Pod, error)
    GetPodMetricsQuery(ctx context.Context, namespace, name string) (*PodMetrics, error)
    EvictPodCommand(ctx context.Context, namespace, name string) error
  }
  ```


### 2. Create Controller Logic Layer

**Location**: `internal/logic/controller/`

- **Files to create**:
  - `service.go` - Use case service with business logic
  - `interfaces.go` - Repository port interfaces (private) and error checking interfaces
  - `errors.go` - Package-specific errors
  - `dto.go` - Domain DTOs (Pod, PodMetrics, etc.)

- **Responsibilities**:
  - Business logic for pod processing
  - Memory threshold checking
  - Reconciliation loop logic
  - Pure domain logic with zero external dependencies
  - Dependencies are interfaces (ports), not implementations

- **Service methods**:
  - `ProcessPodQuery(ctx context.Context, pod Pod) (bool, error)` - Check if pod should be evicted
  - `ReconcileCommand(ctx context.Context) error` - Run reconciliation loop
  - `RunCommand(ctx context.Context, interval time.Duration) error` - Main controller loop

- **Error handling**:
  - Use private `notFound` interface pattern for checking errors
  - Define domain errors in `errors.go`
  - Use `errors.As` for error checking

### 3. Create Application Wiring

**Location**: `internal/app/`

- **Files to create**:
  - `app.go` - Contains `New()` and `Run()` methods
  - `interfaces.go` - Application-level interfaces if needed

- **Responsibilities**:
  - Wire dependencies in correct order:

    1. Create secondary adapters (K8s adapter)
    2. Create logic use case services (inject repository adapters)
    3. Initialize application state

  - Handle graceful shutdown
  - Initialize logger and set as default
  - Create and manage context lifecycle

- **Structure**:
  ```go
  type App struct {
    controller *controller.Service
    // other dependencies
  }
  
  func New(cfg *config.Config) (*App, error)
  func (a *App) Run(ctx context.Context) error
  ```


### 4. Create Shutdown Infrastructure

**Location**: `internal/infra/shutdown/`

- **Files to create**:
  - `shutdown.go` - Signal handling and graceful shutdown
  - `interfaces.go` - Shutdowner interface

- **Responsibilities**:
  - Handle SIGTERM/SIGINT signals
  - Implement `Shutdowner` interface for components
  - Graceful shutdown coordination
  - Check for termination file (`/mnt/signal/terminating`)

### 5. Refactor main.go

**Location**: `cmd/preoomkiller-controller/main.go`

- **Changes**:
  - Minimal main function (only error handling and os.Exit)
  - Call `app.New()` and `app.Run()`
  - Remove all business logic
  - Remove signal handling (moved to infra/shutdown)
  - Remove controller logic (moved to logic/controller)
  - Remove K8s client initialization (moved to adapters)

### 6. Move Constants

**Location**: `internal/logic/controller/constants.go` or appropriate package

- Move constants:
  - `EvictionKind`
  - `EvictionAPIVersion`
  - `PreoomkillerPodLabelSelector`
  - `PreoomkillerAnnotationMemoryThresholdKey`

### 7. Update Logging

- Ensure all logging uses `slog.InfoContext(ctx, ...)` with context
- Use structured logging with camelCase field names
- Annotate logger with traceID where applicable
- All log messages lowercase

### 8. Error Handling

- Create error types in `errors.go` files
- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Implement private interface pattern for error checking
- Handle `sql.ErrNoRows` equivalent (K8s `IsNotFound`) using interface pattern

## File Structure After Refactoring

```
internal/
├── app/
│   ├── app.go              # New() and Run() methods
│   └── interfaces.go        # App-level interfaces
├── logic/
│   └── controller/
│       ├── service.go      # Use case service
│       ├── interfaces.go   # Repository ports (private)
│       ├── errors.go       # Domain errors
│       ├── dto.go          # Domain DTOs
│       └── constants.go    # Constants
├── adapters/
│   └── outbound/
│       └── k8s/
│           ├── adapter.go  # K8s client adapter
│           ├── interfaces.go # Adapter interface
│           └── errors.go   # Adapter errors
└── infra/
    ├── logging/
    │   └── logging.go      # (existing)
    └── shutdown/
        ├── shutdown.go     # Signal handling
        └── interfaces.go   # Shutdowner interface
```

## Key Principles to Follow

1. **Dependency Direction**: Outer layers depend on inner layers (Handlers → Adapters → Logic ← Adapters)
2. **Interface Compliance**: Use `var _ interfaceName = (*implementationName)(nil)` for compile-time checks
3. **CQRS Pattern**: All repository and service methods have `Command` or `Query` suffix
4. **Error Handling**: Use private interfaces for error checking, avoid package imports for error matching
5. **Logging**: Always use context-aware logging, lowercase messages, camelCase fields
6. **Graceful Shutdown**: Components with goroutines implement `Shutdowner` interface
7. **No External Dependencies in Logic**: Logic layer has zero K8s, HTTP, or framework imports

## Testing Considerations

- Logic layer tests should mock repository interfaces
- Adapter tests should test K8s API interactions (may need integration tests)
- Use mockery for generating mocks (v3.6.1)
- Test error translation in adapters
- Test business logic in isolation

## Migration Notes

- Keep existing functionality intact during refactoring
- Maintain backward compatibility with config
- Ensure all error cases are properly handled
- Verify graceful shutdown works correctly
- Check termination file handling
