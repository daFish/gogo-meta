# gogo-meta

A modern TypeScript CLI for managing multi-repository projects. Execute commands across multiple git repositories simultaneously.

## Features

- Clone entire project ecosystems with one command
- Execute arbitrary commands across all repositories
- Parallel or sequential execution modes
- Flexible filtering (include/exclude by name or pattern)
- NPM operations across all projects
- Symlink projects for local development

## Installation

### From GitLab Package Registry

```bash
# Configure registry (replace PROJECT_ID with your GitLab project ID)
npm config set @gogo-meta:registry https://gitlab.com/api/v4/projects/PROJECT_ID/packages/npm/
npm config set '//gitlab.com/api/v4/projects/PROJECT_ID/packages/npm/:_authToken' "${GITLAB_TOKEN}"

# Install globally
npm install -g @gogo-meta/cli
```

### From Source

```bash
git clone <repository-url>
cd gogo-meta
pnpm install
pnpm build
npm link
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

The `.gogo` file defines child repositories, ignore patterns, and predefined commands:

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

### .looprc (optional)

Define default ignore patterns for command execution:

```json
{
  "ignore": ["docs", "examples"]
}
```

## Commands

### Global Options

These options are available for most commands:

| Option | Description |
|--------|-------------|
| `--include-only <dirs>` | Only target specified directories (comma-separated) |
| `--exclude-only <dirs>` | Exclude specified directories (comma-separated) |
| `--include-pattern <regex>` | Include directories matching regex pattern |
| `--exclude-pattern <regex>` | Exclude directories matching regex pattern |
| `--parallel` | Execute commands concurrently |
| `--concurrency <n>` | Maximum parallel processes (default: 4) |

---

### `gogo init`

Initialize a new gogo-meta repository by creating a `.gogo` file.

```bash
gogo init
gogo init --force  # Overwrite existing .gogo file
```

| Option | Description |
|--------|-------------|
| `-f, --force` | Overwrite existing `.gogo` file |

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

| Option | Description |
|--------|-------------|
| `--include-only <dirs>` | Only run in specified directories |
| `--exclude-only <dirs>` | Skip specified directories |
| `--include-pattern <regex>` | Include directories matching pattern |
| `--exclude-pattern <regex>` | Exclude directories matching pattern |
| `--parallel` | Run commands concurrently |
| `--concurrency <n>` | Max parallel processes |

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

| Option | Description |
|--------|-------------|
| `-l, --list` | List all available commands |
| `--include-only <dirs>` | Only run in specified directories (overrides config) |
| `--exclude-only <dirs>` | Skip specified directories (overrides config) |
| `--include-pattern <regex>` | Include directories matching pattern (overrides config) |
| `--exclude-pattern <regex>` | Exclude directories matching pattern (overrides config) |
| `--parallel` | Run commands concurrently (overrides config) |
| `--concurrency <n>` | Max parallel processes (overrides config) |

---

### `gogo git clone <url>`

Clone a meta repository and all its child repositories.

```bash
# Clone to directory matching repo name
gogo git clone git@github.com:org/meta-repo.git

# Clone to custom directory
gogo git clone git@github.com:org/meta-repo.git -d my-project
```

| Option | Description |
|--------|-------------|
| `-d, --directory <dir>` | Target directory name |

---

### `gogo git update`

Clone any child repositories defined in `.gogo` that don't exist locally.

```bash
gogo git update
gogo git update --parallel
gogo git update --include-only api,web
```

| Option | Description |
|--------|-------------|
| `--include-only <dirs>` | Only update specified projects |
| `--exclude-only <dirs>` | Skip specified projects |
| `--parallel` | Clone in parallel |
| `--concurrency <n>` | Max parallel clones |

---

### `gogo git status`

Show git status across all repositories.

```bash
gogo git status
gogo git status --parallel
gogo git status --include-only api
```

| Option | Description |
|--------|-------------|
| `--include-only <dirs>` | Only check specified projects |
| `--exclude-only <dirs>` | Skip specified projects |
| `--parallel` | Run in parallel |

---

### `gogo git pull`

Pull changes in all repositories.

```bash
gogo git pull
gogo git pull --parallel
```

| Option | Description |
|--------|-------------|
| `--include-only <dirs>` | Only pull specified projects |
| `--exclude-only <dirs>` | Skip specified projects |
| `--parallel` | Pull in parallel |
| `--concurrency <n>` | Max parallel pulls |

---

### `gogo git push`

Push changes in all repositories.

```bash
gogo git push
gogo git push --include-only api,web
```

| Option | Description |
|--------|-------------|
| `--include-only <dirs>` | Only push specified projects |
| `--exclude-only <dirs>` | Skip specified projects |
| `--parallel` | Push in parallel |

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

| Option | Description |
|--------|-------------|
| `-d, --delete` | Delete the specified branch |
| `-a, --all` | List all branches (local and remote) |
| `--include-only <dirs>` | Only target specified projects |
| `--exclude-only <dirs>` | Skip specified projects |
| `--parallel` | Run in parallel |

---

### `gogo git checkout <branch>`

Checkout a branch in all repositories.

```bash
# Checkout existing branch
gogo git checkout main

# Create and checkout new branch
gogo git checkout -b feature/new-feature
```

| Option | Description |
|--------|-------------|
| `-b, --create` | Create the branch if it doesn't exist |
| `--include-only <dirs>` | Only target specified projects |
| `--exclude-only <dirs>` | Skip specified projects |
| `--parallel` | Run in parallel |

---

### `gogo git commit`

Commit changes in all repositories with the same message.

```bash
gogo git commit -m "Update dependencies"
gogo git commit -m "Fix bug" --include-only api
```

| Option | Description |
|--------|-------------|
| `-m, --message <msg>` | Commit message (required) |
| `--include-only <dirs>` | Only commit in specified projects |
| `--exclude-only <dirs>` | Skip specified projects |

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
```

This will:
1. Clone the repository (if URL provided and directory doesn't exist)
2. Add the project to `.gogo`
3. Add the path to `.gitignore`

---

### `gogo npm install`

Run `npm install` in all projects.

```bash
gogo npm install
gogo npm i  # Alias
gogo npm install --parallel
```

| Option | Description |
|--------|-------------|
| `--include-only <dirs>` | Only install in specified projects |
| `--exclude-only <dirs>` | Skip specified projects |
| `--parallel` | Run in parallel |
| `--concurrency <n>` | Max parallel installs |

---

### `gogo npm ci`

Run `npm ci` in all projects (clean install from lockfile).

```bash
gogo npm ci
gogo npm ci --parallel
```

| Option | Description |
|--------|-------------|
| `--include-only <dirs>` | Only run in specified projects |
| `--exclude-only <dirs>` | Skip specified projects |
| `--parallel` | Run in parallel |
| `--concurrency <n>` | Max parallel processes |

---

### `gogo npm link`

Create npm links between projects for local development.

```bash
# Create global npm links for all projects
gogo npm link

# Create symlinks between all interdependent projects
gogo npm link --all
```

| Option | Description |
|--------|-------------|
| `--all` | Link all projects bidirectionally (symlink dependencies) |
| `--include-only <dirs>` | Only link specified projects |
| `--exclude-only <dirs>` | Skip specified projects |

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

| Option | Description |
|--------|-------------|
| `--if-present` | Only run if the script exists in package.json |
| `--include-only <dirs>` | Only run in specified projects |
| `--exclude-only <dirs>` | Skip specified projects |
| `--parallel` | Run in parallel |
| `--concurrency <n>` | Max parallel processes |

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

## Requirements

- Node.js 22 or higher
- Git

## License

MIT
