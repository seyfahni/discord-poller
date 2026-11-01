// Package config loads the service's runtime configuration.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config contains the required service configuration.
type Config struct {
	DiscordToken string
	DiscordAppID string
	MySQLDSN     string
}

// Load reads and validates the required environment variables.
func Load() (Config, error) {
	values := []struct {
		name  string
		value string
	}{
		{name: "DISCORD_TOKEN", value: os.Getenv("DISCORD_TOKEN")},
		{name: "DISCORD_APP_ID", value: os.Getenv("DISCORD_APP_ID")},
		{name: "MYSQL_DSN", value: os.Getenv("MYSQL_DSN")},
	}

	for _, value := range values {
		if strings.TrimSpace(value.value) == "" {
			return Config{}, fmt.Errorf("missing required environment variable %s", value.name)
		}
	}

	return Config{
		DiscordToken: values[0].value,
		DiscordAppID: values[1].value,
		MySQLDSN:     values[2].value,
	}, nil
}
