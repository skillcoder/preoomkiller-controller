# Unit test plan: internal/infra/pinger/statistics.go

## Scope

- **File:** `internal/infra/pinger/statistics.go`
- **Package:** `pinger_test` or `pinger` (exported functions).

## Testable logic

| Target | Description |
|--------|-------------|
| `NewLatencyBuffer`, `Add`, `Len`, `GetAll` | Circular buffer behaviour. |
| `CalculatePercentile` | Percentile from sorted latencies. |
| `CalculateMedian` | Median duration. |
| `CalculateAverage` | Average duration. |

## Existing coverage

- pinger_test.go already has TestLatencyBuffer, TestCalculatePercentile, TestCalculateMedian, TestCalculateAverage. Keep and align with give/want and readability rules.

## Test cases (table-driven, give/want)

- LatencyBuffer: give capacity, give addCount, wantLen (and GetAll length).
- CalculatePercentile: give latencies, give percentile, want duration.
- CalculateMedian: give latencies, want duration (odd, even, single, empty).
- CalculateAverage: give latencies, want duration.
