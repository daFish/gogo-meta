# Suggested Commands

## Development

| Command         | Description               |
| --------------- | ------------------------- |
| `bun install`   | Install dependencies      |
| `bun run build` | Build to dist/ (via tsup) |
| `bun run dev`   | Build in watch mode       |

## Testing

| Command                    | Description                |
| -------------------------- | -------------------------- |
| `bun run test`             | Run all tests (vitest run) |
| `bun run test:unit`        | Run unit tests only        |
| `bun run test:integration` | Run integration tests only |
| `bun run test:e2e`         | Run e2e tests only         |
| `bun run test:watch`       | Run tests in watch mode    |
| `bun run test:coverage`    | Run tests with V8 coverage |

## Code Quality

| Command             | Description                             |
| ------------------- | --------------------------------------- |
| `bun run lint`      | Run ESLint on src and tests             |
| `bun run typecheck` | TypeScript type checking (tsc --noEmit) |

## Release

| Command                   | Description              |
| ------------------------- | ------------------------ |
| `bun run release:dry-run` | Dry-run semantic release |

## Docker

| Command                                                                                                    | Description                    |
| ---------------------------------------------------------------------------------------------------------- | ------------------------------ |
| `docker build -t gogo-meta .`                                                                              | Build the Docker image locally |
| `docker run -it --rm -v "$PWD":/workspace -v "$HOME/.ssh":/root/.ssh:ro -w /workspace gogo-meta <command>` | Run gogo via Docker            |

## System Utilities (macOS/Darwin)

| Command                    | Description                            |
| -------------------------- | -------------------------------------- |
| `git`                      | Version control                        |
| `ls`, `cd`, `find`, `grep` | Standard unix utilities                |
| `bun`                      | JavaScript runtime and package manager |
