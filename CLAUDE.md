# gogo-meta

A modern Go CLI for managing multi-repository projects. Reimplementation of [gogo-meta](https://github.com/daFish/gogo-meta/tree/44344a19bfc70995b142f49a51316dbe126e9f8f), originally written in TypeScript and rewritten in Go from commit `44344a19bfc70995b142f49a51316dbe126e9f8f` of the TS codebase.

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
│   ├── migrate.go         # gogo migrate
│   ├── update.go          # gogo update (move + clone + fast-forward)
│   ├── clone.go           # shared clone phase (gogo update, gogo git update)
│   ├── hints.go           # shared user-facing hints
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
│   ├── filter.go          # Include/exclude filtering
│   └── filter_test.go
├── gitstate/
│   ├── gitstate.go        # porcelain=v2 inspection, fetch, ahead/behind, --ff-only
│   └── gitstate_test.go
├── loop/
│   ├── loop.go            # Sequential + parallel orchestration, RunEach worker pool
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
gogo exec --group foo "<command>"  # Restrict to the projects of group foo
gogo run [name]                    # Run predefined command from .gogo
gogo validate                      # Validate config file(s)
gogo update [--dry-run]            # Converge working copy: move + clone + fast-forward
gogo migrate [--dry-run]           # Move/rename working-copy dirs to match config
gogo git clone <url>               # Clone meta + children
gogo git update                    # Clone missing repos
gogo git status|pull|push|branch|checkout|commit
gogo project create|import
gogo npm install|ci|link|run

# Global options
gogo -f .gogo.devops exec "..."    # Merge additional config file
gogo -f a.yaml -f b.yaml exec "..."  # Multiple overlays
gogo --group foo,bar exec "..."    # Filter by named group(s) from the config
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
  "groups": {
    "foo": ["path/to/repo"]
  },
  "commands": {
    "build": "npm run build",
    "test": { "cmd": "npm test", "parallel": true },
    "deploy": { "cmd": "make deploy", "groups": ["foo"] }
  }
}
```

```yaml
projects:
  path/to/repo: git@github.com:org/repo.git
ignore:
  - .git
  - node_modules
groups:
  foo:
    - path/to/repo
commands:
  build: npm run build
  test:
    cmd: npm test
    parallel: true
  deploy:
    cmd: make deploy
    groups:
      - foo
```

### Groups

Groups are named sets of project paths. They are resolved to project paths by
`config.ResolveGroups` and applied as the first stage of `filter.Apply`
(`Options.GroupOnly`), so they intersect with the other filters.

Group and command cross-references are checked lazily, not while reading a
config: `Validate` covers a single file structurally, `ResolveGroups` reports
bad references when a group is actually used, and `config.ValidateReferences`
checks them for `gogo validate` — against the *merged* config plus every config
file found in the cwd and next to the primary one, so an overlay that was not
loaded still counts.
Reading stays permissive because an overlay may group projects declared in the
base config and vice versa.

### Converging the working copy

`gogo update` is the one command that makes the working copy match the config:
it moves projects checked out at another path, clones the missing ones, and
fast-forwards the rest. `gogo migrate` and `gogo git update` remain as the
single-purpose primitives it is composed from.

The three phases share `planMigration` (`internal/cli/migrate.go`), which buckets
every selected project into `present`, `moves`, `missing`, `ambiguous` or
`conflicts`. A project belongs to exactly one bucket and is reported exactly
once: `runUpdate` carries a mutable pull set through the phases rather than
re-planning from disk after the moves, since a re-plan would reclassify projects
an earlier phase already reported.

The pull phase never merges, rebases, stashes or switches branches. It reads
`git status --porcelain=v2 --branch` through `internal/gitstate` — machine
output, because git localizes its prose and the executor cannot pin `LC_ALL` —
then fetches and advances with `git merge --ff-only @{u}`. A dirty working tree
is not a gate; git's own refusal is what guards it.
