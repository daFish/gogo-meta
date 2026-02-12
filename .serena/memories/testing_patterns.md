# Testing Patterns

## Framework

- **Vitest 4.x** with globals enabled (`globals: true` in config)
- Test files: `tests/**/*.test.ts`
- Global setup: `tests/setup.ts` (restores mocks after each test, resets `process.exitCode`)

## Filesystem Mocking

- Uses **memfs** to mock the filesystem
- Pattern: `vi.mock('node:fs/promises', async () => { const memfs = await import('memfs'); return memfs.fs.promises; })`
- Reset volume in `beforeEach`: `vol.reset()`

## Mocking Strategy

- Mock `src/core/executor.ts` to avoid real shell commands
- Mock `src/core/output.ts` to suppress console output
- Use `vi.restoreAllMocks()` in afterEach

## Test Organization

- `tests/unit/core/` — Unit tests for core modules (config, executor, filter, loop, output, ssh)
- `tests/integration/` — Integration tests (exec, init, run, git, project)
- `tests/e2e/` — End-to-end tests

## Coverage

- Provider: V8
- Includes: `src/**/*.ts`
- Excludes: `src/**/*.test.ts`, `src/index.ts`
- Thresholds: statements 40%, branches 70%, functions 60%, lines 40%
