# Unit test plan: internal/infra/appstate/appstate.go

## Scope

- **File:** `internal/infra/appstate/appstate.go`
- **Package:** `appstate_test` (black-box) for exported API.

## Testable logic

| Target | Description |
|--------|-------------|
| State transitions | SetStarting, SetRunning, SetTerminating, Shutdown; invalid transitions (e.g. init→running, terminated→starting). |
| Query methods | GetState, GetStartTime, IsHealthy, IsReady, GetUptime. |
| RegisterPinger / RegisterShutdowner | Registration and use in shutdown/health. |
| Shutdown | Idempotent after terminated; error on double shutdown. |

## Existing coverage

- appstate_test.go covers state transitions, query methods, GetUptime, Shutdown. Keep and align with give/want and readability.

## What not to unit test

- `New`: constructor that only assigns fields (no branching logic).
