# Discord Poller

Discord Poller is a Go project for creating a configurable native Discord poll
through an incoming webhook. The current implementation provides strict YAML
configuration loading and validation, plus JSON logging. Scheduling, poll
creation, and deployment are documented plans and are not implemented yet.

## Requirements

- Go 1.27 or newer

## Development commands

Run from the repository root:

```sh
# Run all tests
go test ./...

# Run tests with the race detector
go test -race ./...

# Check for common Go issues
go vet ./...

# Build all current packages
go build ./...

# Format changed Go files
gofmt -w path/to/changed.go

# Reconcile dependencies after changing imports or module requirements
go mod tidy

# Verify downloaded module checksums
go mod verify
```

The `./...` commands cover all packages currently in the repository. The
executable and its invocation will be documented when CLI support is
implemented.

## Project structure

- `internal/`: non-API code
- `docs/`: documentation and concepts
