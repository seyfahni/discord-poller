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

The sample files in [`docs/examples/`](docs/examples/) contain the supported
configuration fields. JSON Schemas for editor validation are in
[`schemas/`](schemas/). Replace the webhook placeholder in the user config
with a real URL and keep that file secret. Install it owned by root and
readable by root and the service account only (for example, mode `0640` and
group `discord-poller`).

Install the binary and systemd units:

```sh
sudo install -o root -g root -m 0755 /tmp/discord-poller /usr/local/bin/discord-poller
sudo useradd --system --no-create-home --shell /usr/sbin/nologin discord-poller
sudo install -d -o root -g discord-poller -m 0750 /etc/discord-poller
sudo install -o root -g discord-poller -m 0640 docs/examples/user-config.yaml /etc/discord-poller/user-config.yaml
sudo install -o root -g discord-poller -m 0640 docs/examples/system-config.yaml /etc/discord-poller/system-config.yaml
sudo install -o root -g root -m 0644 deployments/discord-poller.service /etc/systemd/system/discord-poller.service
sudo install -o root -g root -m 0644 deployments/discord-poller.timer /etc/systemd/system/discord-poller.timer
sudo systemctl daemon-reload
```

Both configuration paths are required. The user configuration defines poll
content and target timing; the system configuration defines logging and HTTP
timeouts/retry limits. The service runs as the unprivileged `discord-poller`
account. Replace the installed sample URL before enabling the timer.

Start a one-off run or enable the weekly Thursday 18:10 Europe/Berlin timer
(missed runs are caught up once):

```sh
sudo systemctl start discord-poller.service
sudo systemctl enable --now discord-poller.timer
systemctl list-timers discord-poller.timer
sudo journalctl -u discord-poller.service
```

The units are in [`deployments/`](deployments/). The service can also be run
directly with separate user and system configuration paths:

```sh
/usr/local/bin/discord-poller \
  --config /etc/discord-poller/user-config.yaml \
  --system-config /etc/discord-poller/system-config.yaml
```

`--dry-run` loads and validates configuration and the rendered poll, then
performs a webhook-validation GET. It requires the webhook credential and
network access but never sends the poll-creation POST:

```sh
/tmp/discord-poller \
  --config /etc/discord-poller/user-config.yaml \
  --system-config /etc/discord-poller/system-config.yaml \
  --dry-run
```

Use `--help` for the available flags. Exit codes are:

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Internal error |
| 2 | Usage or configuration error |
| 3 | Transport failure or ambiguous delivery outcome |
| 4 | Discord API rejection or exhausted rate-limit retries |

For troubleshooting, check configuration paths/permissions and timezone names
for exit code 2; verify the webhook URL and token if Discord rejects the
request (code 4); and check DNS, outbound HTTPS connectivity, and service
logs for transport failures (code 3). A transport failure after a POST may
have created the poll; do not blindly rerun until checking Discord.

CI runs tests, vet, build, module checks, and validates the YAML examples
against the JSON Schemas.

## Project structure

- `internal/config`: user and system configuration models, validation, and
  strict YAML loaders
- `internal/schedule`: timezone-aware deadline selection and duration math
- `internal/template`: `{end_date}` substitution and brace escaping
- `internal/discord`: native poll payload validation and webhook HTTP client
- `cmd/discord-poller`: CLI composition and process exit behavior
- `internal/logger`: zap logger setup
- `deployments/`: systemd service and weekly timer
- `schemas/`: JSON Schema files for both configurations
- `docs/`: documentation and concepts
