# Discord Poller

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
