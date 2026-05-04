# Suggested Commands

Prefer `make` targets over invoking the underlying tools directly.

## Make Targets (full list, see `Makefile`)

| Target               | Description                                                |
| -------------------- | ---------------------------------------------------------- |
| `make help`          | Show all available targets (auto-generated from `## ...`)  |
| `make build`         | Build the `gogo` binary to `dist/gogo` (CGO disabled, `-trimpath`, ldflags inject version) |
| `make docker`        | Build the gogo container image locally via `Dockerfile.local` (tag `ghcr.io/dafish/gogo-meta:latest`) |
| `make fmt`           | Run `go fmt ./...`                                         |
| `make lint`          | Run `golangci-lint run`                                    |
| `make test`          | Run all tests (`go test ./...`)                            |
| `make test-coverage` | Run tests with coverage profile + HTML report under `coverage/` |
| `make clean`         | Remove `dist/` and `coverage/`                             |
| `make all`           | Clean → lint → test-coverage → build                       |

## Module / Dependencies

| Command          | Description                              |
| ---------------- | ---------------------------------------- |
| `go mod tidy`    | Sync `go.mod` / `go.sum`                 |
| `go mod download`| Pre-fetch dependencies                   |

## Direct Tooling (when bypassing make is needed)

| Command                | Description                          |
| ---------------------- | ------------------------------------ |
| `go test ./...`        | All tests                            |
| `go test ./internal/loop -run TestX -v` | Single package / single test |
| `go vet ./...`         | Vet all packages                     |
| `golangci-lint run`    | Lint directly                        |

## Release

| Command                                  | Description                  |
| ---------------------------------------- | ---------------------------- |
| `goreleaser release --snapshot --clean`  | Local snapshot release build |

(There is no `make` target for goreleaser; release is driven by CI via `.goreleaser.yaml`.)

## System Utilities (macOS/Darwin)

| Command                    | Description                |
| -------------------------- | -------------------------- |
| `git`                      | Version control            |
| `ls`, `cd`, `find`, `grep` | Standard unix utilities    |
| `go`                       | Go toolchain               |
| `golangci-lint`            | Linter (installed locally) |
