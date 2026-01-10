# gogo-meta

A modern TypeScript CLI for managing multi-repository projects. Reimplementation of [meta](https://github.com/mateodelnorte/meta).

## Project Overview

gogo-meta allows developers to manage multiple git repositories as a unified system. It executes commands across all child repositories defined in a `.meta` configuration file.

## Tech Stack

- **Runtime**: Node.js 22+
- **Language**: TypeScript 5.x (strict mode, ESM)
- **CLI Framework**: Commander.js
- **Validation**: Zod
- **Terminal Styling**: picocolors
- **Build**: tsup
- **Testing**: Vitest + memfs

## Project Structure

```
src/
├── cli.ts                 # Entry point, command registration
├── commands/
│   ├── init.ts            # gogo init
│   ├── exec.ts            # gogo exec
│   ├── git/               # Git subcommands (clone, update, status, etc.)
│   ├── project/           # Project management (create, import)
│   └── npm/               # NPM operations (install, link, run)
├── core/
│   ├── config.ts          # .meta file parsing and manipulation
│   ├── executor.ts        # Shell command execution
│   ├── filter.ts          # Include/exclude filtering logic
│   ├── loop.ts            # Multi-repo command orchestration
│   └── output.ts          # Terminal output formatting
└── types/
    └── index.ts           # TypeScript types and Zod schemas
```

## Commands

```bash
pnpm install        # Install dependencies
pnpm build          # Build to dist/
pnpm dev            # Build in watch mode
pnpm test:unit      # Run unit tests
pnpm test:integration  # Run integration tests
pnpm test:coverage  # Run tests with coverage
pnpm typecheck      # Type check without emitting
```

## CLI Usage

```bash
gogo init                          # Create .meta file
gogo exec "<command>" [--parallel] # Run command across repos
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

### .meta (project config)
```json
{
  "projects": {
    "path/to/repo": "git@github.com:org/repo.git"
  },
  "ignore": [".git", "node_modules"]
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
