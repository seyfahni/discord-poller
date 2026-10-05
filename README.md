# Discord Poller

Discord Poller is a Go project for creating a configurable native Discord poll
through an incoming webhook. The current implementation provides strict YAML
configuration loading and validation, deadline selection, poll-duration
calculation, template rendering for question and answer text, and JSON logging.
Poll creation, CLI, and deployment are documented plans and are not implemented
yet.

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

- `internal/config`: user and system configuration models, validation, and
  strict YAML loaders
- `internal/schedule`: timezone-aware deadline selection and duration math
- `internal/template`: `{end_date}` substitution and brace escaping
- `internal/logger`: zap logger setup
- `docs/`: documentation and concepts
