package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadUserConfigExample(t *testing.T) {
	cfg, err := LoadUserConfig(filepath.Join("..", "..", "docs", "examples", "user-config.yaml"))
	require.NoError(t, err)
	want := validUserConfig()
	want.Target.SameDayThreshold = 3 * time.Hour
	want.Poll.Answers = []AnswerConfig{
		{Text: "I'm in", Emoji: "✅"},
		{Text: "Can't make it", Emoji: "❌"},
		{Text: "Maybe"},
	}
	assert.Equal(t, &want, cfg)
}

func TestLoadSystemConfigExample(t *testing.T) {
	cfg, err := LoadSystemConfig(filepath.Join("..", "..", "docs", "examples", "system-config.yaml"))
	require.NoError(t, err)
	want := validSystemConfig()
	assert.Equal(t, &want, cfg)
}

func TestLoadUserConfigErrors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "examples", "user-config.yaml"))
	require.NoError(t, err)
	valid := string(data)

	for _, tt := range []struct {
		name        string
		yaml        string
		errContains string
	}{
		{name: "unknown root key", yaml: valid + "\nunknown: true\n", errContains: "failed to parse user config"},
		{name: "unknown target key", yaml: strings.Replace(valid, "timezone:", "timezoneX:", 1), errContains: "field timezoneX not found"},
		{name: "unknown poll key", yaml: strings.Replace(valid, "answers:", "answersX:", 1), errContains: "field answersX not found"},
		{name: "unknown answer key", yaml: strings.Replace(valid, "emoji:", "emojiX:", 1), errContains: "field emojiX not found"},
		{name: "malformed YAML", yaml: "poll: [", errContains: "failed to parse user config"},
		{name: "invalid duration", yaml: strings.Replace(valid, "3h", "not-a-duration", 1), errContains: "failed to parse user config"},
		{name: "empty file", yaml: "", errContains: "failed to parse user config"},
		{name: "missing required fields", yaml: "{}", errContains: "user config validation failed: webhook_url_env"},
		{name: "invalid threshold", yaml: strings.Replace(valid, "3h", "30m", 1), errContains: "user config validation failed: target.same_day_threshold"},
		{name: "too many answers", yaml: valid + strings.Repeat("    - text: Extra\n", 8), errContains: "user config validation failed: poll.answers"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "user-config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.yaml), 0600))
			cfg, err := LoadUserConfig(path)
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.NotNil(t, errors.Unwrap(err))
		})
	}
}

func TestLoadSystemConfigErrors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "examples", "system-config.yaml"))
	require.NoError(t, err)
	valid := string(data)

	for _, tt := range []struct {
		name        string
		yaml        string
		errContains string
	}{
		{name: "unknown root key", yaml: valid + "\nunknown: true\n", errContains: "failed to parse system config"},
		{name: "unknown logging key", yaml: strings.Replace(valid, "level:", "levelX:", 1), errContains: "field levelX not found"},
		{name: "unknown HTTP key", yaml: strings.Replace(valid, "max_retries:", "max_retriesX:", 1), errContains: "field max_retriesX not found"},
		{name: "malformed YAML", yaml: "http: [", errContains: "failed to parse system config"},
		{name: "invalid duration", yaml: strings.Replace(valid, "10s", "not-a-duration", 1), errContains: "failed to parse system config"},
		{name: "invalid type", yaml: strings.Replace(valid, "max_retries: 2", "max_retries: two", 1), errContains: "failed to parse system config"},
		{name: "empty file", yaml: "", errContains: "failed to parse system config"},
		{name: "missing required fields", yaml: "{}", errContains: "system config validation failed: logging.level"},
		{name: "invalid log level", yaml: strings.Replace(valid, "info", "verbose", 1), errContains: "system config validation failed: logging.level"},
		{name: "invalid retries", yaml: strings.Replace(valid, "max_retries: 2", "max_retries: 6", 1), errContains: "system config validation failed: http.max_retries"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "system-config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.yaml), 0600))
			cfg, err := LoadSystemConfig(path)
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.NotNil(t, errors.Unwrap(err))
		})
	}
}

func TestLoadConfigReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	userCfg, err := LoadUserConfig(path)
	require.Error(t, err)
	assert.Nil(t, userCfg)
	assert.Contains(t, err.Error(), "failed to read user config file")
	assert.ErrorIs(t, err, os.ErrNotExist)

	systemCfg, err := LoadSystemConfig(path)
	require.Error(t, err)
	assert.Nil(t, systemCfg)
	assert.Contains(t, err.Error(), "failed to read system config file")
	assert.ErrorIs(t, err, os.ErrNotExist)
}
