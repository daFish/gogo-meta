# gogo-meta

[![Checks](https://github.com/daFish/gogo-meta/actions/workflows/checks.yml/badge.svg?branch=main)](https://github.com/daFish/gogo-meta/actions/workflows/checks.yml?query=branch%3Amain)
[![Goreleaser](https://github.com/daFish/gogo-meta/actions/workflows/goreleaser.yml/badge.svg?branch=main)](https://github.com/daFish/gogo-meta/actions/workflows/goreleaser.yml?query=branch%3Amain)
[![Release](https://img.shields.io/github/v/release/daFish/gogo-meta)](https://github.com/daFish/gogo-meta/releases)
[![GHCR](https://img.shields.io/badge/ghcr.io-container-blue)](https://github.com/daFish/gogo-meta/pkgs/container/gogo-meta)

A modern Go CLI for managing multi-repository projects. Execute commands across multiple git repositories simultaneously.

Reimplementation of [gogo-meta](https://github.com/daFish/gogo-meta/tree/44344a19bfc70995b142f49a51316dbe126e9f8f) — originally written in TypeScript and now rewritten in Go with near-identical CLI behavior (see [Differences from the TypeScript version](#differences-from-the-typescript-version)).

> **Upgrading from the TypeScript version (v1.x / v2.x)?** See the [upgrade guide](UPGRADE.md) for step-by-step migration instructions and the behavior changes in v3.

## Features

- Clone entire project ecosystems with one command
- Execute arbitrary commands across all repositories
- Parallel or sequential execution modes
- Flexible filtering (include/exclude by name or pattern)
- NPM operations across all projects
- Symlink projects for local development
- JSON and YAML configuration formats
- Multiple config files with `-f` flag (like Docker Compose)

## Differences from the TypeScript version

The Go rewrite is behavior-compatible with the TypeScript original for all valid inputs.
A few intentional divergences harden security and robustness:

- **Built-in commands run without a shell.** Internal `git` / `npm` / `ssh-keyscan`
  invocations execute as argument vectors, not through `/bin/sh -c`. The TypeScript version
  interpolated values (including project URLs read from `.gogo`) into a shell string, which
  allowed command injection / remote code execution when cloning an untrusted meta
  repository. `gogo exec` and `gogo run` still run shell commands — that is their purpose.
- **Project paths are validated.** Project keys in `.gogo` (and the `folder` argument of
  `gogo project create` / `gogo project import`) must be relative and stay within the
  repository; absolute paths and `..` traversal are rejected. The TypeScript version
  performed no such check, so a malicious `.gogo` could create or move directories outside
  the repository.
- **`gogo init` has no `-f` shorthand.** Use `--force`. (The global `-f` / `--file` overlay
  flag is unchanged.) The TypeScript CLI accepted `-f` for both, which the Go CLI framework
  cannot express without a flag collision.

## Installation

### Pre-built Binary (recommended)

Get the most recent [release from GitHub](https://github.com/daFish/gogo-meta/releases).

### Docker (not recommended)

```bash
docker pull ghcr.io/dafish/gogo-meta
```

When using Docker, mount your working directory and SSH keys so gogo can access your repositories:

```bash
docker run -it --rm \
  -v "$PWD":/workspace \
  -v "$HOME/.ssh":/root/.ssh:ro \
  -w /workspace \
  ghcr.io/dafish/gogo-meta <command>
```

Any `gogo` command shown in this README can be run via Docker by replacing `gogo` with the `docker run` call above. For convenience, you can create a shell alias:

```bash
alias gogo='docker run -it --rm -v "$PWD":/workspace -v "$HOME/.ssh":/root/.ssh:ro -w /workspace ghcr.io/dafish/gogo-meta'
```

### From source

```bash
git clone https://github.com/daFish/gogo-meta.git
cd gogo-meta
make build
```

The binary is built to `dist/gogo`. Add it to your `$PATH` or move it to a directory in your `$PATH`:

```bash
mv dist/gogo /usr/local/bin/
```

## Quick Start

```bash
# Initialize a new meta repository
gogo init

# Import existing repositories
gogo project import api git@github.com:org/api.git
gogo project import web git@github.com:org/web.git

# Clone a meta repository (includes all children)
gogo git clone git@github.com:org/meta-repo.git

# Run commands across all projects
gogo exec "npm install"
gogo exec "git status" --parallel
```

## Configuration

### .gogo

The config file defines child repositories, ignore patterns, and predefined commands. Both JSON and YAML formats are supported.

gogo looks for config files in the following order of precedence: `.gogo` (JSON) > `.gogo.yaml` > `.gogo.yml`. The first file found is used.

#### JSON format (.gogo)

```json
{
  "projects": {
    "api": "git@github.com:org/api.git",
    "web": "git@github.com:org/web.git",
    "libs/shared": "git@github.com:org/shared.git"
  },
  "ignore": [".git", "node_modules", ".vagrant", ".vscode"],
  "commands": {
    "build": "npm run build",
    "test": {
      "cmd": "npm test",
      "parallel": true,
      "description": "Run tests in all projects"
    },
    "deploy": {
      "cmd": "npm run deploy",
      "parallel": true,
      "concurrency": 2,
      "includeOnly": ["api", "web"]
    }
  }
}
```

#### YAML format (.gogo.yaml)

```yaml
# Main services
projects:
  api: git@github.com:org/api.git
  web: git@github.com:org/web.git
  libs/shared: git@github.com:org/shared.git

ignore:
  - .git
  - node_modules
  - .vagrant
  - .vscode

# Predefined commands
commands:
  build: npm run build
  test:
    cmd: npm test
    parallel: true
    description: Run tests in all projects
  deploy:
    cmd: npm run deploy
    parallel: true
    concurrency: 2
    includeOnly:
      - api
      - web
```

### Multiple Config Files

For large projects you can split configuration across multiple files and merge them at runtime using the `-f, --file` flag:

```bash
# Load primary .gogo plus additional projects from .gogo.devops
gogo -f .gogo.devops exec "npm test"

# Multiple overlays are applied in order
gogo -f .gogo.devops -f .gogo.extra git status
```

Overlay files follow the same format as the primary config (JSON or YAML). When merging:
- **Projects**: overlay entries are added; on key conflict the overlay wins
- **Ignore**: arrays are concatenated and deduplicated
- **Commands**: overlay entries are added; on key conflict the overlay wins

Overlay paths are resolved relative to the directory containing the primary config file.

Write commands (`project create`, `project import`) only modify the primary config file — overlay projects are never absorbed into it.

### Personal overlay: `.gogo.local`

A `.gogo.local` file sitting next to the primary config is **auto-merged on every normal command** (no `-f` needed) — think of it like `docker-compose.override.yml`. Its filename follows the primary's format: `.gogo.local` beside a JSON `.gogo`, `.gogo.local.yaml` beside a `.gogo.yaml`. Merge order is **primary → `.gogo.local` → `-f` overlays**.

Use it for personal additions: extra repos only you work with, or a shared template you'd rather not pass with `-f` every time. It is **not** merged by write commands (`project create`, `project import`, `git clone`), so your personal projects never leak into the shared `.gogo`.

#### Two-layer ignore model

The ignore destination mirrors the config layer, so personal choices never pollute the shared repo:

| What | Ignored via |
|------|-------------|
| The `.gogo.local` file itself | shared `.gitignore` (like `.env`; each dev keeps their own uncommitted) — added automatically by `gogo init`, `project import`, and `migrate` |
| Project dirs from the **shared** config (`.gogo`) | shared, tracked `.gitignore` |
| Project dirs from **`.gogo.local`** | `.git/info/exclude` — git's per-repo, uncommitted ignore; **never shared** |

`gogo git update` auto-adds local-only project directories to `.git/info/exclude`, so a personal repo cloned into the umbrella stays out of `git status` without touching the shared `.gitignore`.

> If a `.gogo.local.*` file exists in a format that doesn't match the primary (e.g. `.gogo.local.yaml` beside a JSON `.gogo`), gogo prints a warning that it will not be merged.

## Commands

### Global Options

These options are available for most commands:

| Option                      | Description                                              |
| --------------------------- | -------------------------------------------------------- |
| `-f, --file <path>`         | Additional config file to merge (repeatable)             |
| `--include-only <dirs>`     | Only target specified directories (comma-separated)      |
| `--exclude-only <dirs>`     | Exclude specified directories (comma-separated)          |
| `--include-pattern <regex>` | Include directories matching regex pattern               |
| `--exclude-pattern <regex>` | Exclude directories matching regex pattern               |
| `--parallel`                | Execute commands concurrently                            |
| `--concurrency <n>`         | Maximum parallel processes (default: 4)                  |

---

### `gogo init`

Initialize a new gogo-meta repository by creating a config file.

```bash
gogo init                  # Create .gogo (JSON, default)
gogo init --format yaml    # Create .gogo.yaml (YAML)
gogo init --force          # Overwrite existing config file
```

| Option              | Description                                    |
| ------------------- | ---------------------------------------------- |
| `-f, --force`       | Overwrite existing config file                 |
| `--format <format>` | Config file format: `json` (default) or `yaml` |

---

### `gogo exec <command>`

Execute an arbitrary command in all project directories.

```bash
# Run in each project sequentially
gogo exec "npm test"

# Run in parallel
gogo exec "npm install" --parallel

# Run with limited concurrency
gogo exec "npm run build" --parallel --concurrency 2

# Filter to specific projects
gogo exec "git status" --include-only api,web

# Exclude projects
gogo exec "npm install" --exclude-only docs

# Use regex patterns
gogo exec "npm test" --include-pattern "^libs/"
```

| Option                      | Description                          |
| --------------------------- | ------------------------------------ |
| `--include-only <dirs>`     | Only run in specified directories    |
| `--exclude-only <dirs>`     | Skip specified directories           |
| `--include-pattern <regex>` | Include directories matching pattern |
| `--exclude-pattern <regex>` | Exclude directories matching pattern |
| `--parallel`                | Run commands concurrently            |
| `--concurrency <n>`         | Max parallel processes               |

---

### `gogo run [name]`

Run a predefined command from the `.gogo` file.

```bash
# List available commands
gogo run
gogo run --list

# Run a predefined command
gogo run build

# Override config options with CLI flags
gogo run test --parallel
gogo run deploy --include-only api

# Commands can define defaults in .gogo:
# - parallel: true/false
# - concurrency: number
# - includeOnly/excludeOnly: array of directories
# - includePattern/excludePattern: regex patterns
# CLI flags override these config defaults
```

| Option                      | Description                                             |
| --------------------------- | ------------------------------------------------------- |
| `-l, --list`                | List all available commands                             |
| `--include-only <dirs>`     | Only run in specified directories (overrides config)    |
| `--exclude-only <dirs>`     | Skip specified directories (overrides config)           |
| `--include-pattern <regex>` | Include directories matching pattern (overrides config) |
| `--exclude-pattern <regex>` | Exclude directories matching pattern (overrides config) |
| `--parallel`                | Run commands concurrently (overrides config)            |
| `--concurrency <n>`         | Max parallel processes (overrides config)               |

---

### `gogo git clone <url>`

Clone a meta repository and all its child repositories.

```bash
# Clone to directory matching repo name
gogo git clone git@github.com:org/meta-repo.git

# Clone to custom directory
gogo git clone git@github.com:org/meta-repo.git -d my-project
```

| Option                  | Description           |
| ----------------------- | --------------------- |
| `-d, --directory <dir>` | Target directory name |

---

### `gogo git update`

Clone any child repositories defined in `.gogo` that don't exist locally.

```bash
gogo git update
gogo git update --parallel
gogo git update --include-only api,web
```

| Option                  | Description                    |
| ----------------------- | ------------------------------ |
| `--include-only <dirs>` | Only update specified projects |
| `--exclude-only <dirs>` | Skip specified projects        |
| `--parallel`            | Clone in parallel              |
| `--concurrency <n>`     | Max parallel clones            |

---

### `gogo git status`

Show git status across all repositories.

```bash
gogo git status
gogo git status --parallel
gogo git status --include-only api
```

| Option                  | Description                   |
| ----------------------- | ----------------------------- |
| `--include-only <dirs>` | Only check specified projects |
| `--exclude-only <dirs>` | Skip specified projects       |
| `--parallel`            | Run in parallel               |

---

### `gogo git pull`

Pull changes in all repositories.

```bash
gogo git pull
gogo git pull --parallel
```

| Option                  | Description                  |
| ----------------------- | ---------------------------- |
| `--include-only <dirs>` | Only pull specified projects |
| `--exclude-only <dirs>` | Skip specified projects      |
| `--parallel`            | Pull in parallel             |
| `--concurrency <n>`     | Max parallel pulls           |

---

### `gogo git push`

Push changes in all repositories.

```bash
gogo git push
gogo git push --include-only api,web
```

| Option                  | Description                  |
| ----------------------- | ---------------------------- |
| `--include-only <dirs>` | Only push specified projects |
| `--exclude-only <dirs>` | Skip specified projects      |
| `--parallel`            | Push in parallel             |

---

### `gogo git branch [name]`

List, create, or delete branches across all repositories.

```bash
# List branches
gogo git branch

# List all branches (including remote)
gogo git branch --all

# Create a new branch
gogo git branch feature/new-feature

# Delete a branch
gogo git branch feature/old-feature --delete
```

| Option                  | Description                          |
| ----------------------- | ------------------------------------ |
| `-d, --delete`          | Delete the specified branch          |
| `-a, --all`             | List all branches (local and remote) |
| `--include-only <dirs>` | Only target specified projects       |
| `--exclude-only <dirs>` | Skip specified projects              |
| `--parallel`            | Run in parallel                      |

---

### `gogo git checkout <branch>`

Checkout a branch in all repositories.

```bash
# Checkout existing branch
gogo git checkout main

# Create and checkout new branch
gogo git checkout -b feature/new-feature
```

| Option                  | Description                           |
| ----------------------- | ------------------------------------- |
| `-b, --create`          | Create the branch if it doesn't exist |
| `--include-only <dirs>` | Only target specified projects        |
| `--exclude-only <dirs>` | Skip specified projects               |
| `--parallel`            | Run in parallel                       |

---

### `gogo git commit`

Commit changes in all repositories with the same message.

```bash
gogo git commit -m "Update dependencies"
gogo git commit -m "Fix bug" --include-only api
```

| Option                  | Description                       |
| ----------------------- | --------------------------------- |
| `-m, --message <msg>`   | Commit message (required)         |
| `--include-only <dirs>` | Only commit in specified projects |
| `--exclude-only <dirs>` | Skip specified projects           |

---

### `gogo project create <folder> <url>`

Create and initialize a new child repository.

```bash
gogo project create libs/new-lib git@github.com:org/new-lib.git
```

This will:

1. Create the directory
2. Initialize git
3. Add the remote origin
4. Add the project to `.gogo`
5. Add the path to `.gitignore`

---

### `gogo project import <folder> [url]`

Import an existing repository as a child project.

```bash
# Clone and import a remote repository
gogo project import api git@github.com:org/api.git

# Import an existing local directory (reads remote from git)
gogo project import existing-folder

# Register without cloning (clone later with gogo git update)
gogo project import api git@github.com:org/api.git --no-clone
```

| Option       | Description                                 |
| ------------ | ------------------------------------------- |
| `--no-clone` | Register project in `.gogo` without cloning |

This will:

1. Clone the repository (if URL provided and directory doesn't exist, unless `--no-clone`)
2. Add the project to `.gogo`
3. Add the path to `.gitignore` (unless `--no-clone`)

---

### `gogo npm install`

Run `npm install` in all projects.

```bash
gogo npm install
gogo npm i  # Alias
gogo npm install --parallel
```

| Option                  | Description                        |
| ----------------------- | ---------------------------------- |
| `--include-only <dirs>` | Only install in specified projects |
| `--exclude-only <dirs>` | Skip specified projects            |
| `--parallel`            | Run in parallel                    |
| `--concurrency <n>`     | Max parallel installs              |

---

### `gogo npm ci`

Run `npm ci` in all projects (clean install from lockfile).

```bash
gogo npm ci
gogo npm ci --parallel
```

| Option                  | Description                    |
| ----------------------- | ------------------------------ |
| `--include-only <dirs>` | Only run in specified projects |
| `--exclude-only <dirs>` | Skip specified projects        |
| `--parallel`            | Run in parallel                |
| `--concurrency <n>`     | Max parallel processes         |

---

### `gogo npm link`

Create npm links between projects for local development.

```bash
# Create global npm links for all projects
gogo npm link

# Create symlinks between all interdependent projects
gogo npm link --all
```

| Option                  | Description                                              |
| ----------------------- | -------------------------------------------------------- |
| `--all`                 | Link all projects bidirectionally (symlink dependencies) |
| `--include-only <dirs>` | Only link specified projects                             |
| `--exclude-only <dirs>` | Skip specified projects                                  |

---

### `gogo npm run <script>`

Run an npm script in all projects.

```bash
# Run in all projects
gogo npm run build

# Run in parallel
gogo npm run test --parallel

# Only run if script exists
gogo npm run lint --if-present
```

| Option                  | Description                                   |
| ----------------------- | --------------------------------------------- |
| `--if-present`          | Only run if the script exists in package.json |
| `--include-only <dirs>` | Only run in specified projects                |
| `--exclude-only <dirs>` | Skip specified projects                       |
| `--parallel`            | Run in parallel                               |
| `--concurrency <n>`     | Max parallel processes                        |

---

### `gogo validate`

Validate all config files in the current directory.

```bash
gogo validate
```

Checks `.gogo`, `.gogo.yaml`, and `.gogo.yml` files for valid syntax and structure.

---

### `gogo migrate`

Move/rename working-copy directories to match the configuration.

```bash
gogo migrate           # Rename directories to match .gogo config paths
gogo migrate --dry-run # Show what would be moved without making any changes
```

| Option      | Description                                               |
| ----------- | --------------------------------------------------------- |
| `--dry-run` | Show what would be moved without changing anything        |

---

## Examples

### Setting Up a New Meta Repository

```bash
mkdir my-project && cd my-project
gogo init
gogo project import backend git@github.com:org/backend.git
gogo project import frontend git@github.com:org/frontend.git
gogo project import shared git@github.com:org/shared.git
gogo npm install --parallel
```

### Daily Development Workflow

```bash
# Start of day: pull all changes
gogo git pull --parallel

# Check status across all repos
gogo git status

# Create feature branch everywhere
gogo git checkout -b feature/my-feature

# Run tests
gogo npm run test --parallel

# Commit changes
gogo git commit -m "Add feature"

# Push changes
gogo git push
```

### Working with Specific Projects

```bash
# Only work with API and shared libs
gogo exec "npm test" --include-only api,libs/shared

# Exclude documentation from builds
gogo npm run build --exclude-only docs

# Target all libs
gogo git status --include-pattern "^libs/"
```

## Development

### Prerequisites

- Go 1.24 or higher
- Git

### Build Commands

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

### Project Structure

```
./
├── cmd/gogo/          # Entry point
├── internal/
│   ├── cli/           # Cobra command definitions
│   ├── config/        # Config parsing, merging, validation
│   ├── executor/      # Shell command execution
│   ├── filter/        # Include/exclude filtering
│   ├── loop/          # Multi-repo orchestration
│   ├── output/        # Terminal formatting
│   └── ssh/           # SSH host key management
├── Makefile
├── .golangci.yml
└── go.mod
```

## Requirements

- Go 1.24 or higher
- Git

## License

MIT
