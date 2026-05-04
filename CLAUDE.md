# gogo-meta

A modern Go CLI for managing multi-repository projects. Reimplementation of [gogo-meta](https://github.com/daFish/gogo-meta/tree/6ae349afce42af1081c6c40d64a0affb708ff562), originally written in TypeScript and rewritten in Go from commit `6ae349afce42af1081c6c40d64a0affb708ff562` of the TS codebase.

## Project Overview

gogo-meta allows developers to manage multiple git repositories as a unified system. It executes commands across all child repositories defined in a `.gogo` configuration file.

## Tech Stack

- **Language**: Go 1.24+
- **CLI Framework**: cobra
- **YAML**: gopkg.in/yaml.v3
- **Terminal Styling**: fatih/color
- **Testing**: testing (stdlib) + testify
- **Linting**: golangci-lint v2
- **Build**: Makefile with ldflags version injection

## Project Structure

```
cmd/gogo/
└── main.go                # Entry point, version injection via ldflags

internal/
├── cli/
│   ├── root.go            # Root command, persistent flags, overlay preRun
│   ├── helpers.go         # Shared flag helpers (addFilterFlags, resolveFilterOptions, etc.)
│   ├── init.go            # gogo init
│   ├── exec.go            # gogo exec
│   ├── run.go             # gogo run
│   ├── validate.go        # gogo validate
│   ├── git.go             # gogo git (parent command)
│   ├── git_clone.go       # gogo git clone
│   ├── git_update.go      # gogo git update
│   ├── git_status.go      # gogo git status
│   ├── git_pull.go        # gogo git pull
│   ├── git_push.go        # gogo git push
│   ├── git_branch.go      # gogo git branch
│   ├── git_checkout.go    # gogo git checkout
│   ├── git_commit.go      # gogo git commit
│   ├── project.go         # gogo project (parent command)
│   ├── project_create.go  # gogo project create
│   ├── project_import.go  # gogo project import
│   ├── npm.go             # gogo npm (parent command)
│   ├── npm_install.go     # gogo npm install / ci
│   ├── npm_link.go        # gogo npm link
│   └── npm_run.go         # gogo npm run
├── config/
│   ├── config.go          # Types, read/write, merge, find-up, validation, overlay
│   ├── config_test.go
│   ├── gitignore.go       # addToGitignore helper
│   └── gitignore_test.go
├── executor/
│   ├── executor.go        # Executor interface, shell command execution
│   └── executor_test.go
├── filter/
│   ├── filter.go          # Include/exclude + looprc filtering
│   └── filter_test.go
├── loop/
│   ├── loop.go            # Sequential + parallel orchestration
│   └── loop_test.go
├── output/
│   ├── output.go          # Terminal formatting, symbols, summary
│   └── output_test.go
└── ssh/
    ├── ssh.go             # SSH host extraction, known_hosts
    └── ssh_test.go
```

## Commands

```bash
make help           # Show all available targets
make build          # Build the gogo binary to dist/gogo
make docker         # Build the gogo container image locally (Dockerfile.local)
make fmt            # Run go fmt
make lint           # Run golangci-lint
make test           # Run all tests
make test-coverage  # Run tests and generate coverage report (coverage/)
make clean          # Remove build artifacts (dist/, coverage/)
make all            # Clean, lint, test-coverage, then build
```

## CLI Usage

```bash
gogo init                          # Create .gogo file
gogo exec "<command>" [--parallel] # Run command across repos
gogo run [name]                    # Run predefined command from .gogo
gogo git clone <url>               # Clone meta + children
gogo git update                    # Clone missing repos
gogo git status|pull|push|branch|checkout|commit
gogo project create|import
gogo npm install|ci|link|run

# Global options
gogo -f .gogo.devops exec "..."    # Merge additional config file
gogo -f a.yaml -f b.yaml exec "..."  # Multiple overlays
```

## Code Conventions

- Follow idiomatic Go patterns and standard project layout
- Use `internal/` for all non-exported packages
- Use the `Executor` interface for testability (mock in tests, real shell in production)
- Use `context.Context` for cancellation and timeout propagation
- Use `sync.WaitGroup` + channels for parallel execution
- Use `0o755` / `0o644` octal literal style
- Handle errors explicitly — no ignored return values (enforced by errcheck linter)
- Use `fatih/color` for terminal styling (not other color libraries)
- Custom `UnmarshalJSON` / `UnmarshalYAML` on `CommandConfig` to handle the string | object union type

## Testing Patterns

- Unit tests use `t.TempDir()` for filesystem isolation
- Mock `executor.Executor` interface to avoid real shell commands in loop tests
- Override `output.Writer` / `output.ErrWriter` to suppress and capture console output in tests
- Table-driven tests with `testify/assert` and `testify/require`
- Integration tests verify command behavior end-to-end

## Configuration Files

### .gogo (project config)

Supports both JSON (`.gogo`) and YAML (`.gogo.yaml` / `.gogo.yml`) formats.
Precedence: `.gogo` > `.gogo.yaml` > `.gogo.yml`.

```json
{
  "projects": {
    "path/to/repo": "git@github.com:org/repo.git"
  },
  "ignore": [".git", "node_modules"],
  "commands": {
    "build": "npm run build",
    "test": { "cmd": "npm test", "parallel": true }
  }
}
```

```yaml
projects:
  path/to/repo: git@github.com:org/repo.git
ignore:
  - .git
  - node_modules
commands:
  build: npm run build
  test:
    cmd: npm test
    parallel: true
```

### .looprc (optional filtering)

```json
{
  "ignore": ["docs", "examples"]
}
```
