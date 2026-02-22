# Unit test plan: internal/config/config.go

## Scope

- **File:** `internal/config/config.go`
- **Package:** `config_test` (black-box)

## Testable logic

| Target | Description |
|--------|-------------|
| `Load()` | Reads env, parses PREOOMKILLER_INTERVAL_SEC/PREOOMKILLER_PINGER_INTERVAL_SEC and other PREOOMKILLER_* vars, applies defaults. |

## Test cases (table-driven, give/want)

- **TestLoad**
  - `giveEnv`: map of env vars to set (use `t.Setenv`).
  - `wantErr`: expected error (nil for success).
  - `wantCfg`: optional; key fields to assert for success (e.g. `Interval`, `PingerInterval`, `HTTPPort`, `PodLabelSelector`).
  - Cases: all defaults; override PREOOMKILLER_HTTP_PORT, PREOOMKILLER_INTERVAL_SEC, PREOOMKILLER_PINGER_INTERVAL_SEC; invalid PREOOMKILLER_INTERVAL_SEC; invalid PREOOMKILLER_PINGER_INTERVAL_SEC.

## Notes

- Do not unit test `getEnvOrDefault` in isolation; it is covered by `Load`.
- Use `t.Context()` where a context is needed.
