# Testing Patterns

## Framework

- Standard library `testing` package
- `github.com/stretchr/testify` (`assert` + `require`) for assertions
- Test files: `<file>_test.go` colocated with source under `internal/<pkg>/`

## Filesystem Isolation

- Use `t.TempDir()` for any test that touches disk — automatically cleaned up
- No memfs / virtual fs layer; tests work against real temp directories

## Mocking Strategy

- Mock the `executor.Executor` interface to avoid real shell commands in `loop` tests
- Override `output.Writer` / `output.ErrWriter` in tests to capture or suppress console output
- Restore overrides via `t.Cleanup` or deferred resets

## Test Organization

- Unit tests live next to the code they test (`internal/config/config_test.go`, etc.)
- Integration-style tests verify command behavior end-to-end via the cobra command tree in `internal/cli/`
- Table-driven tests with subtests (`t.Run(name, ...)`) are preferred

## Concurrency

- Loop/parallel code is tested by exercising the `Executor` interface with controllable mocks
- Use `context.Context` cancellation paths in tests where relevant

## Coverage

- `make test-coverage` produces a coverage profile under `coverage/`
- No enforced thresholds in CI yet
