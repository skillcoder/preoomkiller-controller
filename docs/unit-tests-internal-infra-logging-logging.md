# Unit test plan: internal/infra/logging/logging.go

## Scope

- **File:** `internal/infra/logging/logging.go`
- **Package:** `logging_test` (black-box).

## Testable logic

| Target | Description |
|--------|-------------|
| New | logFormat (json/text/default) and logLevel (debug/warn/error/info/default) → slog.Logger with correct level and handler type. |

## Test cases (table-driven, give/want)

- **TestNew**: giveLogFormat, giveLogLevel, want: logger level and handler behaviour (e.g. level set, or handler not nil). Avoid asserting on global slog.SetDefault side effect; assert returned logger level or that Enabled returns expected for a level.

## Notes

- Constructor has branching (switch on format/level); worth lightweight unit tests for level and format selection. Keep tests simple; no need to test default slog behaviour in depth.
