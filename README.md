# Discord Poller

Discord Poller is a Go project for creating a configurable native Discord poll
through an incoming webhook. The current implementation provides strict YAML
configuration loading and validation, deadline selection, poll-duration
calculation, template rendering for question and answer text, and Discord
webhook validation and poll creation through a one-shot CLI.

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

The `./...` commands cover all packages currently in the repository.

Build the CLI outside the working tree:

```sh
go build -o /tmp/discord-poller ./cmd/discord-poller
```

Run it with separate user and system configuration files:

```sh
/tmp/discord-poller \
  --config /etc/discord-poller/user-config.yaml \
  --system-config /etc/discord-poller/system-config.yaml
```

Both configuration paths are required. The user configuration defines poll
content and target timing; the system configuration defines logging and HTTP
timeouts/retry limits. The webhook URL is stored in the user configuration;
that file contains a credential, so restrict its permissions (for example,
`0600`) and never commit it. See [`docs/examples/user-config.yaml`](docs/examples/user-config.yaml)
for the `webhook_url` field.

`--dry-run` loads and validates configuration and the rendered poll, then
performs a webhook-validation GET. It requires the webhook credential and
network access but never sends the poll-creation POST:

```sh
/tmp/discord-poller \
  --config /etc/discord-poller/user-config.yaml \
  --system-config /etc/discord-poller/system-config.yaml \
  --dry-run
```

Use `--help` for the available flags. Exit codes are 0 for success, 1 for an
internal error, 2 for usage or configuration errors, 3 for transport or
ambiguous delivery outcomes, and 4 for Discord API rejection or exhausted
rate-limit retries. Deployment files are part of the next planned
implementation phase.

## Project structure

- `internal/config`: user and system configuration models, validation, and
  strict YAML loaders
- `internal/schedule`: timezone-aware deadline selection and duration math
- `internal/template`: `{end_date}` substitution and brace escaping
- `internal/discord`: native poll payload validation and webhook HTTP client
- `cmd/discord-poller`: CLI composition and process exit behavior
- `internal/logger`: zap logger setup
- `docs/`: documentation and concepts
