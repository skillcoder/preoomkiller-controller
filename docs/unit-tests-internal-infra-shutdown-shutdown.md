# Unit test plan: internal/infra/shutdown/shutdown.go

## Scope

- **File:** `internal/infra/shutdown/shutdown.go`
- **Package:** `shutdown_test` (black-box for exported functions).

## Testable logic

| Target | Description |
|--------|-------------|
| CheckTerminationFile | File exists → true; file missing / not exist → false. |
| GracefulShutdown | Shutdowns components in reverse order; collects errors; timeout. |
| CheckTermination | Context done → error; termination file exists → error; else nil. |

## Test cases (table-driven, give/want)

- **TestCheckTerminationFile**: givePath (temp dir + optional file), want bool. Use t.Context().
- **TestGracefulShutdown**: giveShutdowners (mockery Shutdowner mocks with .EXPECT().Shutdown(), .EXPECT().Name()), wantErr. Cases: empty list; one success; one error; reverse order.
- **TestCheckTermination** (optional): giveCtxCancelled / giveTerminationFileExists, wantErr.

## Mocks

- Use **mockery**-generated mock for `Shutdowner`; set expectations with `.EXPECT()`.
