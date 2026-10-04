package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadUserConfig loads and validates a user configuration file.
func LoadUserConfig(filePath string) (*UserConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user config file: %w", err)
	}

	var cfg UserConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}
	if err := ValidateUserConfig(&cfg); err != nil {
		return nil, fmt.Errorf("user config validation failed: %w", err)
	}
	return &cfg, nil
}

// LoadSystemConfig loads and validates a system configuration file.
func LoadSystemConfig(filePath string) (*SystemConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read system config file: %w", err)
	}

	var cfg SystemConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse system config: %w", err)
	}
	if err := ValidateSystemConfig(&cfg); err != nil {
		return nil, fmt.Errorf("system config validation failed: %w", err)
	}
	return &cfg, nil
}
