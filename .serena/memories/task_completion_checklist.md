# Task Completion Checklist

When a coding task is completed, run the following checks:

1. **Format**: `make fmt`
   - Runs `go fmt ./...`

2. **Lint**: `make lint`
   - Runs `golangci-lint` with the config in `.golangci.yml`

3. **Tests**: `make test`
   - Full test suite across all packages (`go test ./...`)

4. **Coverage** (when relevant): `make test-coverage`
   - Generates coverage profile + HTML report under `coverage/`

## Quick Check (minimum before committing)

```bash
make fmt && make lint && make test
```

## Full Pipeline

```bash
make all   # clean → lint → test-coverage → build
```

## Build Sanity

If a change touches `cmd/gogo` or affects build flags, also run:

```bash
make build && ./dist/gogo --version
```

Note: there is no `make vet` target. To run `go vet` directly: `go vet ./...`.
