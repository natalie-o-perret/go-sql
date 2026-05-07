# Copilot code review instructions for go-sql

## General Go guidelines

- Flag any use of `interface{}` or `any` where a concrete typed alternative exists.
- Prefer explicit error handling; every `error` return must be checked.
- Ensure all exported functions, types, and constants have doc comments following the `// Name ...` convention.
- Use table-driven tests; flag test functions that lack cases for edge values (nil, empty, zero rows).

## SQL / database API design

- Warn if a newly added function could be expressed as a composition of existing primitives in the `query` or `scan` package.
- Ensure generic type constraints are as narrow as needed — prefer concrete types or interface constraints over unconstrained `any`.
- Flag any raw string concatenation used to build SQL — all dynamic values must go through parameterised queries.
- Flag direct use of `database/sql` types where a wrapper in this library already provides a safer alternative.
- Prefer returning a new value over mutating a receiver; flag in-place mutation unless clearly documented.

## Test coverage

- Every exported function must have at least one test exercising the happy path and one exercising a boundary/error case.
- Flag calls to `t.Skip` without a corresponding tracking issue reference.

## Dependency hygiene

- This library targets minimal non-test dependencies; flag any new runtime dependency that is not `modernc.org/sqlite` or `go-functionalish`.
- Warn on `go.sum` changes without a corresponding `go.mod` change.
