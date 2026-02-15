# Unit test plan: internal/infra/appstate/handlers.go

## Scope

- **File:** `internal/infra/appstate/handlers.go`
- **Test file:** `handlers_internal_test.go`, **package appstate** (handlers take unexported interfaces).

## Testable logic

| Target | Description |
|--------|-------------|
| HandleHealthz | IsHealthy() → 200 or 503. |
| HandleReadyz | IsReady() → 200 or 503. |
| HandleStatus | GetState, GetUptime, GetStartTime → 200 + JSON body. |

## Test cases (table-driven, give/want)

- HandleHealthz: giveHealthy (mock .EXPECT().IsHealthy().Return(...)), wantStatus.
- HandleReadyz: giveReady, wantStatus.
- HandleStatus: giveState, giveUptime, giveStartTime, wantStatus 200, wantBody (JSON fields).

## Mocks

- Use **mockery**-generated mocks for `healthChecker`, `readyChecker`, `statusGetter` (unexported interfaces; configure mockery for same package). Use `.EXPECT()` only.
- Use httptest.ResponseRecorder and http.Request; require for assertions.
