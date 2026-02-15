# Files excluded from unit testing

This document lists Go files for which **unit tests are not required** because they contain no or negligible logic (routers, pure constructors, type/interface/constant/error definitions, or code that only delegates to external APIs). See AGENTS.md rules: do not unit test constructors or methods without logic.

---

## Entry points and examples

| File | Reason |
|------|--------|
| `cmd/preoomkiller-controller/main.go` | Entry point; `main()` and `os.Exit`; no testable logic. |
| `examples/memory-leaker/main.go` | Example program; not part of production test scope. |

---

## No logic: types, interfaces, constants, errors only

| File | Reason |
|------|--------|
| `internal/logic/controller/constants.go` | Constants only; no behaviour. |
| `internal/logic/controller/dto.go` | Struct definitions only; no behaviour. |
| `internal/logic/controller/errors.go` | Error variable declarations only; no behaviour. |
| `internal/logic/controller/interfaces.go` | Interface definitions only; no behaviour. |
| `internal/app/interfaces.go` | Interface definitions only; no behaviour. |
| `internal/adapters/outbound/k8s/errors.go` | Error type definitions and methods (no branching logic). |
| `internal/infra/pinger/interfaces.go` | Interface definitions only; no behaviour. |
| `internal/infra/pinger/errors.go` | Error variable declarations only; no behaviour. |
| `internal/infra/appstate/interfaces.go` | Interface definitions only; no behaviour. |
| `internal/infra/appstate/errors.go` | Error variable declarations only; no behaviour. |
| `internal/infra/shutdown/interfaces.go` | Interface definitions only; no behaviour. |
| `internal/httpserver/interfaces.go` | Interface definitions only; no behaviour. |
| `internal/httpserver/constants.go` | Constants only; no behaviour. |

---

## Routers / thin delegation / no branching logic

| File | Reason |
|------|--------|
| `internal/adapters/outbound/k8s/adapter.go` | Implements `Repository` by delegating to K8s clientset; no pure logic to unit test. Prefer integration tests against a cluster or fake server. |

---

## Test files

| File | Reason |
|------|--------|
| `internal/logic/controller/service_internal_test.go` | Test file; not a production target. |
| `internal/infra/pinger/pinger_test.go` | Test file; not a production target. |
| `internal/infra/appstate/appstate_test.go` | Test file; not a production target. |

---

## Summary

- **Do not** add unit tests for: entry points, examples, files that only define types/interfaces/constants/errors, or code that only wires and delegates without branching logic.
- **Do** add unit tests for files with real logic (see per-file plans in `docs/unit-tests-internal-*.md`).
