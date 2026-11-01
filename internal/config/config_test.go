package config_test

import (
	"testing"

	"github.com/seyfahni/discord-poller/internal/config"
)

func TestLoad_requiresDiscordAndMySQLConfiguration(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "")
	t.Setenv("DISCORD_APP_ID", "")
	t.Setenv("MYSQL_DSN", "")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() error = nil, want missing configuration error")
	}
}

func TestLoad_readsRuntimeConfiguration(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "test-token")
	t.Setenv("DISCORD_APP_ID", "123456789012345678")
	t.Setenv("MYSQL_DSN", "user:password@tcp(localhost:3306)/poller")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.DiscordToken != "test-token" || got.DiscordAppID != "123456789012345678" ||
		got.MySQLDSN != "user:password@tcp(localhost:3306)/poller" {
		t.Fatalf("Load() = %#v, want the configured values", got)
	}
}
