# Code Style and Conventions

## Go Configuration

- Go 1.24+, module path `github.com/daFish/gogo-meta`
- Standard project layout: `cmd/<binary>/` for entry points, `internal/` for non-exported packages
- All non-exported packages live under `internal/` to prevent external imports

## Naming Conventions

- **Functions**: camelCase (unexported) / PascalCase (exported)
- **Types/Interfaces**: PascalCase (e.g., `Executor`, `MetaConfig`, `FilterOptions`, `CommandConfig`)
- **Constants**: PascalCase or ALL_CAPS for package-level constants
- **Files**: snake_case (`git_clone.go`, `project_import.go`)
- **Test files**: `<file>_test.go` next to source

## Code Patterns

- Use `context.Context` for cancellation and timeout propagation
- Use `sync.WaitGroup` + channels for parallel orchestration
- Define interfaces (e.g., `Executor`) for testability — mock in tests, real shell in production
- Octal literal style: `0o755` / `0o644` (not `0755`)
- Handle errors explicitly — no ignored returns (enforced by `errcheck` linter)
- Use `fatih/color` for terminal styling (not other color libraries)
- Custom `UnmarshalJSON` / `UnmarshalYAML` on `CommandConfig` to handle `string | object` union type
- Prefer table-driven tests with subtests (`t.Run`)

## Linting

- `golangci-lint` v2 (see `.golangci.yml`)
- `go vet` is bundled into `golangci-lint run` (`govet` linter); no standalone `make vet` target
- Active linters include `errcheck`, `govet`, `staticcheck`, `ineffassign`, `unused`, etc.

## Build

- Build via `make build` → output `dist/gogo`
- Version injected at build time through `-ldflags` (see `Makefile`)
- Release binaries built with `goreleaser` (config in `.goreleaser.yaml`)
