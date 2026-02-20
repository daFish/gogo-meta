# gogo-meta

A modern TypeScript CLI for managing multi-repository projects. Reimplementation of [meta](https://github.com/mateodelnorte/meta).

## Project Overview

gogo-meta allows developers to manage multiple git repositories as a unified system. It executes commands across all child repositories defined in a `.gogo` configuration file.

## Tech Stack

- **Runtime**: Bun 1.x
- **Language**: TypeScript 5.x (strict mode, ESM)
- **CLI Framework**: Commander.js
- **Validation**: Zod 4.x
- **Terminal Styling**: picocolors
- **Build**: tsup
- **Testing**: Vitest 4.x + memfs

## Project Structure

```
src/
├── cli.ts                 # Entry point, command registration
├── commands/
│   ├── init.ts            # gogo init
│   ├── exec.ts            # gogo exec
│   ├── run.ts             # gogo run (predefined commands)
│   ├── git/               # Git subcommands (clone, update, status, etc.)
│   ├── project/           # Project management (create, import)
│   └── npm/               # NPM operations (install, link, run)
├── core/
│   ├── config.ts          # .gogo file parsing and manipulation
│   ├── executor.ts        # Shell command execution
│   ├── filter.ts          # Include/exclude filtering logic
│   ├── loop.ts            # Multi-repo command orchestration
│   └── output.ts          # Terminal output formatting
└── types/
    └── index.ts           # TypeScript types and Zod schemas
```

## Commands

```bash
bun install         # Install dependencies
bun run build       # Build to dist/
bun run dev         # Build in watch mode
bun run test:unit   # Run unit tests
bun run test:integration  # Run integration tests
bun run test:coverage  # Run tests with coverage
bun run typecheck   # Type check without emitting
```

## CLI Usage

```bash
gogo init                          # Create .gogo file
gogo exec "<command>" [--parallel] # Run command across repos
gogo run [name]                    # Run predefined command from .gogo
gogo git clone <url>               # Clone meta + children
gogo git update                    # Clone missing repos
gogo git status|diff|log|fetch|pull|push|branch|checkout|commit|merge|stash|tag|reset
gogo project create|import
gogo npm install|ci|link|run
```

## Code Conventions

- Use `node:` prefix for Node.js built-in imports
- Prefer async/await over callbacks
- Use Zod for runtime validation of external data
- Keep functions pure where possible
- Use picocolors for terminal styling (not chalk)

## Testing Patterns

- Unit tests use memfs to mock filesystem
- Mock `src/core/executor.ts` to avoid real shell commands
- Mock `src/core/output.ts` to suppress console output
- Integration tests verify command behavior end-to-end

## Configuration Files

### .gogo (project config)

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

### .looprc (optional filtering)

```json
{
  "ignore": ["docs", "examples"]
}
```

## MCP Server

gogo-meta includes an MCP server (`src/mcp.ts`) that exposes all core functionality as structured tools for AI agents.

### Using gogo MCP tools in projects

**When a `.gogo` file is present, always prefer gogo MCP tools over manual bash git commands.** This applies to any repository managed by gogo-meta.

| Instead of | Use MCP tool |
|---|---|
| `git status` (per repo) | `gogo_git_status` |
| `git diff` (per repo) | `gogo_git_diff` (supports `--cached`, `--stat`, `--name-only`) |
| `git log` (per repo) | `gogo_git_log` (supports `--oneline`, `-n`, `--since`) |
| `git pull` (per repo) | `gogo_git_pull` |
| `git push` (per repo) | `gogo_git_push` (supports `--force-with-lease`, `--tags`) |
| `git fetch` (per repo) | `gogo_git_fetch` (supports `--all`, `--prune`) |
| `git branch` (per repo) | `gogo_git_branch` |
| `git checkout` (per repo) | `gogo_git_checkout` |
| `git commit` (per repo) | `gogo_git_commit` (supports `--fixup`, `--amend`, `-a`) |
| `git merge` (per repo) | `gogo_git_merge` (supports `--no-ff`, `--squash`, `--abort`) |
| `git stash` (per repo) | `gogo_git_stash` (push/pop/list/drop/show) |
| `git tag` (per repo) | `gogo_git_tag` (create/delete/list) |
| `git reset` (per repo) | `gogo_git_reset` (supports `--soft`, `--hard`) |
| Running a shell command in each repo | `gogo_exec` |
| Reading `.gogo` config manually | `gogo_config` |
| Listing child repositories | `gogo_projects` |

The MCP tools operate across **all** child repositories in a single call and return structured JSON with per-project results (exit code, stdout, stderr, duration). Use `includeOnly` / `excludeOnly` parameters to filter to specific repos.
