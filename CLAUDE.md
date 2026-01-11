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
gogo git status|pull|push|branch|checkout|commit
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

## Distribution

Package is configured for GitLab Package Registry. Update `PROJECT_ID` in:
- `package.json` (publishConfig)
- `.npmrc`
- `.gitlab-ci.yml` (automatically uses CI variables)
