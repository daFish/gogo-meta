# gogo-meta — Project Overview

## Purpose
A modern Go CLI for managing multi-repository projects. Reimplementation of [meta](https://github.com/mateodelnorte/meta).
Allows developers to manage multiple git repositories as a unified system by executing commands across all child repositories defined in a `.gogo` configuration file.

Binary name: `gogo`. Module path: `github.com/daFish/gogo-meta`.

## Tech Stack
- **Language**: Go 1.24+
- **CLI Framework**: spf13/cobra
- **YAML**: gopkg.in/yaml.v3
- **Terminal Styling**: fatih/color
- **Testing**: stdlib `testing` + stretchr/testify
- **Linting**: golangci-lint v2 (config in `.golangci.yml`)
- **Build**: Makefile with ldflags version injection
- **Release**: goreleaser + semantic-release

## Project Structure
```
cmd/gogo/
└── main.go                    # Entry point, version injection via ldflags

internal/
├── cli/
│   ├── root.go                # Root command, persistent flags, overlay preRun
│   ├── helpers.go             # Shared flag helpers (addFilterFlags, resolveFilterOptions, etc.)
│   ├── init.go                # gogo init
│   ├── exec.go                # gogo exec
│   ├── run.go                 # gogo run
│   ├── validate.go            # gogo validate
│   ├── git.go                 # gogo git (parent command)
│   ├── git_clone.go           # gogo git clone
│   ├── git_update.go          # gogo git update
│   ├── git_status.go          # gogo git status
│   ├── git_pull.go            # gogo git pull
│   ├── git_push.go            # gogo git push
│   ├── git_branch.go          # gogo git branch
│   ├── git_checkout.go        # gogo git checkout
│   ├── git_commit.go          # gogo git commit
│   ├── project.go             # gogo project (parent command)
│   ├── project_create.go      # gogo project create
│   ├── project_import.go      # gogo project import
│   ├── npm.go                 # gogo npm (parent command)
│   ├── npm_install.go         # gogo npm install / ci
│   ├── npm_link.go            # gogo npm link
│   └── npm_run.go             # gogo npm run
├── config/                    # Types, read/write, merge, find-up, validation, overlay, gitignore
├── executor/                  # Executor interface + shell command execution
├── filter/                    # Include/exclude + .looprc filtering
├── loop/                      # Sequential + parallel orchestration
├── output/                    # Terminal formatting, symbols, summary
└── ssh/                       # SSH host extraction, known_hosts
```

Tests live next to source files in each `internal/<pkg>/` directory (`*_test.go`).

## Docker
- `Dockerfile` — production image
- `Dockerfile.local` — local dev image
- `.goreleaser.yaml` drives release builds and image publishing

## Configuration Files
- `.gogo` — Main project config; supports JSON (`.gogo`) and YAML (`.gogo.yaml` / `.gogo.yml`). Precedence: `.gogo` > `.gogo.yaml` > `.gogo.yml`.
- `.looprc` — Optional filtering config (ignore list)
- Overlay files merged via repeatable `-f, --file` global flag
