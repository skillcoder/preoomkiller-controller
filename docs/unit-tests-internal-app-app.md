# Unit test plan: internal/app/app.go

## Scope

- **File:** `internal/app/app.go`
- **Test file:** `app_internal_test.go`, **package app** (same package for unexported `allChannelsClose`).

## Testable logic

| Target | Description |
|--------|-------------|
| `allChannelsClose` | Waits for all channels to close; returns channel that closes when all done. |

## What not to unit test

- `New`, `Run`, `initialize`, `startServices`, `startHTTPServer`, `startController`, `waitForReady`, `runUntilShutdown`, `Shutdown`: wiring and delegation with no branching logic; unit tests add little value. Prefer integration tests if needed.

## Test cases (table-driven, give/want)

- **TestAllChannelsClose**
  - giveNumChannels, giveContextCancelBeforeClose (or equivalent), want (returned channel closes).
  - Cases: 0 channels (closes immediately); 1 channel; 2 channels; context cancelled before any channel closes.
  - Use `t.Parallel()` only if safe; do not use `tt := tt`.
