# gogo-meta — Project Overview

## Purpose
A modern TypeScript CLI for managing multi-repository projects. Reimplementation of [meta](https://github.com/mateodelnorte/meta).
Allows developers to manage multiple git repositories as a unified system by executing commands across all child repositories defined in a `.gogo` configuration file.

Published as `@dafish/gogo-meta` on npm. Binary name: `gogo`.

## Tech Stack
- **Runtime**: Bun 1.x (Node 24 target for build)
- **Language**: TypeScript 5.x (strict mode, ESM only)
- **CLI Framework**: Commander.js 14.x
- **Validation**: Zod 4.x (for runtime validation of external data like config files)
- **Terminal Styling**: picocolors (not chalk)
- **Build**: tsup (ESM format, with dts and sourcemaps)
- **Testing**: Vitest 4.x + memfs (for filesystem mocking)
- **Linting**: ESLint 9.x + typescript-eslint
- **Release**: semantic-release

## Project Structure
```
src/
├── cli.ts                     # Entry point, command registration (createProgram, getVersion, main)
├── index.ts                   # Library exports
├── commands/
│   ├── init.ts                # gogo init
│   ├── exec.ts                # gogo exec
│   ├── run.ts                 # gogo run (predefined commands)
│   ├── git/                   # Git subcommands (clone, update, status, pull, push, branch, checkout, commit)
│   ├── project/               # Project management (create, import)
│   └── npm/                   # NPM operations (install, link, run)
├── core/
│   ├── config.ts              # .gogo file parsing, manipulation (readMetaConfig, writeMetaConfig, addProject, etc.)
│   ├── executor.ts            # Shell command execution
│   ├── filter.ts              # Include/exclude filtering logic
│   ├── loop.ts                # Multi-repo command orchestration
│   ├── output.ts              # Terminal output formatting
│   ├── ssh.ts                 # SSH host key verification
│   └── index.ts               # Core module barrel export
└── types/
    └── index.ts               # TypeScript types and Zod schemas (MetaConfig, LoopRc, ExecutorResult, etc.)

tests/
├── setup.ts                   # Global test setup (restores mocks, resets exitCode)
├── unit/core/                 # Unit tests for core modules
├── integration/               # Integration tests (exec, init, run, git, project)
└── e2e/                       # End-to-end tests

bin/
└── gogo                       # CLI entry script
```

## Docker
- Multi-stage Dockerfile: Bun build stage + Node 24 Alpine runtime
- Runtime image includes git, git-lfs, openssh-client
- Published to `ghcr.io/dafish/gogo-meta` on release (via `.github/workflows/docker.yml`)
- Triggered by GitHub release event (created by semantic-release)

## Configuration Files
- `.gogo` — Main project config (projects map, ignore patterns, commands)
- `.looprc` — Optional filtering config (ignore list)
