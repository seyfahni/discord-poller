# Discord Poller

A hosted Discord bot for coordinating attendance at recurring tabletop roleplaying sessions.

## Run

Requires Go 1.27+ and MySQL 8.0+. The service reads these environment variables:

- `DISCORD_TOKEN` — bot token
- `DISCORD_APP_ID` — Discord application ID
- `MYSQL_DSN` — MySQL driver DSN, for example `poller:password@tcp(localhost:3306)/discord_poller`

Run locally with `go run ./cmd/discord-poller`, or build and start the container:

```sh
docker build -t discord-poller .
docker run --rm \
  -e DISCORD_TOKEN \
  -e DISCORD_APP_ID \
  -e MYSQL_DSN \
  discord-poller
```

The database must exist before startup; the service creates its required tables if they do not already exist. Keep credentials outside the repository.

## Discord setup

Invite the bot with the `bot` and `applications.commands` OAuth2 scopes and grant **View Channels**, **Manage Channels**, **Send Messages**, **Send Polls**, and **Use Application Commands**. Enable the privileged **Message Content** intent in the Discord Developer Portal. The service also uses the Guilds and Guild Message Polls gateway intents.

## Development

Run from the repository root:

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./...
go mod tidy
go mod verify
```

Set `MYSQL_TEST_DSN` to an isolated MySQL database to run the persistence-across-restarts integration test.
