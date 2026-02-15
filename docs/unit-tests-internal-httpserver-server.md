# Unit test plan: internal/httpserver/server.go

## Scope

- **File:** `internal/httpserver/server.go`
- **Package:** `httpserver_test` (black-box).

## Testable logic

| Target | Description |
|--------|-------------|
| New | Empty port → default port; non-empty port used. |
| Name | Returns "http-server". |
| Ping | Not ready → error; ready → nil. |

## What not to unit test

- `Start`: wires listener and goroutine; integration or shallow test with real listener (e.g. :0) if needed.
- `Shutdown`: delegates to http.Server.Shutdown; integration or minimal test with started server.

## Test cases (table-driven, give/want)

- **TestNew**: givePort (empty / non-empty), wantPort on created Server.
- **TestServer_Name**: want "http-server".
- **TestServer_Ping**: giveReady (false = before Start, true = after Start and Ready()), wantErr. For ready: start on :0, wait Ready(), then Ping; use t.Context().
