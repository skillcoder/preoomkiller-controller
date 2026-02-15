# Unit test plan: internal/infra/pinger/pinger.go

## Scope

- **File:** `internal/infra/pinger/pinger.go`
- **Package:** `pinger_test` (black-box for exported API).

## Testable logic

| Target | Description |
|--------|-------------|
| Register | nil rejected; duplicate name rejected; success. |
| GetStats / GetAllStats | By name; not found; all stats. |
| Start / Shutdown / Ready | Lifecycle; Ready() closes; Shutdown waits. |
| Statistics tracking | Success/error pingers; counts; LastError. |
| IsReady / IsHealthy | Critical vs non-critical pingers. |
| Pinger timeout | Custom timeout; timeout causes error count. |

## Existing coverage

- pinger_test.go already covers these. Per AGENTS.md, replace custom mockPinger/criticalMockPinger/timeoutMockPinger with **mockery**-generated mocks for `Pinger` and use `.EXPECT()`.

## Notes

- Do not unit test `New` in isolation (constructor with no logic). Test behaviour that uses the constructed service.
